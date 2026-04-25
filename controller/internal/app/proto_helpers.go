package app

import (
	"fmt"
	"time"

	"metalx.local/proto/metalxpb"
	"metalx/controller/internal/store"
)

func protoInterfacesToStore(items []*metalxpb.InterfaceInfo) []store.InterfaceInfo {
	out := make([]store.InterfaceInfo, 0, len(items))
	for _, item := range items {
		out = append(out, store.InterfaceInfo{Name: item.GetName(), IP: item.GetIp(), MAC: item.GetMac(), State: item.GetState(), RxMB: item.GetRx(), TxMB: item.GetTx()})
	}
	return out
}

func protoInterfacesToShim(items []*metalxpb.InterfaceInfo) []*storeInterfaceShim {
	out := make([]*storeInterfaceShim, 0, len(items))
	for _, item := range items {
		out = append(out, &storeInterfaceShim{Name: item.GetName(), IP: item.GetIp(), MAC: item.GetMac()})
	}
	return out
}

func protoFilesystemsToStore(items []*metalxpb.FilesystemInfo) []store.FilesystemInfo {
	out := make([]store.FilesystemInfo, 0, len(items))
	for _, item := range items {
		out = append(out, store.FilesystemInfo{Mount: item.GetMount(), Size: item.GetSize(), UsedPercent: item.GetUsedPercent()})
	}
	return out
}

func protoUsersToStore(items []*metalxpb.LoggedUser) []store.LoggedUser {
	out := make([]store.LoggedUser, 0, len(items))
	for _, item := range items {
		out = append(out, store.LoggedUser{User: item.GetUser(), TTY: item.GetTty(), From: item.GetFrom()})
	}
	return out
}

func protoProcessesToStore(items []*metalxpb.ProcessInfo) []store.ProcessInfo {
	out := make([]store.ProcessInfo, 0, len(items))
	for _, item := range items {
		out = append(out, store.ProcessInfo{PID: int(item.GetPid()), Name: item.GetName(), CPU: item.GetCpu(), Mem: item.GetMem()})
	}
	return out
}

func protoAlertsToStore(items []*metalxpb.AlertInfo) []store.AlertInfo {
	out := make([]store.AlertInfo, 0, len(items))
	for _, item := range items {
		out = append(out, store.AlertInfo{Severity: item.GetSeverity(), Message: item.GetMessage(), At: item.GetAt()})
	}
	return out
}

func nodeSummaryToProto(node store.NodeSummary) *metalxpb.NodeSummary {
	return &metalxpb.NodeSummary{
		Id:           node.ID,
		Name:         node.Name,
		Address:      node.Address,
		Os:           node.OS,
		Kernel:       node.Kernel,
		Online:       node.Online,
		LastSeenAt:   node.LastSeenAt.UTC().Format(time.RFC3339Nano),
		CpuUsage:     node.CPUUsage,
		MemoryUsage:  node.MemoryUsage,
		DiskUsage:    node.DiskUsage,
		Load1:        node.Load1,
		Load5:        node.Load5,
		Load15:       node.Load15,
		NetworkRxMb:  node.NetworkRxMB,
		NetworkTxMb:  node.NetworkTxMB,
		ProcessCount: int32(node.ProcessCount),
		IpAddress:    node.IPAddress,
		MacAddress:   node.MACAddress,
		AlertLevel:   node.AlertLevel,
		PrimaryRole:  node.PrimaryRole,
	}
}

func nodeDetailToProto(node store.NodeDetail) *metalxpb.NodeDetail {
	interfaces := make([]*metalxpb.InterfaceInfo, 0, len(node.Interfaces))
	for _, item := range node.Interfaces {
		interfaces = append(interfaces, &metalxpb.InterfaceInfo{Name: item.Name, Ip: item.IP, Mac: item.MAC, State: item.State, Rx: item.RxMB, Tx: item.TxMB})
	}
	filesystems := make([]*metalxpb.FilesystemInfo, 0, len(node.Filesystems))
	for _, item := range node.Filesystems {
		filesystems = append(filesystems, &metalxpb.FilesystemInfo{Mount: item.Mount, Size: item.Size, UsedPercent: item.UsedPercent})
	}
	users := make([]*metalxpb.LoggedUser, 0, len(node.LoggedUsers))
	for _, item := range node.LoggedUsers {
		users = append(users, &metalxpb.LoggedUser{User: item.User, Tty: item.TTY, From: item.From})
	}
	processes := make([]*metalxpb.ProcessInfo, 0, len(node.TopProcesses))
	for _, item := range node.TopProcesses {
		processes = append(processes, &metalxpb.ProcessInfo{Pid: int32(item.PID), Name: item.Name, Cpu: item.CPU, Mem: item.Mem})
	}
	alerts := make([]*metalxpb.AlertInfo, 0, len(node.RecentAlerts))
	for _, item := range node.RecentAlerts {
		alerts = append(alerts, &metalxpb.AlertInfo{Severity: item.Severity, Message: item.Message, At: item.At})
	}
	commands := make([]*metalxpb.TaskResult, 0, len(node.RecentCommands))
	for _, item := range node.RecentCommands {
		commands = append(commands, taskResultToProto(item))
	}
	return &metalxpb.NodeDetail{
		Summary:        nodeSummaryToProto(node.NodeSummary),
		Uptime:         node.Uptime,
		UserCount:      int32(node.UserCount),
		DiskReadMb:     node.DiskReadMB,
		DiskWriteMb:    node.DiskWriteMB,
		Tags:           node.Tags,
		Interfaces:     interfaces,
		Filesystems:    filesystems,
		LoggedUsers:    users,
		TopProcesses:   processes,
		RecentAlerts:   alerts,
		RecentCommands: commands,
	}
}

func taskResultToProto(result store.TaskResult) *metalxpb.TaskResult {
	return &metalxpb.TaskResult{
		NodeId:    result.NodeID,
		Status:    result.Status,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
		ExitCode:  int32(result.ExitCode),
		Duration:  result.Duration,
		StartedAt: result.StartedAt,
	}
}

func taskToProto(task store.Task) *metalxpb.Task {
	results := make([]*metalxpb.TaskResult, 0, len(task.Results))
	for _, result := range task.Results {
		results = append(results, taskResultToProto(result))
	}
	finished := ""
	if task.FinishedAt != nil {
		finished = task.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return &metalxpb.Task{
		Id:         task.ID,
		Command:    task.Command,
		Targets:    task.Targets,
		Status:     task.Status,
		StartedAt:  task.StartedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt: finished,
		Results:    results,
	}
}

func dnsmasqSettingsToProto(settings store.DnsmasqSettings) *metalxpb.DnsmasqSettings {
	return &metalxpb.DnsmasqSettings{
		Enabled:         settings.Enabled,
		ListenInterface: settings.ListenInterface,
		BindAddress:     settings.BindAddress,
		DhcpRangeStart:  settings.DHCPRangeStart,
		DhcpRangeEnd:    settings.DHCPRangeEnd,
		DhcpLeaseTime:   settings.DHCPLeaseTime,
		Gateway:         settings.Gateway,
		DnsServers:      settings.DNSServers,
		TftpRoot:        settings.TFTPRoot,
		BootFile:        settings.BootFile,
		PxePrompt:       settings.PXEPrompt,
		PxeServiceLabel: settings.PXEServiceLabel,
		KernelPath:      settings.KernelPath,
		InitrdPath:      settings.InitrdPath,
		BootArgs:        settings.BootArgs,
		NextServer:      settings.NextServer,
		ConfigPath:      settings.ConfigPath,
		PxeConfigPath:   settings.PXEConfigPath,
		RenderedConfig:  settings.RenderedConfig,
		RenderedPxeMenu: settings.RenderedPXEMenu,
		UpdatedAt:       settings.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func dnsmasqSettingsFromProto(settings *metalxpb.DnsmasqSettings) store.DnsmasqSettings {
	if settings == nil {
		return store.DnsmasqSettings{}
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, settings.GetUpdatedAt())
	if err != nil {
		updatedAt = time.Time{}
	}
	return store.DnsmasqSettings{
		Enabled:         settings.GetEnabled(),
		ListenInterface: settings.GetListenInterface(),
		BindAddress:     settings.GetBindAddress(),
		DHCPRangeStart:  settings.GetDhcpRangeStart(),
		DHCPRangeEnd:    settings.GetDhcpRangeEnd(),
		DHCPLeaseTime:   settings.GetDhcpLeaseTime(),
		Gateway:         settings.GetGateway(),
		DNSServers:      settings.GetDnsServers(),
		TFTPRoot:        settings.GetTftpRoot(),
		BootFile:        settings.GetBootFile(),
		PXEPrompt:       settings.GetPxePrompt(),
		PXEServiceLabel: settings.GetPxeServiceLabel(),
		KernelPath:      settings.GetKernelPath(),
		InitrdPath:      settings.GetInitrdPath(),
		BootArgs:        settings.GetBootArgs(),
		NextServer:      settings.GetNextServer(),
		ConfigPath:      settings.GetConfigPath(),
		PXEConfigPath:   settings.GetPxeConfigPath(),
		RenderedConfig:  settings.GetRenderedConfig(),
		RenderedPXEMenu: settings.GetRenderedPxeMenu(),
		UpdatedAt:       updatedAt,
	}
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	default:
		return 0
	}
}

func asString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}
