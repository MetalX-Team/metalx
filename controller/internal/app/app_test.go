package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metalx.local/proto/metalxpb"
	"metalx/controller/internal/config"
)

func TestAlertLevel(t *testing.T) {
	testCases := []struct {
		name   string
		cpu    float64
		memory float64
		disk   float64
		want   string
	}{
		{name: "normal", cpu: 32, memory: 41, disk: 38, want: "normal"},
		{name: "warning", cpu: 75, memory: 40, disk: 38, want: "warning"},
		{name: "critical", cpu: 32, memory: 91, disk: 38, want: "critical"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := alertLevel(tc.cpu, tc.memory, tc.disk)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestUpdateDnsmasqSettingsPersistsRenderedFiles(t *testing.T) {
	stateDir := t.TempDir()
	app, err := New(config.Config{
		ListenAddress:     ":0",
		GRPCListenAddress: ":0",
		DiscoveryPort:     9527,
		DatabasePath:      filepath.Join(stateDir, "controller.sqlite"),
		DNSMasqStateDir:   filepath.Join(stateDir, "dnsmasq"),
		AllowedShell:      true,
		DefaultNodeAddr:   "127.0.0.1:19091",
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	defer func() {
		_ = app.store.Close()
	}()

	resp, err := app.UpdateDnsmasqSettings(context.Background(), &metalxpb.UpdateDnsmasqSettingsRequest{
		Actor: "test",
		Settings: &metalxpb.DnsmasqSettings{
			Enabled:         true,
			ListenInterface: "eno1",
			DhcpRangeStart:  "10.10.0.10",
			DhcpRangeEnd:    "10.10.0.50",
			DhcpLeaseTime:   "6h",
			Gateway:         "10.10.0.1",
			DnsServers:      []string{"10.10.0.1", "1.1.1.1"},
			TftpRoot:        filepath.Join(stateDir, "tftp"),
			BootFile:        "pxelinux.0",
			PxePrompt:       "Boot from network",
			PxeServiceLabel: "MetalX PXE",
			KernelPath:      "images/metalx/vmlinuz",
			InitrdPath:      "images/metalx/initrd.img",
			BootArgs:        "ip=dhcp",
			NextServer:      "10.10.0.2",
		},
	})
	if err != nil {
		t.Fatalf("update dnsmasq settings: %v", err)
	}
	if !strings.Contains(resp.GetRenderedConfig(), "dhcp-range=10.10.0.10,10.10.0.50,6h") {
		t.Fatalf("rendered config missing dhcp range: %s", resp.GetRenderedConfig())
	}
	if !strings.Contains(resp.GetRenderedPxeMenu(), "KERNEL images/metalx/vmlinuz") {
		t.Fatalf("rendered PXE menu missing kernel path: %s", resp.GetRenderedPxeMenu())
	}
	if _, err := os.Stat(resp.GetConfigPath()); err != nil {
		t.Fatalf("expected dnsmasq config file: %v", err)
	}
	if _, err := os.Stat(resp.GetPxeConfigPath()); err != nil {
		t.Fatalf("expected PXE menu file: %v", err)
	}
}
