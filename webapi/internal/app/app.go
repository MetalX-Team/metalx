package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"metalx.local/proto/metalxpb"
	"metalx/webapi/internal/auth"
	"metalx/webapi/internal/config"
)

type App struct {
	cfg    config.Config
	auth   *auth.Manager
	conn   *grpc.ClientConn
	client metalxpb.ControllerServiceClient
}

func New(cfg config.Config) (*App, error) {
	authManager, err := auth.New(cfg.DatabasePath, cfg.AdminUser, cfg.AdminPassword)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(cfg.ControllerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = authManager.Close()
		return nil, err
	}
	log.Printf("bootstrap admin credentials: %s / %s", cfg.AdminUser, cfg.AdminPassword)
	return &App{
		cfg:    cfg,
		auth:   authManager,
		conn:   conn,
		client: metalxpb.NewControllerServiceClient(conn),
	}, nil
}

func (a *App) Run() error {
	defer func() {
		_ = a.auth.Close()
		_ = a.conn.Close()
	}()

	router := gin.Default()
	router.Use(cors())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/api/auth/login", a.handleLogin)

	protected := router.Group("/api")
	protected.Use(a.auth.GinMiddleware())
	protected.GET("/summary", a.handleSummary)
	protected.GET("/nodes", a.handleNodes)
	protected.GET("/nodes/:id", a.handleNode)
	protected.GET("/tasks", a.handleTasks)
	protected.GET("/audits", a.handleAudits)
	protected.GET("/alerts", a.handleAlerts)
	protected.GET("/system", a.handleSystem)
	protected.GET("/settings/runtime", a.handleRuntimeSettings)
	protected.PUT("/settings/runtime", a.handleUpdateRuntimeSettings)
	protected.GET("/system/dnsmasq", a.handleDnsmasqSettings)
	protected.PUT("/system/dnsmasq", a.handleUpdateDnsmasqSettings)
	protected.GET("/install/profiles", a.handleInstallProfiles)
	protected.PUT("/install/profiles", a.handleUpsertInstallProfile)
	protected.GET("/install/jobs", a.handleInstallJobs)
	protected.POST("/install/jobs", a.handleCreateInstallJob)
	protected.POST("/tasks", a.handleRunTask)
	protected.GET("/terminal", a.handleTerminal)

	return router.Run(a.cfg.ListenAddress)
}

func (a *App) handleLogin(c *gin.Context) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, ok := a.auth.Login(payload.Username, payload.Password)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresIn": 43200,
		"user":      payload.Username,
	})
}

func (a *App) handleSummary(c *gin.Context) {
	resp, err := a.client.GetSummary(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleNodes(c *gin.Context) {
	resp, err := a.client.ListNodes(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleNode(c *gin.Context) {
	resp, err := a.client.GetNode(a.callContext(c), &metalxpb.NodeID{Id: c.Param("id")})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleTasks(c *gin.Context) {
	resp, err := a.client.ListTasks(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleAudits(c *gin.Context) {
	resp, err := a.client.ListAudits(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleAlerts(c *gin.Context) {
	resp, err := a.client.ListAlerts(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleSystem(c *gin.Context) {
	resp, err := a.client.GetSystemInfo(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleRuntimeSettings(c *gin.Context) {
	resp, err := a.client.GetAppSettings(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	data, err := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	body["adminUser"] = a.auth.PrimaryUser()
	c.JSON(http.StatusOK, body)
}

func (a *App) handleUpdateRuntimeSettings(c *gin.Context) {
	var payload struct {
		AllowShell                 bool   `json:"allowShell"`
		DiscoveryPort              int32  `json:"discoveryPort"`
		DNSMasqStateDir            string `json:"dnsmasqStateDir"`
		ProvisioningBaseURL        string `json:"provisioningBaseUrl"`
		PublicGRPCAddress          string `json:"publicGrpcAddress"`
		AgentBinaryPath            string `json:"agentBinaryPath"`
		DefaultNodeAddr            string `json:"defaultNodeAddr"`
		DashboardRefreshIntervalMS int32  `json:"dashboardRefreshIntervalMs"`
		DashboardDefaultCommand    string `json:"dashboardDefaultCommand"`
		TerminalShell              string `json:"terminalShell"`
		AgentListenAddress         string `json:"agentListenAddress"`
		AgentGRPCListenAddress     string `json:"agentGrpcListenAddress"`
		AgentReportIntervalSeconds int32  `json:"agentReportIntervalSeconds"`
		AdminUser                  string `json:"adminUser"`
		AdminPassword              string `json:"adminPassword"`
		Actor                      string `json:"actor"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := a.client.UpdateAppSettings(a.callContext(c), &metalxpb.UpdateAppSettingsRequest{
		Settings: &metalxpb.AppSettings{
			AllowShell:                 payload.AllowShell,
			DiscoveryPort:              payload.DiscoveryPort,
			DnsmasqStateDir:            payload.DNSMasqStateDir,
			ProvisioningBaseUrl:        payload.ProvisioningBaseURL,
			PublicGrpcAddress:          payload.PublicGRPCAddress,
			AgentBinaryPath:            payload.AgentBinaryPath,
			DefaultNodeAddr:            payload.DefaultNodeAddr,
			DashboardRefreshIntervalMs: payload.DashboardRefreshIntervalMS,
			DashboardDefaultCommand:    payload.DashboardDefaultCommand,
			TerminalShell:              payload.TerminalShell,
			AgentListenAddress:         payload.AgentListenAddress,
			AgentGrpcListenAddress:     payload.AgentGRPCListenAddress,
			AgentReportIntervalSeconds: payload.AgentReportIntervalSeconds,
		},
		Actor: payload.Actor,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	if payload.AdminUser != "" && payload.AdminPassword != "" {
		if err := a.auth.UpdateAdminCredentials(payload.AdminUser, payload.AdminPassword); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	data, err := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	body["adminUser"] = a.auth.PrimaryUser()
	c.JSON(http.StatusOK, body)
}

func (a *App) handleDnsmasqSettings(c *gin.Context) {
	resp, err := a.client.GetDnsmasqSettings(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleUpdateDnsmasqSettings(c *gin.Context) {
	var payload struct {
		Enabled         bool     `json:"enabled"`
		ListenInterface string   `json:"listenInterface"`
		BindAddress     string   `json:"bindAddress"`
		DHCPRangeStart  string   `json:"dhcpRangeStart"`
		DHCPRangeEnd    string   `json:"dhcpRangeEnd"`
		DHCPLeaseTime   string   `json:"dhcpLeaseTime"`
		Gateway         string   `json:"gateway"`
		DNSServers      []string `json:"dnsServers"`
		TFTPRoot        string   `json:"tftpRoot"`
		BootFile        string   `json:"bootFile"`
		PXEPrompt       string   `json:"pxePrompt"`
		PXEServiceLabel string   `json:"pxeServiceLabel"`
		KernelPath      string   `json:"kernelPath"`
		InitrdPath      string   `json:"initrdPath"`
		BootArgs        string   `json:"bootArgs"`
		NextServer      string   `json:"nextServer"`
		Actor           string   `json:"actor"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := a.client.UpdateDnsmasqSettings(a.callContext(c), &metalxpb.UpdateDnsmasqSettingsRequest{
		Settings: &metalxpb.DnsmasqSettings{
			Enabled:         payload.Enabled,
			ListenInterface: payload.ListenInterface,
			BindAddress:     payload.BindAddress,
			DhcpRangeStart:  payload.DHCPRangeStart,
			DhcpRangeEnd:    payload.DHCPRangeEnd,
			DhcpLeaseTime:   payload.DHCPLeaseTime,
			Gateway:         payload.Gateway,
			DnsServers:      payload.DNSServers,
			TftpRoot:        payload.TFTPRoot,
			BootFile:        payload.BootFile,
			PxePrompt:       payload.PXEPrompt,
			PxeServiceLabel: payload.PXEServiceLabel,
			KernelPath:      payload.KernelPath,
			InitrdPath:      payload.InitrdPath,
			BootArgs:        payload.BootArgs,
			NextServer:      payload.NextServer,
		},
		Actor: payload.Actor,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleInstallProfiles(c *gin.Context) {
	resp, err := a.client.ListInstallProfiles(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleUpsertInstallProfile(c *gin.Context) {
	var payload struct {
		ID                    string   `json:"id"`
		Name                  string   `json:"name"`
		OSFamily              string   `json:"osFamily"`
		OSVersion             string   `json:"osVersion"`
		Architecture          string   `json:"architecture"`
		Firmware              string   `json:"firmware"`
		InstallSource         string   `json:"installSource"`
		BootKernelPath        string   `json:"bootKernelPath"`
		BootInitrdPath        string   `json:"bootInitrdPath"`
		HostnamePattern       string   `json:"hostnamePattern"`
		Timezone              string   `json:"timezone"`
		Locale                string   `json:"locale"`
		KeyboardLayout        string   `json:"keyboardLayout"`
		AdminUsername         string   `json:"adminUsername"`
		AdminPasswordHash     string   `json:"adminPasswordHash"`
		SSHAuthorizedKeys     []string `json:"sshAuthorizedKeys"`
		Packages              []string `json:"packages"`
		PackageMirror         string   `json:"packageMirror"`
		DiskLayout            string   `json:"diskLayout"`
		NetworkMode           string   `json:"networkMode"`
		AgentBinaryURL        string   `json:"agentBinaryUrl"`
		AgentServiceName      string   `json:"agentServiceName"`
		ControllerGRPCAddress string   `json:"controllerGrpcAddress"`
		ExtraKernelArgs       string   `json:"extraKernelArgs"`
		PostInstallScript     string   `json:"postInstallScript"`
		Enabled               bool     `json:"enabled"`
		Actor                 string   `json:"actor"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := a.client.UpsertInstallProfile(a.callContext(c), &metalxpb.UpsertInstallProfileRequest{
		Profile: &metalxpb.InstallProfile{
			Id:                    payload.ID,
			Name:                  payload.Name,
			OsFamily:              payload.OSFamily,
			OsVersion:             payload.OSVersion,
			Architecture:          payload.Architecture,
			Firmware:              payload.Firmware,
			InstallSource:         payload.InstallSource,
			BootKernelPath:        payload.BootKernelPath,
			BootInitrdPath:        payload.BootInitrdPath,
			HostnamePattern:       payload.HostnamePattern,
			Timezone:              payload.Timezone,
			Locale:                payload.Locale,
			KeyboardLayout:        payload.KeyboardLayout,
			AdminUsername:         payload.AdminUsername,
			AdminPasswordHash:     payload.AdminPasswordHash,
			SshAuthorizedKeys:     payload.SSHAuthorizedKeys,
			Packages:              payload.Packages,
			PackageMirror:         payload.PackageMirror,
			DiskLayout:            payload.DiskLayout,
			NetworkMode:           payload.NetworkMode,
			AgentBinaryUrl:        payload.AgentBinaryURL,
			AgentServiceName:      payload.AgentServiceName,
			ControllerGrpcAddress: payload.ControllerGRPCAddress,
			ExtraKernelArgs:       payload.ExtraKernelArgs,
			PostInstallScript:     payload.PostInstallScript,
			Enabled:               payload.Enabled,
		},
		Actor: payload.Actor,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleInstallJobs(c *gin.Context) {
	resp, err := a.client.ListInstallJobs(a.callContext(c), &metalxpb.Empty{})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleCreateInstallJob(c *gin.Context) {
	var payload struct {
		ProfileID string `json:"profileId"`
		MAC       string `json:"macAddress"`
		Hostname  string `json:"hostname"`
		NodeID    string `json:"nodeId"`
		Actor     string `json:"actor"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := a.client.CreateInstallJob(a.callContext(c), &metalxpb.CreateInstallJobRequest{
		ProfileId:  payload.ProfileID,
		MacAddress: payload.MAC,
		Hostname:   payload.Hostname,
		NodeId:     payload.NodeID,
		Actor:      payload.Actor,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleRunTask(c *gin.Context) {
	var payload struct {
		Command string   `json:"command"`
		Targets []string `json:"targets"`
		Actor   string   `json:"actor"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := a.client.RunTask(a.callContext(c), &metalxpb.RunTaskRequest{
		Command: payload.Command,
		Targets: payload.Targets,
		Actor:   payload.Actor,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}
	writeProtoJSON(c, resp)
}

func (a *App) handleTerminal(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	stream, err := a.client.OpenTerminal(c.Request.Context())
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	errCh := make(chan error, 2)
	go func() {
		for {
			_, data, readErr := ws.ReadMessage()
			if readErr != nil {
				_ = stream.CloseSend()
				errCh <- readErr
				return
			}
			frame := &metalxpb.TerminalFrame{}
			if unmarshalErr := protojson.Unmarshal(data, frame); unmarshalErr != nil {
				errCh <- unmarshalErr
				return
			}
			if sendErr := stream.Send(frame); sendErr != nil {
				errCh <- sendErr
				return
			}
		}
	}()

	go func() {
		for {
			frame, recvErr := stream.Recv()
			if recvErr != nil {
				data, _ := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}.Marshal(&metalxpb.TerminalFrame{Close: true})
				_ = ws.WriteMessage(websocket.TextMessage, data)
				errCh <- recvErr
				return
			}
			data, marshalErr := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}.Marshal(frame)
			if marshalErr != nil {
				errCh <- marshalErr
				return
			}
			if writeErr := ws.WriteMessage(websocket.TextMessage, data); writeErr != nil {
				errCh <- writeErr
				return
			}
		}
	}()

	<-errCh
}

func (a *App) callContext(c *gin.Context) context.Context {
	ctx, _ := context.WithTimeout(c.Request.Context(), 10*time.Second)
	return ctx
}

func writeProtoJSON(c *gin.Context, message proto.Message) {
	data, err := protojson.MarshalOptions{
		UseProtoNames:   false,
		EmitUnpopulated: true,
	}.Marshal(message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func handleGRPCError(c *gin.Context, err error) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case 3:
			c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
			return
		case 5:
			c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
			return
		case 7:
			c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
			return
		}
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
