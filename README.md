# MetalX

MetalX is a LAN-oriented server lifecycle management platform composed of Agent, Controller, WebAPI, and Dashboard.

## Repository Layout

- `agent/`: Go-based runtime for managed hosts.
- `controller/`: Go-based management plane and cluster state service.
- `webapi/`: Gin-based API gateway for dashboard traffic.
- `dashboard/`: React + Vite static frontend.
- `proto/`: shared contract and protocol design notes.
- `docs/`: architecture and implementation documentation.

## Compilation & Installation

Since the repository does not provide pre-compiled binaries, you must build the components from source.

### Prerequisites

- [Go](https://go.dev/dl/) 1.22+ 
- [Node.js](https://nodejs.org/) 18+ and `npm`

### 1. Build Backend Components (Go)

You can build all backend binaries from the project root using the Go workspace. The following commands will generate binaries in the `bin/` directory:

Make sure you have executed `go mod tidy` for 3 following parts.

```bash
# Build the Controller (Management Plane)
go build -o bin/mxctl ./controller/cmd/mxctl

# Build the WebAPI (API Gateway)
go build -o bin/mxapi ./webapi/cmd/mxapi

# Build the Agent (Managed Host Runtime)
go build -o bin/mxagent ./agent/cmd/mxagent
```

### 2. Build Frontend Component (React)

Compile the dashboard for production:

```bash
cd dashboard
npm install
npm run build
```

The production-ready assets will be located in `dashboard/dist/`. These files can be served by any static web server (e.g., Nginx, Apache).

## Quick Start

### 1. Start the controller

```bash
./bin/mxctl
```

The controller now manages generated `dnsmasq` and PXE files. By default it writes them under `runtime/dnsmasq/`.
You can override the output directory with `MX_DNSMASQ_STATE_DIR` or `./bin/mxctl --dnsmasq-dir /srv/metalx/dnsmasq`.
For unattended provisioning URLs rendered into PXE and installer configs, set:

- `MX_PROVISIONING_BASE_URL`, for example `http://192.168.56.10:8081`
- `MX_PUBLIC_CONTROLLER_GRPC_ADDR`, for example `192.168.56.10:19081`
- `MX_AGENT_BINARY_PATH`, pointing to the `mxagent` binary the controller should serve during install

### 2. Start the agent

```bash
./bin/mxagent
```

### 3. Start the API

```bash
cd webapi
go mod tidy
go run ./cmd/mxapi
```

The bootstrap admin credentials are printed when the API starts. Defaults:

- Username: `admin`
- Password: `metalx-admin-2026`

### 4. Start the dashboard

```bash
cd dashboard
npm install
npm run dev
```

## Local Demo

To run the full stack locally on one machine and preview the provisioning UI:

```bash
# from repo root
mkdir -p bin runtime
go build -o bin/mxctl ./controller/cmd/mxctl
go build -o bin/mxapi ./webapi/cmd/mxapi
go build -o bin/mxagent ./agent/cmd/mxagent

MX_PROVISIONING_BASE_URL=http://127.0.0.1:8081 \
MX_PUBLIC_CONTROLLER_GRPC_ADDR=127.0.0.1:19081 \
MX_AGENT_BINARY_PATH=bin/mxagent \
./bin/mxctl

MX_CONTROLLER_ADDR=127.0.0.1:19081 ./bin/mxapi

MX_CONTROLLER_ADDR=127.0.0.1:19081 \
MX_AGENT_NAME=local-demo-agent \
./bin/mxagent

cd dashboard
npm install
VITE_API_BASE=http://127.0.0.1:8090 npm run dev -- --host 127.0.0.1 --port 5173
```

Local URLs:

- Controller health: `http://127.0.0.1:8081/healthz`
- Controller PXE entry: `http://127.0.0.1:8081/boot/<mac>`
- WebAPI health: `http://127.0.0.1:8090/healthz`
- Dashboard: `http://127.0.0.1:5173`

The dashboard will auto-login using the default credentials unless you override:

- `VITE_METALX_USER`
- `VITE_METALX_PASSWORD`

## Current Capabilities

- Agent exposes real host snapshot and command execution endpoints based on live system data.
- Controller ingests live agent reports, tracks node state, tasks, alerts, and audits, and exposes cluster APIs.
- Controller persists editable `dnsmasq` settings, renders `dnsmasq.conf` and `pxelinux.cfg/default`, and stores an audit record for PXE changes.
- Controller persists install profiles and install jobs, renders iPXE scripts, Ubuntu autoinstall, Debian preseed, and Kickstart files, and serves a post-install agent bootstrap script.
- WebAPI provides login and authenticated proxy endpoints to the controller.
- Dashboard renders a live operational command center backed by WebAPI data only.
- Dashboard includes `AIChat`, a controlled agent workspace that converts natural-language operator goals into explicit plans backed by existing MetalX capabilities.
- Dashboard system settings allow editing `dnsmasq` parameters for PXE boot, including DHCP range, TFTP root, boot file, kernel/initrd, and boot arguments.
- Dashboard now includes `装机模板` and `装机任务` for provisioning Ubuntu, Debian, Fedora, CentOS Stream, and RHEL hosts.
- Dashboard login is no longer hardcoded. Runtime defaults such as refresh interval, terminal shell, provisioning URLs, agent listen addresses, agent report interval, and admin credentials can be updated from `系统设置`.
- System settings include OpenAI-compatible LLM configuration for `AIChat`: API Base URL, API Key, model, and temperature.

## AIChat Agent Workflow

`AIChat` is a real LLM-backed agent control layer. The LLM API must implement the OpenAI Chat Completions format at:

```text
<LLM Base URL>/chat/completions
Authorization: Bearer <API Key>
```

The endpoint can be configured in two ways:

- Environment variables for `mxapi`: `MX_LLM_BASE_URL`, `MX_LLM_API_KEY`, `MX_LLM_MODEL`
- Dashboard `系统设置`: `大模型 API Base URL`, `大模型 API Key`, `大模型 Model`, `Temperature`

The API Key is stored in the WebAPI local SQLite database and is never sent to the browser after saving.

1. Open the dashboard and go to `AIChat`.
2. Describe the goal, for example `检查集群健康状态`, `查看磁盘`, `执行 uptime`, `创建装机任务`, or `配置 PXE`.
3. The dashboard calls `POST /api/aichat`; WebAPI forwards the conversation to the configured LLM and exposes controller tools to the model.
4. The agent maps user intent to the MetalX capability surface:
   - `mxctl`: controller state, PXE/dnsmasq, install profiles, install jobs, and provisioning artifacts
   - `mxagent`: node command execution and streaming terminal sessions
   - `mxapi`: authenticated dashboard REST/WebSocket gateway
5. When `允许调用 controller 工具` is enabled, WebAPI executes model tool calls against controller gRPC. Write operations are recorded through normal controller task/config/audit flows with actor `aichat-agent`.

Available LLM tools cover the controller surface:

- `get_summary`, `list_nodes`, `get_node`
- `list_tasks`, `list_audits`, `list_alerts`, `get_system`
- `get_runtime_settings`, `update_runtime_settings`
- `get_dnsmasq_settings`, `update_dnsmasq_settings`
- `list_install_profiles`, `upsert_install_profile`
- `list_install_jobs`, `create_install_job`, `get_install_job`
- `run_task`

Because the configured LLM receives conversation context and tool results, only point `MX_LLM_BASE_URL` at a model endpoint you trust with cluster operational metadata.

## PXE / dnsmasq Workflow

1. Open the dashboard and go to `系统设置`.
2. Fill in the `dnsmasq / PXE 引导配置` form.
3. Save the configuration. The controller will validate the values, persist them in SQLite, and render:
   - `runtime/dnsmasq/dnsmasq.conf`
   - `runtime/dnsmasq/tftp-root/pxelinux.cfg/default` by default
4. Place your PXE boot assets under the configured TFTP root so the generated menu paths resolve correctly.

The dashboard also shows the rendered `dnsmasq.conf` and PXE menu preview so you can verify the output before wiring the files into a host-level `dnsmasq` service.

## Provisioning Workflow

1. Configure `dnsmasq` so DHCP points PXE clients at your iPXE entrypoint.
2. In the dashboard, open `装机模板` and edit a profile for the target OS family.
3. Create a job in `装机任务` by providing the profile and target MAC address.
4. The client requests `GET /boot/{mac}` from controller and receives an iPXE script.
5. The installer then downloads one of:
   - Ubuntu: `/provisioning/jobs/{jobID}/seed/{token}/user-data`
   - Debian: `/provisioning/jobs/{jobID}/preseed/{token}.cfg`
   - Fedora / CentOS / RHEL: `/provisioning/jobs/{jobID}/kickstart/{token}.ks`
6. During unattended install, the rendered post-install hook downloads `/provisioning/jobs/{jobID}/agent-install/{token}.sh`, installs `mxagent`, writes a systemd unit, and starts it.
7. Installer and agent lifecycle progress is reported back through `/provisioning/jobs/{jobID}/events`.

The default install profiles are seeded automatically for:

- Ubuntu 24.04 LTS
- Debian 12
- Fedora Server 41
- CentOS Stream 9
- RHEL 9

These profiles are examples. You should edit the install source, kernel/initrd paths, password hash, SSH keys, and optional package list to match your environment.

## Next Steps

- Replace HTTP placeholders with generated gRPC contracts from `proto/`.
- Persist controller and webapi state in SQLite.
- Add WebSocket streaming for terminal and live metrics.
- Expand the dashboard into multi-page navigation and richer long-range history views.
