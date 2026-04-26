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
- Dashboard overview, AIChat, node details, tasks, terminal, alerts, audit, and settings shells.

## LLM Agent-Controlled Dashboard

The dashboard `AIChat` page is backed by WebAPI, which proxies an OpenAI-compatible Chat Completions endpoint and executes model tool calls against controller gRPC.

- LLM settings live in WebAPI local SQLite auth/config storage, not in controller state. This keeps API keys out of controller gRPC and out of browser responses.
- The dashboard stores only conversation messages and a boolean `allowTools` flag for each AIChat request.
- WebAPI injects a MetalX system prompt and tool schemas, calls `<LLM Base URL>/chat/completions`, and loops over tool calls for a bounded number of rounds.
- Tool calls cover the controller API surface: summary, nodes, tasks, audits, alerts, system info, runtime settings, dnsmasq/PXE settings, install profiles, install jobs, and node command execution.
- Mutating tool calls use actor `aichat-agent`, preserving normal task/config/audit records.
- Interactive terminal takeover remains a dashboard WebSocket flow; AIChat can explain or request it, while actual interactive terminal IO stays in the terminal page.

The system prompt instructs the model to inspect state before acting, avoid inventing node/template identifiers, explain risky changes, avoid secret disclosure, and reply in Chinese with executed actions, key results, and next steps.

This design gives the agent complete controller capability while keeping execution centralized in WebAPI and controller-owned audit trails.
