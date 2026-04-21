package app

import (
	"strings"

	"metalx/controller/internal/store"
)

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func choosePrimaryNetwork(interfaces []*storeInterfaceShim, ips, macs []string) (string, string) {
	for _, iface := range interfaces {
		if iface.Name == "docker0" || strings.HasPrefix(iface.Name, "veth") || iface.IP == "" {
			continue
		}
		return iface.IP, iface.MAC
	}
	return first(ips), first(macs)
}

type storeInterfaceShim struct {
	Name string
	IP   string
	MAC  string
}

func fallback(value, replacement string) string {
	if value != "" {
		return value
	}
	return replacement
}

func alertLevel(cpu, memory, disk float64) string {
	switch {
	case cpu >= 90 || memory >= 90 || disk >= 90:
		return "critical"
	case cpu >= 70 || memory >= 70 || disk >= 80:
		return "warning"
	default:
		return "normal"
	}
}

func summarizeTaskStatus(results []store.TaskResult) string {
	if len(results) == 0 {
		return "completed"
	}
	hasFailure := false
	hasSuccess := false
	for _, result := range results {
		if result.Status == "success" {
			hasSuccess = true
		} else {
			hasFailure = true
		}
	}
	switch {
	case hasSuccess && hasFailure:
		return "partial"
	case hasFailure:
		return "failed"
	default:
		return "completed"
	}
}
