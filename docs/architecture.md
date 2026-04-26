# MetalX Architecture

MetalX is organized as a monorepo with four deployable parts:

- `agent`: runs on managed servers and reports metrics to the controller.
- `controller`: owns node registration, discovery, command routing, and cluster state.
- `webapi`: exposes authentication, REST, and WebSocket APIs for the dashboard.
- `dashboard`: static React + Vite application deployed behind Nginx.

## Runtime Topology

- Agents connect to the controller over gRPC.
- WebAPI calls the controller over gRPC.
- Dashboard talks only to WebAPI using REST and WebSocket.

## Data Ownership

- Controller persists cluster metadata, audit history, task history, and metric snapshots in SQLite.
- Agents cache only their local runtime identity and ephemeral collection state.
- WebAPI persists local auth/session state in SQLite and proxies controller-managed resources.

## Runtime Surface

- Agent registration and heartbeat.
- Cluster summary and per-node details.
- Command execution and task history.
- Controller-managed dnsmasq and PXE boot profile generation.
- Controller-managed provisioning profiles, install jobs, iPXE boot scripts, OS-specific unattended install files, and agent bootstrap script generation.
- Dashboard overview, node details, tasks, terminal, alerts, audit, and settings shells.
