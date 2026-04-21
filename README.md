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
