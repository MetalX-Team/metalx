package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"metalx.local/proto/metalxpb"
	"metalx/controller/internal/store"
)

var macSanitizer = regexp.MustCompile(`[^0-9a-f]`)

func (a *App) ensureInstallProfiles() error {
	settings := a.getAppSettings()
	if len(a.store.ListInstallProfiles()) > 0 {
		return nil
	}
	now := time.Now().UTC()
	defaults := []store.InstallProfile{
		{
			ID:                 "ubuntu-24.04",
			Name:               "Ubuntu 24.04 LTS",
			OSFamily:           "ubuntu",
			OSVersion:          "24.04",
			Architecture:       "amd64",
			Firmware:           "uefi",
			InstallSource:      "http://mirror.local/ubuntu/ubuntu-24.04-live-server-amd64.iso",
			BootKernelPath:     "casper/vmlinuz",
			BootInitrdPath:     "casper/initrd",
			HostnamePattern:    "ubuntu-${mac}",
			Timezone:           "Etc/UTC",
			Locale:             "en_US.UTF-8",
			KeyboardLayout:     "us",
			AdminUsername:      "metalx",
			DiskLayout:         "direct",
			NetworkMode:        "dhcp",
			AgentServiceName:   "metalx-agent",
			ControllerGRPCAddr: settings.PublicGRPCAddress,
			Enabled:            true,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                 "debian-12",
			Name:               "Debian 12",
			OSFamily:           "debian",
			OSVersion:          "12",
			Architecture:       "amd64",
			Firmware:           "uefi",
			InstallSource:      "http://mirror.local/debian/dists/bookworm/main/installer-amd64/",
			BootKernelPath:     "netboot/debian-installer/amd64/linux",
			BootInitrdPath:     "netboot/debian-installer/amd64/initrd.gz",
			HostnamePattern:    "debian-${mac}",
			Timezone:           "Etc/UTC",
			Locale:             "en_US.UTF-8",
			KeyboardLayout:     "us",
			AdminUsername:      "metalx",
			DiskLayout:         "atomic",
			NetworkMode:        "dhcp",
			AgentServiceName:   "metalx-agent",
			ControllerGRPCAddr: settings.PublicGRPCAddress,
			Enabled:            true,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                 "fedora-41",
			Name:               "Fedora Server 41",
			OSFamily:           "fedora",
			OSVersion:          "41",
			Architecture:       "amd64",
			Firmware:           "uefi",
			InstallSource:      "http://mirror.local/fedora/releases/41/Server/x86_64/os/",
			BootKernelPath:     "images/pxeboot/vmlinuz",
			BootInitrdPath:     "images/pxeboot/initrd.img",
			HostnamePattern:    "fedora-${mac}",
			Timezone:           "Etc/UTC",
			Locale:             "en_US.UTF-8",
			KeyboardLayout:     "us",
			AdminUsername:      "metalx",
			DiskLayout:         "lvm",
			NetworkMode:        "dhcp",
			AgentServiceName:   "metalx-agent",
			ControllerGRPCAddr: settings.PublicGRPCAddress,
			Enabled:            true,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                 "centos-stream-9",
			Name:               "CentOS Stream 9",
			OSFamily:           "centos",
			OSVersion:          "9",
			Architecture:       "amd64",
			Firmware:           "uefi",
			InstallSource:      "http://mirror.local/centos-stream/9-stream/BaseOS/x86_64/os/",
			BootKernelPath:     "images/pxeboot/vmlinuz",
			BootInitrdPath:     "images/pxeboot/initrd.img",
			HostnamePattern:    "centos-${mac}",
			Timezone:           "Etc/UTC",
			Locale:             "en_US.UTF-8",
			KeyboardLayout:     "us",
			AdminUsername:      "metalx",
			DiskLayout:         "lvm",
			NetworkMode:        "dhcp",
			AgentServiceName:   "metalx-agent",
			ControllerGRPCAddr: settings.PublicGRPCAddress,
			Enabled:            true,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                 "rhel-9",
			Name:               "RHEL 9",
			OSFamily:           "rhel",
			OSVersion:          "9",
			Architecture:       "amd64",
			Firmware:           "uefi",
			InstallSource:      "http://mirror.local/rhel/9/BaseOS/x86_64/os/",
			BootKernelPath:     "images/pxeboot/vmlinuz",
			BootInitrdPath:     "images/pxeboot/initrd.img",
			HostnamePattern:    "rhel-${mac}",
			Timezone:           "Etc/UTC",
			Locale:             "en_US.UTF-8",
			KeyboardLayout:     "us",
			AdminUsername:      "metalx",
			DiskLayout:         "lvm",
			NetworkMode:        "dhcp",
			AgentServiceName:   "metalx-agent",
			ControllerGRPCAddr: settings.PublicGRPCAddress,
			Enabled:            true,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	}
	for _, item := range defaults {
		if err := a.store.SaveInstallProfile(item); err != nil {
			return err
		}
	}
	return nil
}

func normalizeMAC(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	value = strings.ReplaceAll(value, "-", ":")
	parts := strings.Split(value, ":")
	if len(parts) == 6 {
		for i, part := range parts {
			if len(part) == 1 {
				parts[i] = "0" + part
			}
		}
		return strings.Join(parts, ":")
	}
	compact := macSanitizer.ReplaceAllString(value, "")
	if len(compact) == 12 {
		return strings.Join([]string{compact[0:2], compact[2:4], compact[4:6], compact[6:8], compact[8:10], compact[10:12]}, ":")
	}
	return value
}

func macSlug(macAddress string) string {
	return macSanitizer.ReplaceAllString(strings.ToLower(macAddress), "")
}

func expandHostname(pattern, macAddress, osFamily string) string {
	slug := macSlug(macAddress)
	if pattern == "" {
		return fmt.Sprintf("%s-%s", osFamily, slug)
	}
	return strings.NewReplacer("${mac}", slug, "${os}", osFamily).Replace(pattern)
}

func randomToken(prefix string) string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return prefix + "-" + hex.EncodeToString(buf)
}

func urlJoin(base, suffix string) string {
	if strings.HasSuffix(base, "/") {
		return base + strings.TrimPrefix(suffix, "/")
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}

func (a *App) renderInstallJob(profile store.InstallProfile, macAddress, hostname, nodeID string) store.InstallJob {
	settings := a.getAppSettings()
	now := time.Now().UTC()
	macAddress = normalizeMAC(macAddress)
	if hostname == "" {
		hostname = expandHostname(profile.HostnamePattern, macAddress, profile.OSFamily)
	}
	if nodeID == "" {
		nodeID = "node-" + macSlug(macAddress)
	}
	jobID := randomToken("install")
	token := randomToken("tok")
	job := store.InstallJob{
		ID:          jobID,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		OSFamily:    profile.OSFamily,
		Status:      "planned",
		MACAddress:  macAddress,
		Hostname:    hostname,
		NodeID:      nodeID,
		Token:       token,
		CreatedAt:   now,
		UpdatedAt:   now,
		Events: []store.InstallEvent{
			{Phase: "planned", Status: "planned", Message: "install job created", CreatedAt: now},
		},
	}
	job.BootURL = urlJoin(settings.ProvisioningBaseURL, "/boot/"+macAddress)
	job.AgentScriptURL = urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/agent-install/%s.sh", job.ID, job.Token))
	switch profile.OSFamily {
	case "ubuntu":
		job.ConfigURL = urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/seed/%s/user-data", job.ID, job.Token))
	case "debian":
		job.ConfigURL = urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/preseed/%s.cfg", job.ID, job.Token))
	default:
		job.ConfigURL = urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/kickstart/%s.ks", job.ID, job.Token))
	}
	job.BootPreview = a.renderIPXEScript(job, profile)
	job.ConfigPreview = a.renderInstallConfig(job, profile)
	job.LastEvent = "install job created"
	return job
}

func absoluteOrJoin(base, item string) string {
	if strings.HasPrefix(item, "http://") || strings.HasPrefix(item, "https://") {
		return item
	}
	return urlJoin(base, item)
}

func (a *App) renderIPXEScript(job store.InstallJob, profile store.InstallProfile) string {
	settings := a.getAppSettings()
	kernelURL := absoluteOrJoin(profile.InstallSource, profile.BootKernelPath)
	initrdURL := absoluteOrJoin(profile.InstallSource, profile.BootInitrdPath)
	switch profile.OSFamily {
	case "ubuntu":
		seedBase := urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/seed/%s/", job.ID, job.Token))
		return strings.Join([]string{
			"#!ipxe",
			"dhcp",
			fmt.Sprintf("kernel %s ip=dhcp url=%s autoinstall ds=nocloud-net;s=%s %s", kernelURL, profile.InstallSource, seedBase, strings.TrimSpace(profile.ExtraKernelArgs)),
			fmt.Sprintf("initrd %s", initrdURL),
			"boot",
			"",
		}, "\n")
	case "debian":
		return strings.Join([]string{
			"#!ipxe",
			"dhcp",
			fmt.Sprintf("kernel %s auto=true priority=critical url=%s hostname=%s interface=auto %s", kernelURL, job.ConfigURL, job.Hostname, strings.TrimSpace(profile.ExtraKernelArgs)),
			fmt.Sprintf("initrd %s", initrdURL),
			"boot",
			"",
		}, "\n")
	default:
		return strings.Join([]string{
			"#!ipxe",
			"dhcp",
			fmt.Sprintf("kernel %s initrd=initrd.img inst.repo=%s inst.ks=%s ip=dhcp %s", kernelURL, profile.InstallSource, job.ConfigURL, strings.TrimSpace(profile.ExtraKernelArgs)),
			fmt.Sprintf("initrd --name initrd.img %s", initrdURL),
			"boot",
			"",
		}, "\n")
	}
}

func shellQuote(input string) string {
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
}

func renderPackageLines(packages []string, defaultPackages ...string) []string {
	set := make([]string, 0, len(defaultPackages)+len(packages))
	set = append(set, defaultPackages...)
	set = append(set, packages...)
	return set
}

func (a *App) renderInstallConfig(job store.InstallJob, profile store.InstallProfile) string {
	switch profile.OSFamily {
	case "ubuntu":
		return a.renderUbuntuAutoinstall(job, profile)
	case "debian":
		return a.renderDebianPreseed(job, profile)
	default:
		return a.renderKickstart(job, profile)
	}
}

func (a *App) renderUbuntuAutoinstall(job store.InstallJob, profile store.InstallProfile) string {
	settings := a.getAppSettings()
	keys := make([]string, 0, len(profile.SSHAuthorizedKeys))
	for _, key := range profile.SSHAuthorizedKeys {
		keys = append(keys, fmt.Sprintf("      - %s", key))
	}
	packages := make([]string, 0, len(profile.Packages))
	for _, pkg := range renderPackageLines(profile.Packages, "curl", "ca-certificates") {
		packages = append(packages, fmt.Sprintf("    - %s", pkg))
	}
	lines := []string{
		"#cloud-config",
		"autoinstall:",
		"  version: 1",
		fmt.Sprintf("  locale: %s", fallback(profile.Locale, "en_US.UTF-8")),
		"  keyboard:",
		fmt.Sprintf("    layout: %s", fallback(profile.KeyboardLayout, "us")),
		fmt.Sprintf("  timezone: %s", fallback(profile.Timezone, "Etc/UTC")),
		"  identity:",
		fmt.Sprintf("    hostname: %s", job.Hostname),
		fmt.Sprintf("    username: %s", fallback(profile.AdminUsername, "metalx")),
		fmt.Sprintf("    password: %s", shellQuote(profile.AdminPasswordHash)),
		"  ssh:",
		"    install-server: true",
		"    allow-pw: true",
	}
	if len(keys) > 0 {
		lines = append(lines, "    authorized-keys:")
		lines = append(lines, keys...)
	}
	if len(packages) > 0 {
		lines = append(lines, "  packages:")
		lines = append(lines, packages...)
	}
	lines = append(lines,
		"  storage:",
		"    layout:",
		fmt.Sprintf("      name: %s", fallback(profile.DiskLayout, "direct")),
		"  late-commands:",
		fmt.Sprintf("    - curtin in-target -- /bin/bash -lc %s", shellQuote("curl -fsSL "+job.AgentScriptURL+" | bash")),
		fmt.Sprintf("    - curtin in-target -- /bin/bash -lc %s", shellQuote(fmt.Sprintf("curl -fsS -X POST -H 'Content-Type: application/json' -d '{\"status\":\"installing\",\"phase\":\"late-commands\",\"message\":\"ubuntu late-commands completed\",\"token\":\"%s\"}' %s", job.Token, urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events")))),
		"  error-commands:",
		fmt.Sprintf("    - curl -fsS -X POST -H 'Content-Type: application/json' -d %s %s || true", shellQuote(fmt.Sprintf("{\"status\":\"failed\",\"phase\":\"error\",\"message\":\"ubuntu install failed\",\"token\":\"%s\"}", job.Token)), urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events")),
	)
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) renderDebianPreseed(job store.InstallJob, profile store.InstallProfile) string {
	settings := a.getAppSettings()
	packages := strings.Join(renderPackageLines(profile.Packages, "curl", "ca-certificates"), " ")
	lateCmd := fmt.Sprintf("in-target /bin/bash -lc %s; wget -qO- --header='Content-Type: application/json' --post-data=%s %s || true",
		shellQuote("curl -fsSL "+job.AgentScriptURL+" | bash"),
		shellQuote(fmt.Sprintf("{\"status\":\"installing\",\"phase\":\"late-command\",\"message\":\"debian late command completed\",\"token\":\"%s\"}", job.Token)),
		urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events"),
	)
	lines := []string{
		fmt.Sprintf("d-i debian-installer/locale string %s", fallback(profile.Locale, "en_US.UTF-8")),
		fmt.Sprintf("d-i keyboard-configuration/xkb-keymap select %s", fallback(profile.KeyboardLayout, "us")),
		"d-i netcfg/choose_interface select auto",
		fmt.Sprintf("d-i netcfg/get_hostname string %s", job.Hostname),
		fmt.Sprintf("d-i passwd/user-fullname string %s", fallback(profile.AdminUsername, "metalx")),
		fmt.Sprintf("d-i passwd/username string %s", fallback(profile.AdminUsername, "metalx")),
		fmt.Sprintf("d-i passwd/user-password-crypted password %s", profile.AdminPasswordHash),
		"d-i clock-setup/utc boolean true",
		fmt.Sprintf("d-i time/zone string %s", fallback(profile.Timezone, "Etc/UTC")),
		"d-i partman-auto/method string regular",
		"d-i partman-auto/choose_recipe select atomic",
		"d-i partman-partitioning/confirm_write_new_label boolean true",
		"d-i partman/choose_partition select finish",
		"d-i partman/confirm boolean true",
		"d-i partman/confirm_nooverwrite boolean true",
		fmt.Sprintf("d-i pkgsel/include string %s", packages),
		"d-i grub-installer/only_debian boolean true",
		"d-i grub-installer/with_other_os boolean true",
		"d-i finish-install/reboot_in_progress note",
		fmt.Sprintf("d-i preseed/late_command string %s", lateCmd),
	}
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) renderKickstart(job store.InstallJob, profile store.InstallProfile) string {
	settings := a.getAppSettings()
	lines := []string{
		"text",
		"reboot",
		fmt.Sprintf("lang %s", fallback(profile.Locale, "en_US.UTF-8")),
		fmt.Sprintf("keyboard %s", fallback(profile.KeyboardLayout, "us")),
		fmt.Sprintf("timezone %s --utc", fallback(profile.Timezone, "Etc/UTC")),
		fmt.Sprintf("network --bootproto=dhcp --hostname=%s --activate", job.Hostname),
		fmt.Sprintf("url --url=%s", profile.InstallSource),
		"bootloader --location=boot",
		"zerombr",
		"clearpart --all --initlabel",
		fmt.Sprintf("autopart --type=%s", fallback(profile.DiskLayout, "lvm")),
		fmt.Sprintf("rootpw --iscrypted %s", profile.AdminPasswordHash),
		fmt.Sprintf("user --name=%s --groups=wheel --password=%s --iscrypted", fallback(profile.AdminUsername, "metalx"), profile.AdminPasswordHash),
		"%packages",
		"@^minimal-environment",
		"curl",
		"ca-certificates",
	}
	for _, pkg := range profile.Packages {
		lines = append(lines, pkg)
	}
	lines = append(lines,
		"%end",
		"%post --log=/root/metalx-post.log",
		fmt.Sprintf("curl -fsS -X POST -H 'Content-Type: application/json' -d '%s' %s || true", fmt.Sprintf("{\"status\":\"installing\",\"phase\":\"%%post\",\"message\":\"kickstart %%post started\",\"token\":\"%s\"}", job.Token), urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events")),
		fmt.Sprintf("curl -fsSL %s | bash", job.AgentScriptURL),
	)
	if script := strings.TrimSpace(profile.PostInstallScript); script != "" {
		lines = append(lines, script)
	}
	lines = append(lines,
		fmt.Sprintf("curl -fsS -X POST -H 'Content-Type: application/json' -d '%s' %s || true", fmt.Sprintf("{\"status\":\"installing\",\"phase\":\"%%post\",\"message\":\"kickstart %%post completed\",\"token\":\"%s\"}", job.Token), urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events")),
		"%end",
	)
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) renderAgentInstallScript(job store.InstallJob, profile store.InstallProfile) string {
	settings := a.getAppSettings()
	agentBinaryURL := profile.AgentBinaryURL
	if agentBinaryURL == "" {
		agentBinaryURL = urlJoin(settings.ProvisioningBaseURL, fmt.Sprintf("/provisioning/jobs/%s/artifacts/%s/mxagent", job.ID, job.Token))
	}
	serviceName := fallback(profile.AgentServiceName, "metalx-agent")
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

install -d -m 0755 /usr/local/bin /etc/metalx
curl -fsSL %s -o /usr/local/bin/mxagent
chmod 0755 /usr/local/bin/mxagent
cat >/etc/metalx/agent.env <<'EOF'
MX_AGENT_ID=%s
MX_AGENT_NAME=%s
MX_CONTROLLER_ADDR=%s
MX_AGENT_LISTEN=%s
MX_AGENT_GRPC_LISTEN=%s
MX_AGENT_INTERVAL=%ds
EOF
cat >/etc/systemd/system/%s.service <<'EOF'
[Unit]
Description=MetalX Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/metalx/agent.env
ExecStart=/bin/sh -c '/usr/local/bin/mxagent --id "$MX_AGENT_ID" --name "$MX_AGENT_NAME" --controller "$MX_CONTROLLER_ADDR" --listen "$MX_AGENT_LISTEN" --grpc-listen "$MX_AGENT_GRPC_LISTEN" --interval "$MX_AGENT_INTERVAL"'
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now %s.service
curl -fsS -X POST -H 'Content-Type: application/json' -d '{"status":"managed","phase":"agent","message":"agent service enabled","token":"%s"}' %s || true
`, agentBinaryURL, job.NodeID, job.Hostname, fallback(profile.ControllerGRPCAddr, settings.PublicGRPCAddress), settings.AgentListenAddress, settings.AgentGRPCListenAddress, settings.AgentReportIntervalSeconds, serviceName, serviceName, job.Token, urlJoin(settings.ProvisioningBaseURL, "/provisioning/jobs/"+job.ID+"/events"))
}

func (a *App) authorizeJob(jobID, token string) (store.InstallJob, store.InstallProfile, bool) {
	job, ok := a.store.GetInstallJob(jobID)
	if !ok || job.Token != token {
		return store.InstallJob{}, store.InstallProfile{}, false
	}
	profile, ok := a.store.GetInstallProfile(job.ProfileID)
	if !ok {
		return store.InstallJob{}, store.InstallProfile{}, false
	}
	return job, profile, true
}

func (a *App) updateInstallJobStatus(job *store.InstallJob, statusValue, message, phase string) {
	now := time.Now().UTC()
	if statusValue != "" {
		job.Status = statusValue
	}
	job.LastEvent = message
	job.UpdatedAt = now
	job.Events = append(job.Events, store.InstallEvent{
		Phase:     phase,
		Status:    fallback(statusValue, job.Status),
		Message:   message,
		CreatedAt: now,
	})
	_ = a.store.SaveInstallJob(*job)
}

func (a *App) ListInstallProfiles(_ context.Context, _ *metalxpb.Empty) (*metalxpb.ListInstallProfilesResponse, error) {
	items := a.store.ListInstallProfiles()
	result := make([]*metalxpb.InstallProfile, 0, len(items))
	for _, item := range items {
		copyItem := item
		result = append(result, installProfileToProto(copyItem))
	}
	return &metalxpb.ListInstallProfilesResponse{Items: result}, nil
}

func (a *App) UpsertInstallProfile(_ context.Context, payload *metalxpb.UpsertInstallProfileRequest) (*metalxpb.InstallProfile, error) {
	profile := installProfileFromProto(payload.GetProfile())
	now := time.Now().UTC()
	if profile.ID == "" {
		profile.ID = randomToken("profile")
		profile.CreatedAt = now
	} else if current, ok := a.store.GetInstallProfile(profile.ID); ok {
		profile.CreatedAt = current.CreatedAt
	}
	profile.UpdatedAt = now
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	if profile.Name == "" || profile.OSFamily == "" || profile.InstallSource == "" {
		return nil, grpcInvalid("profile name, os family and install source are required")
	}
	if err := a.store.SaveInstallProfile(profile); err != nil {
		return nil, err
	}
	a.store.AddAudit(store.AuditRecord{
		ID:        "audit-" + randomToken("profile"),
		Actor:     fallback(payload.GetActor(), "dashboard"),
		Action:    "upsert_install_profile",
		Target:    profile.ID,
		CreatedAt: now,
	})
	return installProfileToProto(profile), nil
}

func (a *App) ListInstallJobs(_ context.Context, _ *metalxpb.Empty) (*metalxpb.ListInstallJobsResponse, error) {
	items := a.store.ListInstallJobs()
	result := make([]*metalxpb.InstallJob, 0, len(items))
	for _, item := range items {
		copyItem := item
		result = append(result, installJobToProto(copyItem))
	}
	return &metalxpb.ListInstallJobsResponse{Items: result}, nil
}

func (a *App) CreateInstallJob(_ context.Context, payload *metalxpb.CreateInstallJobRequest) (*metalxpb.InstallJob, error) {
	profile, ok := a.store.GetInstallProfile(payload.GetProfileId())
	if !ok {
		return nil, grpcNotFound("install profile not found")
	}
	if !profile.Enabled {
		return nil, grpcForbidden("install profile is disabled")
	}
	macAddress := normalizeMAC(payload.GetMacAddress())
	if macAddress == "" {
		return nil, grpcInvalid("mac address is required")
	}
	job := a.renderInstallJob(profile, macAddress, payload.GetHostname(), payload.GetNodeId())
	if err := a.store.SaveInstallJob(job); err != nil {
		return nil, err
	}
	a.store.AddAudit(store.AuditRecord{
		ID:        "audit-" + job.ID,
		Actor:     fallback(payload.GetActor(), "dashboard"),
		Action:    "create_install_job",
		Target:    job.MACAddress,
		CreatedAt: job.CreatedAt,
	})
	return installJobToProto(job), nil
}

func (a *App) GetInstallJob(_ context.Context, payload *metalxpb.InstallJobID) (*metalxpb.InstallJob, error) {
	job, ok := a.store.GetInstallJob(payload.GetId())
	if !ok {
		return nil, grpcNotFound("install job not found")
	}
	return installJobToProto(job), nil
}

func (a *App) handleBoot(w http.ResponseWriter, r *http.Request) {
	macAddress := strings.TrimPrefix(r.URL.Path, "/boot/")
	job, ok := a.store.GetInstallJobByMAC(normalizeMAC(macAddress))
	if !ok {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("#!ipxe\nexit\n"))
		return
	}
	profile, ok := a.store.GetInstallProfile(job.ProfileID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.updateInstallJobStatus(&job, "pxe-armed", "boot script served", "boot")
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(a.renderIPXEScript(job, profile)))
}

func (a *App) handleProvisioningSeed(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/provisioning/jobs/"), "/")
	if len(parts) < 4 || parts[1] != "seed" {
		http.NotFound(w, r)
		return
	}
	job, profile, ok := a.authorizeJob(parts[0], parts[2])
	if !ok || profile.OSFamily != "ubuntu" {
		http.NotFound(w, r)
		return
	}
	switch parts[3] {
	case "user-data":
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(a.renderUbuntuAutoinstall(job, profile)))
	case "meta-data":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", job.ID, job.Hostname)))
	default:
		http.NotFound(w, r)
	}
}

func (a *App) handleProvisioningConfig(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/provisioning/jobs/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	jobID := parts[0]
	segment := parts[1]
	tokenAndSuffix := parts[2]
	token := strings.TrimSuffix(strings.TrimSuffix(tokenAndSuffix, ".cfg"), ".ks")
	job, profile, ok := a.authorizeJob(jobID, token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch {
	case segment == "preseed" && profile.OSFamily == "debian":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(a.renderDebianPreseed(job, profile)))
	case segment == "kickstart" && profile.OSFamily != "debian" && profile.OSFamily != "ubuntu":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(a.renderKickstart(job, profile)))
	default:
		http.NotFound(w, r)
	}
}

func (a *App) handleAgentInstallScript(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/provisioning/jobs/"), "/")
	if len(parts) < 3 || parts[1] != "agent-install" {
		http.NotFound(w, r)
		return
	}
	token := strings.TrimSuffix(parts[2], ".sh")
	job, profile, ok := a.authorizeJob(parts[0], token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript")
	_, _ = w.Write([]byte(a.renderAgentInstallScript(job, profile)))
}

func (a *App) handleProvisioningArtifact(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/provisioning/jobs/"), "/")
	if len(parts) < 4 || parts[1] != "artifacts" || parts[3] != "mxagent" {
		http.NotFound(w, r)
		return
	}
	if _, _, ok := a.authorizeJob(parts[0], parts[2]); !ok {
		http.NotFound(w, r)
		return
	}
	if a.cfg.AgentBinaryPath == "" {
		http.Error(w, "agent binary path not configured", http.StatusServiceUnavailable)
		return
	}
	data, err := os.ReadFile(a.cfg.AgentBinaryPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=mxagent")
	_, _ = w.Write(data)
}

func (a *App) handleInstallEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/provisioning/jobs/"), "/")
	if len(parts) < 2 || parts[1] != "events" {
		http.NotFound(w, r)
		return
	}
	var payload struct {
		Status  string `json:"status"`
		Phase   string `json:"phase"`
		Message string `json:"message"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	job, ok := a.store.GetInstallJob(parts[0])
	if !ok || payload.Token != job.Token {
		http.NotFound(w, r)
		return
	}
	a.updateInstallJobStatus(&job, payload.Status, fallback(payload.Message, "event received"), fallback(payload.Phase, "install"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func installProfileToProto(profile store.InstallProfile) *metalxpb.InstallProfile {
	return &metalxpb.InstallProfile{
		Id:                    profile.ID,
		Name:                  profile.Name,
		OsFamily:              profile.OSFamily,
		OsVersion:             profile.OSVersion,
		Architecture:          profile.Architecture,
		Firmware:              profile.Firmware,
		InstallSource:         profile.InstallSource,
		BootKernelPath:        profile.BootKernelPath,
		BootInitrdPath:        profile.BootInitrdPath,
		HostnamePattern:       profile.HostnamePattern,
		Timezone:              profile.Timezone,
		Locale:                profile.Locale,
		KeyboardLayout:        profile.KeyboardLayout,
		AdminUsername:         profile.AdminUsername,
		AdminPasswordHash:     profile.AdminPasswordHash,
		SshAuthorizedKeys:     profile.SSHAuthorizedKeys,
		Packages:              profile.Packages,
		PackageMirror:         profile.PackageMirror,
		DiskLayout:            profile.DiskLayout,
		NetworkMode:           profile.NetworkMode,
		AgentBinaryUrl:        profile.AgentBinaryURL,
		AgentServiceName:      profile.AgentServiceName,
		ControllerGrpcAddress: profile.ControllerGRPCAddr,
		ExtraKernelArgs:       profile.ExtraKernelArgs,
		PostInstallScript:     profile.PostInstallScript,
		Enabled:               profile.Enabled,
		CreatedAt:             profile.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             profile.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func installProfileFromProto(profile *metalxpb.InstallProfile) store.InstallProfile {
	if profile == nil {
		return store.InstallProfile{}
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, profile.GetCreatedAt())
	updatedAt, _ := time.Parse(time.RFC3339Nano, profile.GetUpdatedAt())
	return store.InstallProfile{
		ID:                 profile.GetId(),
		Name:               profile.GetName(),
		OSFamily:           profile.GetOsFamily(),
		OSVersion:          profile.GetOsVersion(),
		Architecture:       profile.GetArchitecture(),
		Firmware:           profile.GetFirmware(),
		InstallSource:      profile.GetInstallSource(),
		BootKernelPath:     profile.GetBootKernelPath(),
		BootInitrdPath:     profile.GetBootInitrdPath(),
		HostnamePattern:    profile.GetHostnamePattern(),
		Timezone:           profile.GetTimezone(),
		Locale:             profile.GetLocale(),
		KeyboardLayout:     profile.GetKeyboardLayout(),
		AdminUsername:      profile.GetAdminUsername(),
		AdminPasswordHash:  profile.GetAdminPasswordHash(),
		SSHAuthorizedKeys:  profile.GetSshAuthorizedKeys(),
		Packages:           profile.GetPackages(),
		PackageMirror:      profile.GetPackageMirror(),
		DiskLayout:         profile.GetDiskLayout(),
		NetworkMode:        profile.GetNetworkMode(),
		AgentBinaryURL:     profile.GetAgentBinaryUrl(),
		AgentServiceName:   profile.GetAgentServiceName(),
		ControllerGRPCAddr: profile.GetControllerGrpcAddress(),
		ExtraKernelArgs:    profile.GetExtraKernelArgs(),
		PostInstallScript:  profile.GetPostInstallScript(),
		Enabled:            profile.GetEnabled(),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}

func installJobToProto(job store.InstallJob) *metalxpb.InstallJob {
	return &metalxpb.InstallJob{
		Id:             job.ID,
		ProfileId:      job.ProfileID,
		ProfileName:    job.ProfileName,
		OsFamily:       job.OSFamily,
		Status:         job.Status,
		MacAddress:     job.MACAddress,
		Hostname:       job.Hostname,
		NodeId:         job.NodeID,
		Token:          job.Token,
		BootUrl:        job.BootURL,
		ConfigUrl:      job.ConfigURL,
		AgentScriptUrl: job.AgentScriptURL,
		LastEvent:      job.LastEvent,
		CreatedAt:      job.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		BootPreview:    job.BootPreview,
		ConfigPreview:  job.ConfigPreview,
	}
}
