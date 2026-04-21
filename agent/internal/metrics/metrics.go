package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

type Snapshot struct {
	NodeID       string           `json:"nodeId"`
	Hostname     string           `json:"hostname"`
	OS           string           `json:"os"`
	Kernel       string           `json:"kernel"`
	GrpcAddress  string           `json:"grpcAddress"`
	Uptime       string           `json:"uptime"`
	CPUUsage     float64          `json:"cpuUsage"`
	MemoryUsage  float64          `json:"memoryUsage"`
	DiskUsage    float64          `json:"diskUsage"`
	Load1        float64          `json:"load1"`
	Load5        float64          `json:"load5"`
	Load15       float64          `json:"load15"`
	NetworkRxMB  float64          `json:"networkRxMb"`
	NetworkTxMB  float64          `json:"networkTxMb"`
	DiskReadMB   float64          `json:"diskReadMb"`
	DiskWriteMB  float64          `json:"diskWriteMb"`
	IPAddresses  []string         `json:"ipAddresses"`
	MACAddresses []string         `json:"macAddresses"`
	ProcessCount int              `json:"processCount"`
	UserCount    int              `json:"userCount"`
	Interfaces   []InterfaceInfo  `json:"interfaces"`
	Filesystems  []FilesystemInfo `json:"filesystems"`
	LoggedUsers  []LoggedUser     `json:"loggedUsers"`
	TopProcesses []ProcessInfo    `json:"topProcesses"`
	RecentAlerts []AlertInfo      `json:"recentAlerts"`
	CollectedAt  time.Time        `json:"collectedAt"`
}

type sampleCache struct {
	mu          sync.Mutex
	lastCPU     cpuStat
	lastNetRx   uint64
	lastNetTx   uint64
	lastDiskR   uint64
	lastDiskW   uint64
	initialized bool
}

type cpuStat struct {
	idle  uint64
	total uint64
}

var cache sampleCache

func Collect(nodeID string) Snapshot {
	host, _ := os.Hostname()
	osName := readOSName()
	kernel := readKernel()
	uptime := readUptime()
	load1, load5, load15 := readLoadAvg()
	memUsage := readMemoryUsage()
	diskUsage, filesystems := readFilesystems()
	ipAddresses, macAddresses, interfaces, netRxTotal, netTxTotal := readInterfaces()
	processCount := readProcessCount()
	users := readLoggedUsers()
	topProcesses := readTopProcesses()
	cpuUsage := readCPUUsage()
	diskReadMB, diskWriteMB := readDiskIO()
	alerts := deriveAlerts(cpuUsage, memUsage, diskUsage, load1)

	return Snapshot{
		NodeID:       nodeID,
		Hostname:     host,
		OS:           osName,
		Kernel:       kernel,
		Uptime:       uptime,
		CPUUsage:     cpuUsage,
		MemoryUsage:  memUsage,
		DiskUsage:    diskUsage,
		Load1:        load1,
		Load5:        load5,
		Load15:       load15,
		NetworkRxMB:  netRxTotal,
		NetworkTxMB:  netTxTotal,
		DiskReadMB:   diskReadMB,
		DiskWriteMB:  diskWriteMB,
		IPAddresses:  ipAddresses,
		MACAddresses: macAddresses,
		ProcessCount: processCount,
		UserCount:    len(users),
		Interfaces:   interfaces,
		Filesystems:  filesystems,
		LoggedUsers:  users,
		TopProcesses: topProcesses,
		RecentAlerts: alerts,
		CollectedAt:  time.Now().UTC(),
	}
}

func readOSName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "linux"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(line[len("PRETTY_NAME="):], `"`)
		}
	}
	return "linux"
}

func readKernel() string {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return "unknown"
	}
	return charsToString(u.Release[:])
}

func readUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "unknown"
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "unknown"
	}
	return (time.Duration(seconds) * time.Second).Truncate(time.Minute).String()
}

func readLoadAvg() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	return parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
}

func readMemoryUsage() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, available float64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = parseFloat(fields[1])
		case "MemAvailable:":
			available = parseFloat(fields[1])
		}
	}
	if total == 0 {
		return 0
	}
	return ((total - available) / total) * 100
}

func readFilesystems() (float64, []FilesystemInfo) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return 0, nil
	}

	ignore := map[string]bool{
		"proc": true, "sysfs": true, "tmpfs": true, "devtmpfs": true, "devpts": true,
		"cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true, "nsfs": true,
		"mqueue": true, "tracefs": true, "fusectl": true, "securityfs": true,
	}

	seen := map[string]bool{}
	out := make([]FilesystemInfo, 0, 8)
	var totalBytes uint64
	var usedBytes uint64

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mountpoint := fields[1]
		fsType := fields[2]
		if ignore[fsType] || seen[mountpoint] || strings.HasPrefix(mountpoint, "/snap") {
			continue
		}
		seen[mountpoint] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountpoint, &stat); err != nil || stat.Blocks == 0 {
			continue
		}

		size := stat.Blocks * uint64(stat.Bsize)
		available := stat.Bavail * uint64(stat.Bsize)
		used := size - available
		usedPct := (float64(used) / float64(size)) * 100

		totalBytes += size
		usedBytes += used
		out = append(out, FilesystemInfo{
			Mount:       mountpoint,
			Size:        humanBytes(size),
			UsedPercent: usedPct,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	if totalBytes == 0 {
		return 0, out
	}
	return (float64(usedBytes) / float64(totalBytes)) * 100, out
}

func readInterfaces() ([]string, []string, []InterfaceInfo, float64, float64) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, nil, 0, 0
	}

	rxByName, txByName := readNetDev()
	var ips []string
	var macs []string
	var details []InterfaceInfo
	var totalRx, totalTx uint64

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		ip := ""
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				ip = ipNet.IP.String()
				ips = append(ips, ip)
			}
		}
		mac := iface.HardwareAddr.String()
		if mac != "" {
			macs = append(macs, mac)
		}
		state := "down"
		if iface.Flags&net.FlagUp != 0 {
			state = "up"
		}
		rx := rxByName[iface.Name]
		tx := txByName[iface.Name]
		totalRx += rx
		totalTx += tx
		details = append(details, InterfaceInfo{
			Name:  iface.Name,
			IP:    ip,
			MAC:   mac,
			State: state,
			RxMB:  fmt.Sprintf("%.2f MB", float64(rx)/1024/1024),
			TxMB:  fmt.Sprintf("%.2f MB", float64(tx)/1024/1024),
		})
	}

	sort.Strings(ips)
	sort.Strings(macs)
	sort.Slice(details, func(i, j int) bool { return details[i].Name < details[j].Name })
	return uniqueStrings(ips), uniqueStrings(macs), details, float64(totalRx) / 1024 / 1024, float64(totalTx) / 1024 / 1024
}

func readNetDev() (map[string]uint64, map[string]uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return map[string]uint64{}, map[string]uint64{}
	}
	rx := map[string]uint64{}
	tx := map[string]uint64{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines[2:] {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rx[name] = parseUint(fields[0])
		tx[name] = parseUint(fields[8])
	}
	return rx, tx
}

func readProcessCount() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && isDigits(entry.Name()) {
			count++
		}
	}
	return count
}

func readLoggedUsers() []LoggedUser {
	cmd := exec.Command("/usr/bin/who")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	users := make([]LoggedUser, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		from := strings.Trim(fields[len(fields)-1], "()")
		users = append(users, LoggedUser{
			User: fields[0],
			TTY:  fields[1],
			From: from,
		})
	}
	return users
}

func readTopProcesses() []ProcessInfo {
	cmd := exec.Command("/bin/sh", "-lc", "ps -eo pid,comm,%cpu,%mem --sort=-%cpu | head -n 6")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	processes := make([]ProcessInfo, 0, 5)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		processes = append(processes, ProcessInfo{
			PID:  int(parseUint(fields[0])),
			Name: fields[1],
			CPU:  parseFloat(fields[2]),
			Mem:  parseFloat(fields[3]),
		})
	}
	return processes
}

func readCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return 0
	}
	var total uint64
	for _, field := range fields[1:] {
		total += parseUint(field)
	}
	idle := parseUint(fields[4]) + parseUint(fields[5])

	cache.mu.Lock()
	defer cache.mu.Unlock()
	current := cpuStat{idle: idle, total: total}
	if !cache.initialized {
		cache.lastCPU = current
		cache.initialized = true
		return 0
	}
	totalDelta := current.total - cache.lastCPU.total
	idleDelta := current.idle - cache.lastCPU.idle
	cache.lastCPU = current
	if totalDelta == 0 {
		return 0
	}
	return (1 - float64(idleDelta)/float64(totalDelta)) * 100
}

func readDiskIO() (float64, float64) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return 0, 0
	}
	var sectorsRead uint64
	var sectorsWritten uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if shouldIgnoreBlock(name) {
			continue
		}
		sectorsRead += parseUint(fields[5])
		sectorsWritten += parseUint(fields[9])
	}
	return float64(sectorsRead*512) / 1024 / 1024, float64(sectorsWritten*512) / 1024 / 1024
}

func deriveAlerts(cpu, memory, disk, load1 float64) []AlertInfo {
	alerts := make([]AlertInfo, 0, 4)
	now := time.Now().UTC().Format(time.RFC3339)
	if cpu >= 85 {
		alerts = append(alerts, AlertInfo{Severity: "critical", Message: "CPU usage above 85%", At: now})
	} else if cpu >= 70 {
		alerts = append(alerts, AlertInfo{Severity: "warning", Message: "CPU usage above 70%", At: now})
	}
	if memory >= 85 {
		alerts = append(alerts, AlertInfo{Severity: "critical", Message: "Memory usage above 85%", At: now})
	} else if memory >= 70 {
		alerts = append(alerts, AlertInfo{Severity: "warning", Message: "Memory usage above 70%", At: now})
	}
	if disk >= 90 {
		alerts = append(alerts, AlertInfo{Severity: "critical", Message: "Disk usage above 90%", At: now})
	} else if disk >= 80 {
		alerts = append(alerts, AlertInfo{Severity: "warning", Message: "Disk usage above 80%", At: now})
	}
	if load1 >= 4 {
		alerts = append(alerts, AlertInfo{Severity: "warning", Message: "Load average above 4.0", At: now})
	}
	return alerts
}

func parseFloat(value string) float64 {
	number, _ := strconv.ParseFloat(value, 64)
	return number
}

func parseUint(value string) uint64 {
	number, _ := strconv.ParseUint(value, 10, 64)
	return number
}

func charsToString(values []int8) string {
	buf := make([]byte, 0, len(values))
	for _, v := range values {
		if v == 0 {
			break
		}
		buf = append(buf, byte(v))
	}
	return string(buf)
}

func humanBytes(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := uint64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func shouldIgnoreBlock(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") {
		return true
	}
	matches, _ := filepath.Match("dm-*", name)
	return matches
}
