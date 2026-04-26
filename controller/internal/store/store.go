package store

import (
	"fmt"
	"time"
)

type InterfaceInfo struct {
	Name  string `json:"name"`
	IP    string `json:"ip"`
	MAC   string `json:"mac"`
	State string `json:"state"`
	RxMB  string `json:"rx"`
	TxMB  string `json:"tx"`
}

type FilesystemInfo struct {
	Mount       string  `json:"mount"`
	Size        string  `json:"size"`
	UsedPercent float64 `json:"usedPercent"`
}

type LoggedUser struct {
	User string `json:"user"`
	TTY  string `json:"tty"`
	From string `json:"from"`
}

type ProcessInfo struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu"`
	Mem  float64 `json:"mem"`
}

type AlertInfo struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	At       string `json:"at"`
}

type NodeSummary struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`
	OS           string    `json:"os"`
	Kernel       string    `json:"kernel"`
	Online       bool      `json:"online"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
	CPUUsage     float64   `json:"cpuUsage"`
	MemoryUsage  float64   `json:"memoryUsage"`
	DiskUsage    float64   `json:"diskUsage"`
	Load1        float64   `json:"load1"`
	Load5        float64   `json:"load5"`
	Load15       float64   `json:"load15"`
	NetworkRxMB  float64   `json:"networkRxMb"`
	NetworkTxMB  float64   `json:"networkTxMb"`
	ProcessCount int       `json:"processCount"`
	IPAddress    string    `json:"ipAddress"`
	MACAddress   string    `json:"macAddress"`
	AlertLevel   string    `json:"alertLevel"`
	PrimaryRole  string    `json:"primaryRole"`
}

type NodeDetail struct {
	NodeSummary
	Uptime         string           `json:"uptime"`
	UserCount      int              `json:"userCount"`
	DiskReadMB     float64          `json:"diskReadMb"`
	DiskWriteMB    float64          `json:"diskWriteMb"`
	Tags           []string         `json:"tags"`
	Interfaces     []InterfaceInfo  `json:"interfaces"`
	Filesystems    []FilesystemInfo `json:"filesystems"`
	LoggedUsers    []LoggedUser     `json:"loggedUsers"`
	TopProcesses   []ProcessInfo    `json:"topProcesses"`
	RecentAlerts   []AlertInfo      `json:"recentAlerts"`
	RecentCommands []TaskResult     `json:"recentCommands"`
}

type Task struct {
	ID         string       `json:"id"`
	Command    string       `json:"command"`
	Targets    []string     `json:"targets"`
	Status     string       `json:"status"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt *time.Time   `json:"finishedAt,omitempty"`
	Results    []TaskResult `json:"results"`
}

type TaskResult struct {
	NodeID    string `json:"nodeId"`
	Status    string `json:"status"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exitCode"`
	Duration  string `json:"duration"`
	StartedAt string `json:"startedAt"`
}

type AuditRecord struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"createdAt"`
}

type DnsmasqSettings struct {
	Enabled         bool      `json:"enabled"`
	ListenInterface string    `json:"listenInterface"`
	BindAddress     string    `json:"bindAddress"`
	DHCPRangeStart  string    `json:"dhcpRangeStart"`
	DHCPRangeEnd    string    `json:"dhcpRangeEnd"`
	DHCPLeaseTime   string    `json:"dhcpLeaseTime"`
	Gateway         string    `json:"gateway"`
	DNSServers      []string  `json:"dnsServers"`
	TFTPRoot        string    `json:"tftpRoot"`
	BootFile        string    `json:"bootFile"`
	PXEPrompt       string    `json:"pxePrompt"`
	PXEServiceLabel string    `json:"pxeServiceLabel"`
	KernelPath      string    `json:"kernelPath"`
	InitrdPath      string    `json:"initrdPath"`
	BootArgs        string    `json:"bootArgs"`
	NextServer      string    `json:"nextServer"`
	ConfigPath      string    `json:"configPath"`
	PXEConfigPath   string    `json:"pxeConfigPath"`
	RenderedConfig  string    `json:"renderedConfig"`
	RenderedPXEMenu string    `json:"renderedPxeMenu"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type InstallProfile struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	OSFamily           string    `json:"osFamily"`
	OSVersion          string    `json:"osVersion"`
	Architecture       string    `json:"architecture"`
	Firmware           string    `json:"firmware"`
	InstallSource      string    `json:"installSource"`
	BootKernelPath     string    `json:"bootKernelPath"`
	BootInitrdPath     string    `json:"bootInitrdPath"`
	HostnamePattern    string    `json:"hostnamePattern"`
	Timezone           string    `json:"timezone"`
	Locale             string    `json:"locale"`
	KeyboardLayout     string    `json:"keyboardLayout"`
	AdminUsername      string    `json:"adminUsername"`
	AdminPasswordHash  string    `json:"adminPasswordHash"`
	SSHAuthorizedKeys  []string  `json:"sshAuthorizedKeys"`
	Packages           []string  `json:"packages"`
	PackageMirror      string    `json:"packageMirror"`
	DiskLayout         string    `json:"diskLayout"`
	NetworkMode        string    `json:"networkMode"`
	AgentBinaryURL     string    `json:"agentBinaryUrl"`
	AgentServiceName   string    `json:"agentServiceName"`
	ControllerGRPCAddr string    `json:"controllerGrpcAddress"`
	ExtraKernelArgs    string    `json:"extraKernelArgs"`
	PostInstallScript  string    `json:"postInstallScript"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type InstallEvent struct {
	Phase     string    `json:"phase"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type InstallJob struct {
	ID             string         `json:"id"`
	ProfileID      string         `json:"profileId"`
	ProfileName    string         `json:"profileName"`
	OSFamily       string         `json:"osFamily"`
	Status         string         `json:"status"`
	MACAddress     string         `json:"macAddress"`
	Hostname       string         `json:"hostname"`
	NodeID         string         `json:"nodeId"`
	Token          string         `json:"token"`
	BootURL        string         `json:"bootUrl"`
	ConfigURL      string         `json:"configUrl"`
	AgentScriptURL string         `json:"agentScriptUrl"`
	LastEvent      string         `json:"lastEvent"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	BootPreview    string         `json:"bootPreview"`
	ConfigPreview  string         `json:"configPreview"`
	Events         []InstallEvent `json:"events"`
}

type AppSettings struct {
	AllowShell                 bool      `json:"allowShell"`
	DiscoveryPort              int       `json:"discoveryPort"`
	DNSMasqStateDir            string    `json:"dnsmasqStateDir"`
	ProvisioningBaseURL        string    `json:"provisioningBaseUrl"`
	PublicGRPCAddress          string    `json:"publicGrpcAddress"`
	AgentBinaryPath            string    `json:"agentBinaryPath"`
	DefaultNodeAddr            string    `json:"defaultNodeAddr"`
	DashboardRefreshIntervalMS int       `json:"dashboardRefreshIntervalMs"`
	DashboardDefaultCommand    string    `json:"dashboardDefaultCommand"`
	TerminalShell              string    `json:"terminalShell"`
	AgentListenAddress         string    `json:"agentListenAddress"`
	AgentGRPCListenAddress     string    `json:"agentGrpcListenAddress"`
	AgentReportIntervalSeconds int       `json:"agentReportIntervalSeconds"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

func (s *Store) Summary() map[string]any {
	nodes := s.ListNodes()
	online := 0
	var cpu, memory, disk, throughput float64
	alerts := 0
	success := 0
	results := 0
	for _, node := range nodes {
		if node.Online {
			online++
		}
		cpu += node.CPUUsage
		memory += node.MemoryUsage
		disk += node.DiskUsage
		throughput += node.NetworkRxMB + node.NetworkTxMB
		if node.AlertLevel == "critical" || node.AlertLevel == "warning" {
			alerts++
		}
	}
	for _, task := range s.Tasks() {
		for _, result := range task.Results {
			results++
			if result.Status == "success" {
				success++
			}
		}
	}

	count := float64(len(nodes))
	if count == 0 {
		count = 1
	}
	taskSuccessRate := 100.0
	if results > 0 {
		taskSuccessRate = (float64(success) / float64(results)) * 100
	}

	return map[string]any{
		"totalNodes":        len(nodes),
		"onlineNodes":       online,
		"offlineNodes":      len(nodes) - online,
		"averageCPU":        cpu / count,
		"averageMemory":     memory / count,
		"averageDisk":       disk / count,
		"alertCount":        alerts,
		"runningTasks":      0,
		"updatedAt":         time.Now().UTC(),
		"hotNodes":          nodes,
		"taskSuccessRate":   taskSuccessRate,
		"networkThroughput": throughput,
	}
}

func NewTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func isOnline(lastSeenAt time.Time) bool {
	return time.Since(lastSeenAt) <= 45*time.Second
}
