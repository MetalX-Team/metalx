package app

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"metalx.local/proto/metalxpb"
	"metalx/controller/internal/config"
	"metalx/controller/internal/discovery"
	"metalx/controller/internal/store"
)

type App struct {
	metalxpb.UnimplementedControllerServiceServer
	cfg   config.Config
	store *store.Store
}

func New(cfg config.Config) (*App, error) {
	persistentStore, err := store.New(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	app := &App{
		cfg:   cfg,
		store: persistentStore,
	}
	settings := app.getDnsmasqSettings()
	if err := app.persistDnsmasqSettings(settings); err != nil {
		_ = persistentStore.Close()
		return nil, err
	}
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	defer func() {
		_ = a.store.Close()
	}()
	log.Print(discovery.Banner(a.cfg.DiscoveryPort))

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", a.handleHealth)
	httpSrv := &http.Server{
		Addr:    a.cfg.ListenAddress,
		Handler: httpMux,
	}

	grpcListener, err := net.Listen("tcp", a.cfg.GRPCListenAddress)
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer()
	metalxpb.RegisterControllerServiceServer(grpcSrv, a)

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

	log.Printf("mxctl http listening on %s", a.cfg.ListenAddress)
	log.Printf("mxctl grpc listening on %s", a.cfg.GRPCListenAddress)

	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}

func (a *App) ReportSnapshot(_ context.Context, payload *metalxpb.Snapshot) (*metalxpb.Ack, error) {
	primaryIP, primaryMAC := choosePrimaryNetwork(protoInterfacesToShim(payload.GetInterfaces()), payload.GetIpAddresses(), payload.GetMacAddresses())
	parsedCollectedAt, err := time.Parse(time.RFC3339Nano, payload.GetCollectedAt())
	if err != nil {
		parsedCollectedAt = time.Now().UTC()
	}

	detail := store.NodeDetail{
		NodeSummary: store.NodeSummary{
			ID:           payload.GetNodeId(),
			Name:         payload.GetHostname(),
			Address:      fallback(payload.GetGrpcAddress(), a.cfg.DefaultNodeAddr),
			OS:           payload.GetOs(),
			Kernel:       payload.GetKernel(),
			Online:       true,
			LastSeenAt:   parsedCollectedAt,
			CPUUsage:     payload.GetCpuUsage(),
			MemoryUsage:  payload.GetMemoryUsage(),
			DiskUsage:    payload.GetDiskUsage(),
			Load1:        payload.GetLoad1(),
			Load5:        payload.GetLoad5(),
			Load15:       payload.GetLoad15(),
			NetworkRxMB:  payload.GetNetworkRxMb(),
			NetworkTxMB:  payload.GetNetworkTxMb(),
			ProcessCount: int(payload.GetProcessCount()),
			IPAddress:    primaryIP,
			MACAddress:   primaryMAC,
			AlertLevel:   alertLevel(payload.GetCpuUsage(), payload.GetMemoryUsage(), payload.GetDiskUsage()),
			PrimaryRole:  "unclassified",
		},
		Uptime:       payload.GetUptime(),
		UserCount:    int(payload.GetUserCount()),
		DiskReadMB:   payload.GetDiskReadMb(),
		DiskWriteMB:  payload.GetDiskWriteMb(),
		Tags:         []string{"auto-discovered"},
		Interfaces:   protoInterfacesToStore(payload.GetInterfaces()),
		Filesystems:  protoFilesystemsToStore(payload.GetFilesystems()),
		LoggedUsers:  protoUsersToStore(payload.GetLoggedUsers()),
		TopProcesses: protoProcessesToStore(payload.GetTopProcesses()),
		RecentAlerts: protoAlertsToStore(payload.GetRecentAlerts()),
	}
	a.store.UpsertNode(detail)
	return &metalxpb.Ack{Status: "accepted"}, nil
}

func (a *App) GetSummary(context.Context, *metalxpb.Empty) (*metalxpb.Summary, error) {
	summary := a.store.Summary()
	hotNodes := make([]*metalxpb.NodeSummary, 0)
	for _, node := range a.store.ListNodes() {
		copyNode := node
		hotNodes = append(hotNodes, nodeSummaryToProto(copyNode))
	}
	return &metalxpb.Summary{
		TotalNodes:        int32(asInt(summary["totalNodes"])),
		OnlineNodes:       int32(asInt(summary["onlineNodes"])),
		OfflineNodes:      int32(asInt(summary["offlineNodes"])),
		AverageCpu:        asFloat(summary["averageCPU"]),
		AverageMemory:     asFloat(summary["averageMemory"]),
		AverageDisk:       asFloat(summary["averageDisk"]),
		AlertCount:        int32(asInt(summary["alertCount"])),
		RunningTasks:      int32(asInt(summary["runningTasks"])),
		UpdatedAt:         summary["updatedAt"].(time.Time).UTC().Format(time.RFC3339Nano),
		TaskSuccessRate:   asFloat(summary["taskSuccessRate"]),
		NetworkThroughput: asFloat(summary["networkThroughput"]),
		HotNodes:          hotNodes,
	}, nil
}

func (a *App) ListNodes(context.Context, *metalxpb.Empty) (*metalxpb.ListNodesResponse, error) {
	nodes := a.store.ListNodes()
	items := make([]*metalxpb.NodeSummary, 0, len(nodes))
	for _, node := range nodes {
		copyNode := node
		items = append(items, nodeSummaryToProto(copyNode))
	}
	return &metalxpb.ListNodesResponse{Items: items}, nil
}

func (a *App) GetNode(_ context.Context, id *metalxpb.NodeID) (*metalxpb.NodeDetail, error) {
	node, ok := a.store.GetNode(id.GetId())
	if !ok {
		return nil, grpcNotFound("node not found")
	}
	return nodeDetailToProto(node), nil
}

func (a *App) RunTask(ctx context.Context, payload *metalxpb.RunTaskRequest) (*metalxpb.Task, error) {
	if !a.cfg.AllowedShell {
		return nil, grpcForbidden("shell execution disabled")
	}
	if payload.GetCommand() == "" || len(payload.GetTargets()) == 0 {
		return nil, grpcInvalid("command and targets are required")
	}

	task := store.Task{
		ID:        store.NewTaskID(),
		Command:   payload.GetCommand(),
		Targets:   payload.GetTargets(),
		Status:    "completed",
		StartedAt: time.Now().UTC(),
		Results:   make([]store.TaskResult, 0, len(payload.GetTargets())),
	}
	for _, target := range payload.GetTargets() {
		result := a.executeOnNode(ctx, target, payload.GetCommand())
		task.Results = append(task.Results, result)
		a.store.AddRecentCommand(target, result)
	}
	task.Status = summarizeTaskStatus(task.Results)
	finished := time.Now().UTC()
	task.FinishedAt = &finished
	a.store.AddTask(task)
	a.store.AddAudit(store.AuditRecord{
		ID:        "audit-" + task.ID,
		Actor:     fallback(payload.GetActor(), "api-admin"),
		Action:    "run_task",
		Target:    strings.Join(payload.GetTargets(), ","),
		CreatedAt: finished,
	})
	return taskToProto(task), nil
}

func (a *App) ListTasks(context.Context, *metalxpb.Empty) (*metalxpb.ListTasksResponse, error) {
	tasks := a.store.Tasks()
	items := make([]*metalxpb.Task, 0, len(tasks))
	for _, task := range tasks {
		copyTask := task
		items = append(items, taskToProto(copyTask))
	}
	return &metalxpb.ListTasksResponse{Items: items}, nil
}

func (a *App) ListAudits(context.Context, *metalxpb.Empty) (*metalxpb.ListAuditsResponse, error) {
	audits := a.store.Audits()
	items := make([]*metalxpb.AuditRecord, 0, len(audits))
	for _, audit := range audits {
		items = append(items, &metalxpb.AuditRecord{
			Id:        audit.ID,
			Actor:     audit.Actor,
			Action:    audit.Action,
			Target:    audit.Target,
			CreatedAt: audit.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return &metalxpb.ListAuditsResponse{Items: items}, nil
}

func (a *App) ListAlerts(context.Context, *metalxpb.Empty) (*metalxpb.ListAlertsResponse, error) {
	alerts := a.store.Alerts()
	items := make([]*metalxpb.AlertRecord, 0, len(alerts))
	for _, alert := range alerts {
		items = append(items, &metalxpb.AlertRecord{
			NodeId:   asString(alert["nodeId"]),
			NodeName: asString(alert["nodeName"]),
			Severity: asString(alert["severity"]),
			Message:  asString(alert["message"]),
			At:       asString(alert["at"]),
		})
	}
	return &metalxpb.ListAlertsResponse{Items: items}, nil
}

func (a *App) GetSystemInfo(context.Context, *metalxpb.Empty) (*metalxpb.SystemInfo, error) {
	return &metalxpb.SystemInfo{
		ControllerAddress: a.cfg.GRPCListenAddress,
		DiscoveryPort:     int32(a.cfg.DiscoveryPort),
		DatabasePath:      a.cfg.DatabasePath,
		ShellEnabled:      a.cfg.AllowedShell,
		Store:             "sqlite",
		Timestamp:         time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (a *App) GetDnsmasqSettings(context.Context, *metalxpb.Empty) (*metalxpb.DnsmasqSettings, error) {
	settings := a.getDnsmasqSettings()
	return dnsmasqSettingsToProto(settings), nil
}

func (a *App) UpdateDnsmasqSettings(_ context.Context, payload *metalxpb.UpdateDnsmasqSettingsRequest) (*metalxpb.DnsmasqSettings, error) {
	settings := dnsmasqSettingsFromProto(payload.GetSettings())
	settings.UpdatedAt = time.Now().UTC()
	settings = a.materializeDnsmasqSettings(settings)
	if err := validateDnsmasqSettings(settings); err != nil {
		return nil, grpcInvalid(err.Error())
	}
	if err := a.persistDnsmasqSettings(settings); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	a.store.AddAudit(store.AuditRecord{
		ID:        "audit-dnsmasq-" + settings.UpdatedAt.Format("20060102150405.000000000"),
		Actor:     fallback(payload.GetActor(), "dashboard"),
		Action:    "update_dnsmasq",
		Target:    settings.ListenInterface,
		CreatedAt: settings.UpdatedAt,
	})
	return dnsmasqSettingsToProto(settings), nil
}

func (a *App) OpenTerminal(stream grpc.BidiStreamingServer[metalxpb.TerminalFrame, metalxpb.TerminalFrame]) error {
	firstFrame, err := stream.Recv()
	if err != nil {
		return err
	}
	nodeID := firstFrame.GetNodeId()
	if nodeID == "" {
		return grpcInvalid("nodeId is required")
	}
	node, ok := a.store.GetNode(nodeID)
	if !ok {
		return grpcNotFound("node not found")
	}

	ctx := stream.Context()
	conn, err := grpc.DialContext(ctx, node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	agentStream, err := metalxpb.NewAgentServiceClient(conn).OpenTerminal(ctx)
	if err != nil {
		return err
	}
	if err := agentStream.Send(firstFrame); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	var sendMu sync.Mutex
	sendUp := func(frame *metalxpb.TerminalFrame) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(frame)
	}

	go func() {
		for {
			frame, recvErr := agentStream.Recv()
			if recvErr != nil {
				errCh <- recvErr
				return
			}
			if sendErr := sendUp(frame); sendErr != nil {
				errCh <- sendErr
				return
			}
			if frame.GetClose() {
				_ = agentStream.CloseSend()
				errCh <- io.EOF
				return
			}
		}
	}()

	go func() {
		for {
			frame, recvErr := stream.Recv()
			if recvErr != nil {
				_ = agentStream.CloseSend()
				errCh <- recvErr
				return
			}
			if sendErr := agentStream.Send(frame); sendErr != nil {
				errCh <- sendErr
				return
			}
		}
	}()

	err = <-errCh
	if err == io.EOF {
		return nil
	}
	return nil
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"store":  "sqlite",
	})
}

func (a *App) executeOnNode(ctx context.Context, nodeID, command string) store.TaskResult {
	started := time.Now().UTC()
	node, ok := a.store.GetNode(nodeID)
	if !ok {
		return store.TaskResult{NodeID: nodeID, Status: "failed", Stderr: "node not found", ExitCode: 1, StartedAt: started.Format(time.RFC3339)}
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(callCtx, node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return store.TaskResult{NodeID: nodeID, Status: "failed", Stderr: err.Error(), ExitCode: 1, StartedAt: started.Format(time.RFC3339)}
	}
	defer conn.Close()

	result, err := metalxpb.NewAgentServiceClient(conn).ExecuteCommand(callCtx, &metalxpb.CommandRequest{Command: command})
	if err != nil {
		return store.TaskResult{NodeID: nodeID, Status: "failed", Stderr: err.Error(), ExitCode: 1, StartedAt: started.Format(time.RFC3339)}
	}

	status := "success"
	if result.GetExitCode() != 0 {
		status = "failed"
	}
	return store.TaskResult{
		NodeID:    nodeID,
		Status:    status,
		Stdout:    result.GetStdout(),
		Stderr:    result.GetStderr(),
		ExitCode:  int(result.GetExitCode()),
		Duration:  time.Since(started).String(),
		StartedAt: started.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func grpcForbidden(message string) error {
	return status.Error(codes.PermissionDenied, message)
}

func grpcInvalid(message string) error {
	return status.Error(codes.InvalidArgument, message)
}

func grpcNotFound(message string) error {
	return status.Error(codes.NotFound, message)
}
