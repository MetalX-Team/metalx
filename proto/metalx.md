# MetalX gRPC Contract

This repository keeps the protocol definition as a checked-in design artifact until code generation is wired in.

## Services

- `AgentControl`
  - `Register(RegisterRequest) returns (RegisterResponse)`
  - `Heartbeat(HeartbeatRequest) returns (HeartbeatResponse)`
  - `Collect(NodeSnapshotRequest) returns (NodeSnapshotResponse)`
  - `Execute(CommandRequest) returns (CommandResponse)`
  - `OpenTerminal(stream TerminalFrame) returns (stream TerminalFrame)`
- `ControlPlane`
  - `ListNodes(ListNodesRequest) returns (ListNodesResponse)`
  - `GetNode(GetNodeRequest) returns (NodeDetailResponse)`
  - `RunTask(TaskRequest) returns (TaskResponse)`
  - `GetDnsmasqSettings(Empty) returns (DnsmasqSettings)`
  - `UpdateDnsmasqSettings(UpdateDnsmasqSettingsRequest) returns (DnsmasqSettings)`
  - `StreamEvents(stream EventRequest) returns (stream EventEnvelope)`

## Core Types

- `NodeIdentity`
  - `id`, `hostname`, `agent_version`, `os`, `kernel`, `primary_ip`
- `NodeSummary`
  - `identity`, `online`, `last_seen_at`, `cpu_usage`, `memory_usage`, `disk_usage`
- `NodeDetail`
  - `summary`, `processes`, `network_interfaces`, `filesystems`, `logged_users`, `alerts`
- `Task`
  - `id`, `command`, `targets`, `status`, `started_at`, `finished_at`, `results`
- `DnsmasqSettings`
  - `enabled`, `listen_interface`, `dhcp_range_start`, `dhcp_range_end`, `gateway`, `dns_servers`
  - `tftp_root`, `boot_file`, `kernel_path`, `initrd_path`, `boot_args`, `next_server`
  - `rendered_config`, `rendered_pxe_menu`, `updated_at`

The Go services currently expose equivalent JSON models internally. A future step can replace the markdown contract with `metalx.proto` and generated stubs without changing the higher-level data shape.
