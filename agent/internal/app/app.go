package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"metalx.local/proto/metalxpb"
	"metalx/agent/internal/config"
	"metalx/agent/internal/discovery"
	"metalx/agent/internal/metrics"
)

type App struct {
	metalxpb.UnimplementedAgentServiceServer
	cfg config.Config
}

func New(cfg config.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.cfg.Validate(); err != nil {
		return err
	}

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", a.handleHealth)
	httpMux.HandleFunc("/snapshot", a.handleSnapshot)

	httpSrv := &http.Server{
		Addr:    a.cfg.ListenAddress,
		Handler: httpMux,
	}

	grpcListener, err := net.Listen("tcp", a.cfg.GRPCListenAddress)
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer()
	metalxpb.RegisterAgentServiceServer(grpcSrv, a)

	errCh := make(chan error, 2)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		grpcSrv.GracefulStop()
	}()
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()
	go func() {
		errCh <- grpcSrv.Serve(grpcListener)
	}()
	go a.reportLoop(ctx)

	log.Printf("mxagent http listening on %s", a.cfg.ListenAddress)
	log.Printf("mxagent grpc listening on %s", a.cfg.GRPCListenAddress)

	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}

func (a *App) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.ReportInterval)
	defer ticker.Stop()

	if err := a.reportSnapshot(ctx); err != nil {
		log.Printf("initial report failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.reportSnapshot(ctx); err != nil {
				log.Printf("report failed: %v", err)
			}
		}
	}
}

func (a *App) reportSnapshot(ctx context.Context) error {
	target := a.cfg.ControllerAddress
	if target == "" {
		discovered, err := discovery.DiscoverController(ctx, a.cfg.DiscoveryUDPPort)
		if err == nil {
			target = discovered
		}
	}
	if target == "" {
		return fmt.Errorf("controller not configured")
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(callCtx, target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := metalxpb.NewControllerServiceClient(conn)
	_, err = client.ReportSnapshot(callCtx, a.collectProtoSnapshot())
	return err
}

func (a *App) GetSnapshot(context.Context, *metalxpb.Empty) (*metalxpb.Snapshot, error) {
	return a.collectProtoSnapshot(), nil
}

func (a *App) ExecuteCommand(ctx context.Context, input *metalxpb.CommandRequest) (*metalxpb.CommandResult, error) {
	return executeCommand(ctx, input.GetCommand())
}

func (a *App) OpenTerminal(stream grpc.BidiStreamingServer[metalxpb.TerminalFrame, metalxpb.TerminalFrame]) error {
	firstFrame, err := stream.Recv()
	if err != nil {
		return err
	}

	shell := firstFrame.GetShell()
	if shell == "" {
		if _, err := os.Stat("/bin/bash"); err == nil {
			shell = "/bin/bash"
		} else {
			shell = "/bin/sh"
		}
	}
	cmd := exec.Command(shell)
	cmd.Env = os.Environ()

	size := &pty.Winsize{Cols: 120, Rows: 32}
	if firstFrame.GetCols() > 0 {
		size.Cols = uint16(firstFrame.GetCols())
	}
	if firstFrame.GetRows() > 0 {
		size.Rows = uint16(firstFrame.GetRows())
	}

	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	var sendMu sync.Mutex
	send := func(frame *metalxpb.TerminalFrame) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(frame)
	}

	if err := send(&metalxpb.TerminalFrame{
		NodeId:    a.cfg.NodeID,
		SessionId: fallbackString(firstFrame.GetSessionId(), fmt.Sprintf("session-%d", time.Now().UnixNano())),
		Open:      true,
		Output:    "",
	}); err != nil {
		return err
	}

	readErrCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if sendErr := send(&metalxpb.TerminalFrame{
					NodeId: firstFrame.GetNodeId(),
					Output: string(buf[:n]),
				}); sendErr != nil {
					readErrCh <- sendErr
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					_ = send(&metalxpb.TerminalFrame{NodeId: firstFrame.GetNodeId(), Close: true})
				}
				readErrCh <- err
				return
			}
		}
	}()

	recvCh := make(chan *metalxpb.TerminalFrame, 1)
	recvErrCh := make(chan error, 1)
	go func() {
		for {
			frame, recvErr := stream.Recv()
			if recvErr != nil {
				recvErrCh <- recvErr
				return
			}
			recvCh <- frame
		}
	}()

	if firstFrame.GetInput() != "" {
		if _, err := ptmx.Write([]byte(firstFrame.GetInput())); err != nil {
			return err
		}
	}

	for {
		select {
		case err := <-readErrCh:
			if err == io.EOF {
				_ = send(&metalxpb.TerminalFrame{NodeId: a.cfg.NodeID, Close: true})
				return nil
			}
			return err
		case err := <-recvErrCh:
			_ = cmd.Process.Kill()
			if err == io.EOF {
				return nil
			}
			return err
		case frame := <-recvCh:
			if frame.GetClose() {
				_ = cmd.Process.Kill()
				return send(&metalxpb.TerminalFrame{NodeId: a.cfg.NodeID, Close: true})
			}
			if frame.GetCols() > 0 && frame.GetRows() > 0 {
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(frame.GetCols()), Rows: uint16(frame.GetRows())})
			}
			if frame.GetInput() != "" {
				if _, err := ptmx.Write([]byte(frame.GetInput())); err != nil {
					return err
				}
			}
		}
	}
}

func (a *App) collectProtoSnapshot() *metalxpb.Snapshot {
	snapshot := metrics.Collect(a.cfg.NodeID)
	snapshot.GrpcAddress = advertisedGRPCAddress(a.cfg.GRPCListenAddress, snapshot.IPAddresses)
	return snapshotToProto(snapshot)
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"nodeId":      a.cfg.NodeID,
		"grpcAddress": a.cfg.GRPCListenAddress,
	})
}

func (a *App) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	snapshot := metrics.Collect(a.cfg.NodeID)
	snapshot.GrpcAddress = advertisedGRPCAddress(a.cfg.GRPCListenAddress, snapshot.IPAddresses)
	writeJSON(w, http.StatusOK, snapshot)
}

func executeCommand(ctx context.Context, command string) (*metalxpb.CommandResult, error) {
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := int32(0)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			return nil, err
		}
	}

	return &metalxpb.CommandResult{
		Command:  command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

func snapshotToProto(snapshot metrics.Snapshot) *metalxpb.Snapshot {
	interfaces := make([]*metalxpb.InterfaceInfo, 0, len(snapshot.Interfaces))
	for _, item := range snapshot.Interfaces {
		interfaces = append(interfaces, &metalxpb.InterfaceInfo{
			Name:  item.Name,
			Ip:    item.IP,
			Mac:   item.MAC,
			State: item.State,
			Rx:    item.RxMB,
			Tx:    item.TxMB,
		})
	}
	filesystems := make([]*metalxpb.FilesystemInfo, 0, len(snapshot.Filesystems))
	for _, item := range snapshot.Filesystems {
		filesystems = append(filesystems, &metalxpb.FilesystemInfo{
			Mount:       item.Mount,
			Size:        item.Size,
			UsedPercent: item.UsedPercent,
		})
	}
	users := make([]*metalxpb.LoggedUser, 0, len(snapshot.LoggedUsers))
	for _, item := range snapshot.LoggedUsers {
		users = append(users, &metalxpb.LoggedUser{
			User: item.User,
			Tty:  item.TTY,
			From: item.From,
		})
	}
	processes := make([]*metalxpb.ProcessInfo, 0, len(snapshot.TopProcesses))
	for _, item := range snapshot.TopProcesses {
		processes = append(processes, &metalxpb.ProcessInfo{
			Pid:  int32(item.PID),
			Name: item.Name,
			Cpu:  item.CPU,
			Mem:  item.Mem,
		})
	}
	alerts := make([]*metalxpb.AlertInfo, 0, len(snapshot.RecentAlerts))
	for _, item := range snapshot.RecentAlerts {
		alerts = append(alerts, &metalxpb.AlertInfo{
			Severity: item.Severity,
			Message:  item.Message,
			At:       item.At,
		})
	}

	return &metalxpb.Snapshot{
		NodeId:       snapshot.NodeID,
		Hostname:     snapshot.Hostname,
		Os:           snapshot.OS,
		Kernel:       snapshot.Kernel,
		Uptime:       snapshot.Uptime,
		CpuUsage:     snapshot.CPUUsage,
		MemoryUsage:  snapshot.MemoryUsage,
		DiskUsage:    snapshot.DiskUsage,
		Load1:        snapshot.Load1,
		Load5:        snapshot.Load5,
		Load15:       snapshot.Load15,
		NetworkRxMb:  snapshot.NetworkRxMB,
		NetworkTxMb:  snapshot.NetworkTxMB,
		DiskReadMb:   snapshot.DiskReadMB,
		DiskWriteMb:  snapshot.DiskWriteMB,
		IpAddresses:  snapshot.IPAddresses,
		MacAddresses: snapshot.MACAddresses,
		ProcessCount: int32(snapshot.ProcessCount),
		UserCount:    int32(snapshot.UserCount),
		Interfaces:   interfaces,
		Filesystems:  filesystems,
		LoggedUsers:  users,
		TopProcesses: processes,
		RecentAlerts: alerts,
		CollectedAt:  snapshot.CollectedAt.UTC().Format(time.RFC3339Nano),
		GrpcAddress:  snapshot.GrpcAddress,
	}
}

func advertisedGRPCAddress(listenAddress string, ips []string) string {
	host, port, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return listenAddress
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		return net.JoinHostPort(host, port)
	}
	for _, ip := range ips {
		if strings.HasPrefix(ip, "172.17.") {
			continue
		}
		return net.JoinHostPort(ip, port)
	}
	if len(ips) > 0 {
		return net.JoinHostPort(ips[0], port)
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func fallbackString(value, replacement string) string {
	if value != "" {
		return value
	}
	return replacement
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
