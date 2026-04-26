package app

import (
	"context"
	"time"

	"metalx.local/proto/metalxpb"
	"metalx/controller/internal/store"
)

func (a *App) defaultAppSettings() store.AppSettings {
	return store.AppSettings{
		AllowShell:                 a.cfg.AllowedShell,
		DiscoveryPort:              a.cfg.DiscoveryPort,
		DNSMasqStateDir:            a.cfg.DNSMasqStateDir,
		ProvisioningBaseURL:        a.cfg.ProvisioningBaseURL,
		PublicGRPCAddress:          a.cfg.PublicGRPCAddress,
		AgentBinaryPath:            a.cfg.AgentBinaryPath,
		DefaultNodeAddr:            a.cfg.DefaultNodeAddr,
		DashboardRefreshIntervalMS: 1000,
		DashboardDefaultCommand:    "uptime",
		TerminalShell:              "/bin/bash",
		AgentListenAddress:         ":18081",
		AgentGRPCListenAddress:     ":19091",
		AgentReportIntervalSeconds: 1,
		UpdatedAt:                  time.Now().UTC(),
	}
}

func (a *App) getAppSettings() store.AppSettings {
	settings, ok := a.store.LoadAppSettings()
	if !ok {
		settings = a.defaultAppSettings()
	}
	if settings.UpdatedAt.IsZero() {
		settings.UpdatedAt = time.Now().UTC()
	}
	if settings.DashboardRefreshIntervalMS <= 0 {
		settings.DashboardRefreshIntervalMS = 1000
	}
	if settings.AgentReportIntervalSeconds <= 0 {
		settings.AgentReportIntervalSeconds = 1
	}
	if settings.DashboardDefaultCommand == "" {
		settings.DashboardDefaultCommand = "uptime"
	}
	if settings.TerminalShell == "" {
		settings.TerminalShell = "/bin/bash"
	}
	if settings.AgentListenAddress == "" {
		settings.AgentListenAddress = ":18081"
	}
	if settings.AgentGRPCListenAddress == "" {
		settings.AgentGRPCListenAddress = ":19091"
	}
	return settings
}

func (a *App) ensureAppSettings() error {
	settings := a.getAppSettings()
	return a.store.SaveAppSettings(settings)
}

func appSettingsToProto(settings store.AppSettings) *metalxpb.AppSettings {
	return &metalxpb.AppSettings{
		AllowShell:                 settings.AllowShell,
		DiscoveryPort:              int32(settings.DiscoveryPort),
		DnsmasqStateDir:            settings.DNSMasqStateDir,
		ProvisioningBaseUrl:        settings.ProvisioningBaseURL,
		PublicGrpcAddress:          settings.PublicGRPCAddress,
		AgentBinaryPath:            settings.AgentBinaryPath,
		DefaultNodeAddr:            settings.DefaultNodeAddr,
		DashboardRefreshIntervalMs: int32(settings.DashboardRefreshIntervalMS),
		DashboardDefaultCommand:    settings.DashboardDefaultCommand,
		TerminalShell:              settings.TerminalShell,
		AgentListenAddress:         settings.AgentListenAddress,
		AgentGrpcListenAddress:     settings.AgentGRPCListenAddress,
		AgentReportIntervalSeconds: int32(settings.AgentReportIntervalSeconds),
		UpdatedAt:                  settings.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func appSettingsFromProto(settings *metalxpb.AppSettings) store.AppSettings {
	if settings == nil {
		return store.AppSettings{}
	}
	updatedAt, _ := time.Parse(time.RFC3339Nano, settings.GetUpdatedAt())
	return store.AppSettings{
		AllowShell:                 settings.GetAllowShell(),
		DiscoveryPort:              int(settings.GetDiscoveryPort()),
		DNSMasqStateDir:            settings.GetDnsmasqStateDir(),
		ProvisioningBaseURL:        settings.GetProvisioningBaseUrl(),
		PublicGRPCAddress:          settings.GetPublicGrpcAddress(),
		AgentBinaryPath:            settings.GetAgentBinaryPath(),
		DefaultNodeAddr:            settings.GetDefaultNodeAddr(),
		DashboardRefreshIntervalMS: int(settings.GetDashboardRefreshIntervalMs()),
		DashboardDefaultCommand:    settings.GetDashboardDefaultCommand(),
		TerminalShell:              settings.GetTerminalShell(),
		AgentListenAddress:         settings.GetAgentListenAddress(),
		AgentGRPCListenAddress:     settings.GetAgentGrpcListenAddress(),
		AgentReportIntervalSeconds: int(settings.GetAgentReportIntervalSeconds()),
		UpdatedAt:                  updatedAt,
	}
}

func (a *App) GetAppSettings(context.Context, *metalxpb.Empty) (*metalxpb.AppSettings, error) {
	return appSettingsToProto(a.getAppSettings()), nil
}

func (a *App) UpdateAppSettings(_ context.Context, payload *metalxpb.UpdateAppSettingsRequest) (*metalxpb.AppSettings, error) {
	settings := appSettingsFromProto(payload.GetSettings())
	current := a.getAppSettings()
	settings.UpdatedAt = time.Now().UTC()
	if settings.DNSMasqStateDir == "" {
		settings.DNSMasqStateDir = current.DNSMasqStateDir
	}
	if settings.ProvisioningBaseURL == "" {
		settings.ProvisioningBaseURL = current.ProvisioningBaseURL
	}
	if settings.PublicGRPCAddress == "" {
		settings.PublicGRPCAddress = current.PublicGRPCAddress
	}
	if settings.AgentBinaryPath == "" {
		settings.AgentBinaryPath = current.AgentBinaryPath
	}
	if settings.DefaultNodeAddr == "" {
		settings.DefaultNodeAddr = current.DefaultNodeAddr
	}
	if settings.DiscoveryPort <= 0 {
		settings.DiscoveryPort = current.DiscoveryPort
	}
	if settings.DashboardRefreshIntervalMS <= 0 {
		settings.DashboardRefreshIntervalMS = current.DashboardRefreshIntervalMS
	}
	if settings.DashboardDefaultCommand == "" {
		settings.DashboardDefaultCommand = current.DashboardDefaultCommand
	}
	if settings.TerminalShell == "" {
		settings.TerminalShell = current.TerminalShell
	}
	if settings.AgentListenAddress == "" {
		settings.AgentListenAddress = current.AgentListenAddress
	}
	if settings.AgentGRPCListenAddress == "" {
		settings.AgentGRPCListenAddress = current.AgentGRPCListenAddress
	}
	if settings.AgentReportIntervalSeconds <= 0 {
		settings.AgentReportIntervalSeconds = current.AgentReportIntervalSeconds
	}
	if err := a.store.SaveAppSettings(settings); err != nil {
		return nil, err
	}
	a.store.AddAudit(store.AuditRecord{
		ID:        "audit-settings-" + settings.UpdatedAt.Format("20060102150405.000000000"),
		Actor:     fallback(payload.GetActor(), "dashboard"),
		Action:    "update_app_settings",
		Target:    "app_settings",
		CreatedAt: settings.UpdatedAt,
	})
	return appSettingsToProto(settings), nil
}
