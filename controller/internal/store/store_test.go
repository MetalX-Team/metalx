package store

import (
	"testing"
	"time"
)

func TestSummaryStartsEmpty(t *testing.T) {
	s, err := New(t.TempDir() + "/controller.sqlite")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	summary := s.Summary()
	if got := summary["totalNodes"]; got != 0 {
		t.Fatalf("expected 0 nodes, got %v", got)
	}
	if got := summary["onlineNodes"]; got != 0 {
		t.Fatalf("expected 0 online nodes, got %v", got)
	}
}

func TestAddTaskPrependsNewestTask(t *testing.T) {
	s, err := New(t.TempDir() + "/controller.sqlite")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	first := Task{ID: "task-1", Command: "uptime", StartedAt: time.Now().UTC().Add(-time.Minute)}
	second := Task{ID: "task-2", Command: "date", StartedAt: time.Now().UTC()}

	s.AddTask(first)
	s.AddTask(second)

	tasks := s.Tasks()
	if len(tasks) < 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "task-2" {
		t.Fatalf("expected newest task first, got %s", tasks[0].ID)
	}
}

func TestSaveAndLoadDnsmasqSettings(t *testing.T) {
	s, err := New(t.TempDir() + "/controller.sqlite")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	expected := DnsmasqSettings{
		Enabled:         true,
		ListenInterface: "eno1",
		DHCPRangeStart:  "10.0.0.10",
		DHCPRangeEnd:    "10.0.0.50",
		DHCPLeaseTime:   "8h",
		Gateway:         "10.0.0.1",
		DNSServers:      []string{"10.0.0.1"},
		TFTPRoot:        "/srv/tftp",
		BootFile:        "pxelinux.0",
		PXEServiceLabel: "MetalX",
		KernelPath:      "images/vmlinuz",
		InitrdPath:      "images/initrd.img",
		NextServer:      "10.0.0.2",
		UpdatedAt:       time.Now().UTC(),
	}
	if err := s.SaveDnsmasqSettings(expected); err != nil {
		t.Fatalf("save dnsmasq settings: %v", err)
	}

	got, ok := s.LoadDnsmasqSettings()
	if !ok {
		t.Fatalf("expected dnsmasq settings to exist")
	}
	if got.ListenInterface != expected.ListenInterface {
		t.Fatalf("expected interface %s, got %s", expected.ListenInterface, got.ListenInterface)
	}
	if len(got.DNSServers) != 1 || got.DNSServers[0] != "10.0.0.1" {
		t.Fatalf("unexpected DNS servers: %#v", got.DNSServers)
	}
}
