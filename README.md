# MetalX

MetalX is a LAN-oriented server lifecycle management platform composed of Agent, Controller, WebAPI, and Dashboard.

## Repository Layout

- `agent/`: Go-based runtime for managed hosts.
- `controller/`: Go-based management plane and cluster state service.
- `webapi/`: Gin-based API gateway for dashboard traffic.
- `dashboard/`: React + Vite static frontend.
- `proto/`: shared contract and protocol design notes.
- `docs/`: architecture and implementation documentation.

## Quick Start

### 1. Start the controller

```bash
cd controller
go run ./cmd/mxctl
```

### 2. Start the agent

```bash
cd agent
go run ./cmd/mxagent
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

## Current Capabilities

- Agent exposes real host snapshot and command execution endpoints based on live system data.
- Controller ingests live agent reports, tracks node state, tasks, alerts, and audits, and exposes cluster APIs.
- WebAPI provides login and authenticated proxy endpoints to the controller.
- Dashboard renders a live operational command center backed by WebAPI data only.

## Next Steps

- Replace HTTP placeholders with generated gRPC contracts from `proto/`.
- Persist controller and webapi state in SQLite.
- Add WebSocket streaming for terminal and live metrics.
- Expand the dashboard into multi-page navigation and richer long-range history views.
