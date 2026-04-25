package app

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"metalx/controller/internal/store"
)

func (a *App) getDnsmasqSettings() store.DnsmasqSettings {
	settings, ok := a.store.LoadDnsmasqSettings()
	if !ok {
		settings = a.defaultDnsmasqSettings()
	}
	return a.materializeDnsmasqSettings(settings)
}

func (a *App) defaultDnsmasqSettings() store.DnsmasqSettings {
	tftpRoot := filepath.Join(a.cfg.DNSMasqStateDir, "tftp-root")
	return store.DnsmasqSettings{
		Enabled:         false,
		ListenInterface: "eth0",
		DHCPRangeStart:  "192.168.56.100",
		DHCPRangeEnd:    "192.168.56.180",
		DHCPLeaseTime:   "12h",
		Gateway:         "192.168.56.1",
		DNSServers:      []string{"1.1.1.1", "8.8.8.8"},
		TFTPRoot:        tftpRoot,
		BootFile:        "pxelinux.0",
		PXEPrompt:       "Press F8 to launch MetalX PXE installer",
		PXEServiceLabel: "MetalX Network Boot",
		KernelPath:      "images/metalx/vmlinuz",
		InitrdPath:      "images/metalx/initrd.img",
		BootArgs:        "ip=dhcp inst.repo=http://192.168.56.1/os",
		NextServer:      "192.168.56.1",
		UpdatedAt:       time.Now().UTC(),
	}
}

func (a *App) materializeDnsmasqSettings(settings store.DnsmasqSettings) store.DnsmasqSettings {
	tftpRoot := strings.TrimSpace(settings.TFTPRoot)
	if tftpRoot == "" {
		tftpRoot = filepath.Join(a.cfg.DNSMasqStateDir, "tftp-root")
	}
	settings.TFTPRoot = tftpRoot
	settings.BootFile = fallback(strings.TrimSpace(settings.BootFile), "pxelinux.0")
	settings.ListenInterface = fallback(strings.TrimSpace(settings.ListenInterface), "eth0")
	settings.DHCPLeaseTime = fallback(strings.TrimSpace(settings.DHCPLeaseTime), "12h")
	settings.PXEServiceLabel = fallback(strings.TrimSpace(settings.PXEServiceLabel), "MetalX Network Boot")
	settings.PXEPrompt = fallback(strings.TrimSpace(settings.PXEPrompt), "Press F8 to launch MetalX PXE installer")
	settings.ConfigPath = filepath.Join(a.cfg.DNSMasqStateDir, "dnsmasq.conf")
	settings.PXEConfigPath = filepath.Join(tftpRoot, "pxelinux.cfg", "default")
	settings.RenderedConfig = renderDnsmasqConfig(settings)
	settings.RenderedPXEMenu = renderPXEMenu(settings)
	if settings.UpdatedAt.IsZero() {
		settings.UpdatedAt = time.Now().UTC()
	}
	return settings
}

func validateDnsmasqSettings(settings store.DnsmasqSettings) error {
	required := map[string]string{
		"listenInterface": settings.ListenInterface,
		"dhcpRangeStart":  settings.DHCPRangeStart,
		"dhcpRangeEnd":    settings.DHCPRangeEnd,
		"gateway":         settings.Gateway,
		"tftpRoot":        settings.TFTPRoot,
		"bootFile":        settings.BootFile,
		"kernelPath":      settings.KernelPath,
		"initrdPath":      settings.InitrdPath,
		"nextServer":      settings.NextServer,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	for _, ip := range []string{settings.DHCPRangeStart, settings.DHCPRangeEnd, settings.Gateway, settings.NextServer} {
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid IP address: %s", ip)
		}
	}
	for _, dnsServer := range settings.DNSServers {
		if net.ParseIP(strings.TrimSpace(dnsServer)) == nil {
			return fmt.Errorf("invalid DNS server: %s", dnsServer)
		}
	}
	return nil
}

func (a *App) persistDnsmasqSettings(settings store.DnsmasqSettings) error {
	if err := os.MkdirAll(a.cfg.DNSMasqStateDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(settings.PXEConfigPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(settings.TFTPRoot, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(settings.ConfigPath, []byte(settings.RenderedConfig), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(settings.PXEConfigPath, []byte(settings.RenderedPXEMenu), 0o644); err != nil {
		return err
	}
	return a.store.SaveDnsmasqSettings(settings)
}

func renderDnsmasqConfig(settings store.DnsmasqSettings) string {
	lines := []string{
		"# Managed by MetalX. Edit from the dashboard.",
		"port=0",
		"bind-interfaces",
		fmt.Sprintf("interface=%s", settings.ListenInterface),
	}
	if settings.BindAddress != "" {
		lines = append(lines, fmt.Sprintf("listen-address=%s", settings.BindAddress))
	}
	if settings.Enabled {
		lines = append(lines,
			fmt.Sprintf("dhcp-range=%s,%s,%s", settings.DHCPRangeStart, settings.DHCPRangeEnd, settings.DHCPLeaseTime),
			fmt.Sprintf("dhcp-option=option:router,%s", settings.Gateway),
		)
		for _, dnsServer := range settings.DNSServers {
			lines = append(lines, fmt.Sprintf("dhcp-option=option:dns-server,%s", strings.TrimSpace(dnsServer)))
		}
		lines = append(lines,
			"enable-tftp",
			fmt.Sprintf("tftp-root=%s", settings.TFTPRoot),
			fmt.Sprintf("dhcp-boot=%s,,%s", settings.BootFile, settings.NextServer),
			fmt.Sprintf("pxe-prompt=%s,5", settings.PXEPrompt),
			fmt.Sprintf("pxe-service=X86PC,\"%s\",%s", settings.PXEServiceLabel, settings.BootFile),
		)
	} else {
		lines = append(lines, "# PXE service is disabled")
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderPXEMenu(settings store.DnsmasqSettings) string {
	lines := []string{
		"DEFAULT metalx",
		"PROMPT 0",
		fmt.Sprintf("MENU TITLE %s", settings.PXEServiceLabel),
		"LABEL metalx",
		fmt.Sprintf("  MENU LABEL %s", settings.PXEServiceLabel),
		fmt.Sprintf("  KERNEL %s", settings.KernelPath),
		fmt.Sprintf("  INITRD %s", settings.InitrdPath),
	}
	bootArgs := strings.TrimSpace(settings.BootArgs)
	if bootArgs == "" {
		bootArgs = "ip=dhcp"
	}
	lines = append(lines, fmt.Sprintf("  APPEND %s", bootArgs))
	return strings.Join(lines, "\n") + "\n"
}
