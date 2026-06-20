# Spectra

Spectra is a system monitoring solution written in Go. Lightweight agents collect metrics from Linux, Windows, and FreeBSD hosts and transmit them to a central server backed by PostgreSQL with TimescaleDB for time-series storage. A React dashboard provides fleet-wide visibility and per-host drill-down.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Agent (Pi)  │     │ Agent (VM)  │     │ Agent (Win) │
│  ARM/Linux   │     │  x64/Linux  │     │ x64/Windows │
└──────┬───────┘     └──────┬──────┘     └──────┬──────┘
       │    gzip/JSON       │                   │
       └────────────────────┼───────────────────┘
                            ▼
                    ┌───────────────┐
                    │ Spectra Server │
                    │   Go + Mux    │
                    └───────┬───────┘
                            │
                    ┌───────▼───────┐
                    │  PostgreSQL   │
                    │ + TimescaleDB │
                    └───────────────┘
```

Agents register with the server using one-time tokens, then collect metrics at configurable intervals aligned to minute boundaries. Metrics are sent in compressed batches with automatic retry and local caching when the server is unreachable. The server persists metrics to hypertables and maintains a `current_metrics` cache table for fast dashboard queries.

The server authenticates dashboard users with session-based login and a three-tier role model (superadmin, admin, viewer), and supports TLS with a self-signed CA generated at setup. Agents authenticate to the server with a SHA-256 credential issued during registration.

## Motivation

Most monitoring solutions treat all hosts the same—abstracting away the hardware until it becomes a "black box." Spectra was built to bridge the gap between high-level application monitoring and low-level hardware diagnostics.

1. **Hardware Awareness:** A Raspberry Pi has different critical metrics (voltage, throttling, SD card wear) than an Intel server. Spectra treats these physical realities as first-class citizens.
2. **Unified Workloads:** Whether a workload is a Docker container, an LXC container, or a QEMU VM, Spectra abstracts them into a single entity for unified dashboards across heterogeneous clusters.
3. **Active Diagnostics:** Passive monitoring tells you *that* something is wrong; Spectra lets you investigate. Remote diagnostic primitives (ping, traceroute, disk usage analysis) are embedded directly in the agent, allowing troubleshooting without SSH/RDP access.

## Quick Start

**Prerequisites:** Go 1.26+. PostgreSQL 16+ with TimescaleDB is installed automatically by the setup tool on supported platforms (Debian/Ubuntu, RHEL, etc.).

Spectra ships a guided installer, `spectra-setup`, that provisions the database, runs migrations, creates the initial superadmin, generates TLS certificates (if requested) and the secret-encryption key, writes the server config, and starts the service.

From a workstation, targeting a fresh host:

```bash
make setup DEPLOY_HOST=10.10.107.1
```

On the box itself:

```bash
sudo make setup
```

`sudo` is scoped to the privileged install steps; the build runs as your user. Both forms build the server and setup binaries, install them along with the systemd unit, then run `spectra-setup` (interactively over SSH for the remote form) to configure the database, admin account, key, and TLS, and bring the service up.

Once the server is running, provision agents from the dashboard, which issues a one-time registration token and the platform-appropriate install command. The agent registers, receives credentials, and begins collecting metrics aligned to the next minute boundary.

### Unattended setup

`spectra-setup` accepts a YAML file for non-interactive installs:

```bash
spectra-setup -from setup.yaml
```

```yaml
database:
  host: localhost
  port: "5432"
  name: spectra
  user: spectra
  password: <db-password>
  ssl: disable
  create: true
admin:
  username: admin
  password: <admin-password>
server:
  port: 8080
  migrations: internal/database/migrations
  external_url: https://10.10.107.1:8080
tls:
  enabled: true
  sans:
    - 10.10.107.1
skip_prerequisites: false
```

> YAML key names follow the `SetupFile` struct tags; adjust to match if you have customized them.

### Updating an existing install

```bash
make deploy DEPLOY_HOST=10.10.107.1
```

Both `setup` and `deploy` run locally when `DEPLOY_HOST` is unset and over SSH when it is set.

## Configuration

### Server

The server reads its configuration from a JSON file written by `spectra-setup` (default `/etc/spectra/server.json`), passed via `-config`:

```bash
spectra-server -config /etc/spectra/server.json
```

The config holds the database URL, listen port, external URL, and TLS certificate paths. The systemd unit invokes the server this way; you do not normally run it by hand.

### Secret encryption key

Spectra encrypts recoverable secrets at rest (currently the SMTP password) using AES-256-GCM. The key is supplied via the `SPECTRA_SECRET_KEY` environment variable as a base64-encoded 32-byte value.

`spectra-setup` generates this automatically and writes it to `/etc/spectra/spectra.env`, which the systemd unit loads:

```ini
EnvironmentFile=-/etc/spectra/spectra.env
```

The leading `-` makes the file optional: the server still starts without it, with email delivery disabled until a key is present. To generate one manually:

```bash
openssl rand -base64 32
```

The key is generated once and never rotated automatically — rotating it would render existing encrypted values unrecoverable. Setup will not overwrite an existing key file.

### Agent Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SPECTRA_SERVER` | `http://127.0.0.1:8080` | Server URL |
| `HOSTNAME` | Auto-detected | Override hostname |

## Current Status

### Metric Collectors

| Collector | Linux | Windows | FreeBSD | Interval | Description |
|-----------|-------|---------|---------|----------|-------------|
| CPU | ✓ | ✓ | ✓ | 5s | Usage, per-core, load averages, iowait |
| Memory | ✓ | ✓ | ✓ | 10s | RAM total/used/available, swap |
| Disk | ✓ | ✓ | ✓ | 60s | Per-mount usage, filesystem type, inodes |
| Disk I/O | ✓ | ✓ | ✓ | 5s | Read/write bytes, ops, latency |
| Network | ✓ | ✓ | ✓ | 5s | Per-interface RX/TX bytes, packets, errors |
| Processes | ✓ | ✓ | ✓ | 15s | Top processes by CPU/memory |
| Services | ✓ | ✓ | – | 60s | systemd (Linux), Windows services |
| Temperature | ✓ | ✓ | ✓ | 10s | Hardware sensors via hwmon/WMI/sysctl |
| WiFi | ✓ | ✓ | – | 30s | Signal strength, SSID, bitrate |
| Containers | ✓ | ✓ | – | 60s | Docker + Proxmox guests (LXC/VM) |
| System | ✓ | ✓ | ✓ | 300s | Uptime, boot time, process count |
| Applications | ✓ | ✓ | – | Nightly | Installed application inventory |
| Updates | ✓ | ✓ | – | Nightly | Pending updates, security patches, reboot status |
| Raspberry Pi | ✓ | – | – | Various | CPU/GPU clocks, voltages, throttle state |

### Container Support

| Source | Type | Requirements |
|--------|------|--------------|
| Docker | Containers | Docker daemon (10s timeout, health tracking) |
| Proxmox | LXC/VM | `pvesh` CLI on Proxmox node |

### Database

Metrics are stored in TimescaleDB hypertables with automatic 30-day retention via compression and drop policies. The schema includes:

- **agents** — registered hosts with hardware metadata
- **metrics_*** — per-metric-type hypertables (cpu, memory, disk, network, etc.)
- **current_metrics** — single-row-per-agent cache for dashboard overview queries
- **current_processes/services/applications/updates** — latest state tables
- **users / sessions** — dashboard authentication and role assignment
- **alert_rules / alert_channels / alert_rule_channels / alert_events** — alerting
- **smtp_config** — server-wide email transport (single row)

### Authentication & Roles

Dashboard access is session-based. Three roles gate functionality:

- **superadmin** — full control, including user role changes; the first user created at setup
- **admin** — operational writes (agent config, provisioning, labels, SMTP setup)
- **viewer** — read-only

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Log in, set session cookie |
| POST | `/api/v1/auth/logout` | Log out |
| GET | `/api/v1/auth/me` | Current user and role |
| GET | `/api/v1/admin/users` | List users |
| POST | `/api/v1/admin/users` | Create user (admin+) |
| DELETE | `/api/v1/admin/users/{id}` | Delete user (admin+) |
| PUT | `/api/v1/admin/users/{id}/role` | Change role (superadmin) |

### API Endpoints

#### Dashboard (Read)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/overview` | All agents with current metrics |
| GET | `/api/v1/agents` | List registered agents |
| GET | `/api/v1/agents/{id}` | Agent details |
| DELETE | `/api/v1/agents/{id}` | Remove agent and cascade data (admin+) |
| GET | `/api/v1/agents/{id}/cpu` | CPU metrics (time range) |
| GET | `/api/v1/agents/{id}/memory` | Memory metrics (time range) |
| GET | `/api/v1/agents/{id}/disk` | Disk metrics (time range) |
| GET | `/api/v1/agents/{id}/diskio` | Disk I/O metrics (time range) |
| GET | `/api/v1/agents/{id}/network` | Network metrics (time range) |
| GET | `/api/v1/agents/{id}/temperature` | Temperature metrics (time range) |
| GET | `/api/v1/agents/{id}/system` | System metrics (time range) |
| GET | `/api/v1/agents/{id}/containers` | Container metrics (time range) |
| GET | `/api/v1/agents/{id}/wifi` | WiFi metrics (time range) |
| GET | `/api/v1/agents/{id}/pi` | Raspberry Pi metrics (time range) |
| GET | `/api/v1/agents/{id}/processes` | Top processes (`?sort=cpu\|memory&limit=20`) |
| GET | `/api/v1/agents/{id}/services` | Current services |
| GET | `/api/v1/agents/{id}/applications` | Installed applications |
| GET | `/api/v1/agents/{id}/updates` | Pending updates |

**Time range parameters:** All metric endpoints support `?range=5m|15m|1h|6h|24h|7d|30d` for quick ranges or `?start=<RFC3339>&end=<RFC3339>` for calendar ranges. Default is `1h`. Start is clamped to 30-day retention.

#### Agent

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agent/register` | Register with one-time token |
| POST | `/api/v1/agent/metrics` | Submit metric batch (gzip) |
| GET | `/api/v1/agent/command` | Long-poll for commands |
| POST | `/api/v1/agent/command/result` | Submit command results |
| GET | `/api/v1/agent/config` | Fetch current agent config |

#### Admin

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/tokens` | Generate registration token (admin+) |
| POST | `/api/v1/admin/provision` | Provision a new agent (admin+) |
| POST | `/api/v1/admin/logs` | Trigger log fetch from agent (admin+) |
| POST | `/api/v1/admin/disk` | Trigger disk usage scan (admin+) |
| POST | `/api/v1/admin/network` | Trigger network diagnostic (admin+) |
| POST | `/api/v1/admin/update` | Push agent self-update (admin+) |

### Alerting

Spectra evaluates alert rules fleet-wide on a background loop (every 60s) and delivers notifications through configurable channels. Rules and channels are global objects manageable by any authenticated user; SMTP transport setup is admin-only.

**Condition types:**

| Condition | Description | Parameters |
|-----------|-------------|------------|
| `agent_offline` | Agent has not reported within a timeout | `timeout_seconds` (default 300) |
| `disk_prediction` | Disk projected to fill within a window, via linear regression over the last 6h | `mount`, `warn_hours` (default 72) |
| `service_down` | A named service is not healthy | `service_name` |

**Rule scope:** Rules are either `global` (every agent) or `agent` (one agent). Agent-scoped rules take precedence: for a given agent and condition type, an agent-scoped rule suppresses any global rule of the same condition type, allowing per-agent overrides of fleet defaults. `service_down` rules must be agent-scoped — a global service rule would fire on every agent not running that service.

**Channels:**

| Type | Config | Notes |
|------|--------|-------|
| `webhook` | `{"url": "..."}` | POSTs a JSON alert payload |
| `email` | `{"to": "..."}` | Uses the server-wide SMTP transport |

An alert fires once per incident and auto-resolves when the condition clears; it does not re-notify while firing. A per-rule `cooldown_seconds` suppresses re-firing after resolution.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/alerts/channels` | List channels |
| POST | `/api/v1/alerts/channels` | Create channel |
| PUT | `/api/v1/alerts/channels/{id}` | Update channel |
| DELETE | `/api/v1/alerts/channels/{id}` | Delete channel |
| GET | `/api/v1/alerts/rules` | List rules |
| POST | `/api/v1/alerts/rules` | Create rule |
| GET | `/api/v1/alerts/rules/{id}` | Rule with its channels |
| PUT | `/api/v1/alerts/rules/{id}` | Update rule |
| PUT | `/api/v1/alerts/rules/{id}/enabled` | Enable/disable a rule |
| DELETE | `/api/v1/alerts/rules/{id}` | Delete rule |
| GET | `/api/v1/alerts/active` | Currently firing alerts |
| GET | `/api/v1/alerts/history` | Paginated event history (`?limit=&offset=`) |
| GET | `/api/v1/agents/{id}/alerts/history` | Per-agent event history |

**Email / SMTP:** SMTP transport is a single server-wide configuration, managed by admins. Regular users only set the recipient on email channels; they never handle transport credentials.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/smtp` | Current config (password redacted) |
| PUT | `/api/v1/admin/smtp` | Update config |
| POST | `/api/v1/admin/smtp/test` | Send a test email without saving |

Supported TLS modes: `starttls` (e.g. SES port 587), `implicit` (port 465), and `none` (unauthenticated LAN relay). The password is encrypted at rest with the `SPECTRA_SECRET_KEY`; setting a password requires the key to be configured. The test endpoint validates a configuration — including a live send — before it is saved.

> **FreeBSD note:** the FreeBSD service collector reports enabled-state rather than live process state, so `service_down` on FreeBSD detects a service being disabled, not one that has crashed.

### On-Demand Diagnostics

| Command | Linux | Windows | Description |
|---------|-------|---------|-------------|
| Fetch Logs | ✓ | ✓ | System logs filtered by severity |
| Disk Usage | ✓ | ✓ | Top largest files/directories |
| List Mounts | ✓ | ✓ | Available mount points |
| Ping | ✓ | ✓ | ICMP ping |
| Connect | ✓ | ✓ | TCP connection test |
| Netstat | ✓ | ✓ | Active connections |
| Traceroute | ✓ | ✓ | Network path tracing |

### Agent Features

- **Token-based registration** — one-time tokens with configurable expiry
- **Persistent identity** — credentials stored in `/etc/spectra/agent-id.json`
- **TLS** — server-issued CA trust, optional `tls_skip_verify` for self-signed setups
- **Clock alignment** — collectors start on minute boundaries for consistent charting
- **Metric caching** — buffers envelopes when the server is unreachable
- **Retry with drain** — cached metrics sent first on reconnection, with exponential backoff and jitter
- **Gzip compression** — all metric batches compressed in transit
- **Self-update** — server-pushed binary update with SHA-256 verification and atomic replacement
- **Platform detection** — ARM board identification, Windows 11 build detection

## Building

```bash
make build-server    # frontend assets + server binary
make build-setup     # setup binary
make release         # cross-compiled agent binaries + checksums
```

Or directly:

```bash
go build -o spectra-server ./cmd/server
go build -o spectra-agent ./cmd/agent
go build -o spectra-setup ./cmd/setup
```

### Cross-Compilation

`make release` builds all agent targets. To build individually:

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o spectra-agent ./cmd/agent

# Windows
GOOS=windows GOARCH=amd64 go build -o spectra-agent.exe ./cmd/agent

# Raspberry Pi 4 (ARM64)
GOOS=linux GOARCH=arm64 go build -o spectra-agent ./cmd/agent

# Raspberry Pi 1/Zero (ARMv6)
GOOS=linux GOARCH=arm GOARM=6 go build -o spectra-agent ./cmd/agent

# FreeBSD
GOOS=freebsd GOARCH=amd64 go build -o spectra-agent ./cmd/agent
```

### Running the Agent as a Service

```ini
# /etc/systemd/system/spectra-agent.service
[Unit]
Description=Spectra Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/spectra-agent
Environment=SPECTRA_SERVER=https://10.10.107.1:8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now spectra-agent
```

The server's own unit is installed by `make setup` / `spectra-setup`; see [Quick Start](#quick-start).

## Testing

```bash
go test ./...                                        # All tests
go test -race ./...                                  # With race detector
go test -v ./...                                     # Verbose
go test -bench=. -benchmem ./internal/collector/...  # Benchmarks
```

Tests use table-driven patterns with mock interfaces. Platform-specific tests use build tags. The server package uses a `MockDB` implementing the `DB` interface for handler testing without a database.

## Project Structure

```
spectra/
├── cmd/
│   ├── agent/              # Agent entry point
│   ├── server/             # Server entry point
│   └── setup/              # Interactive / unattended installer
├── internal/
│   ├── agent/              # Agent runtime, collector orchestration, caching
│   ├── collector/          # Metric collectors (Linux, Windows, FreeBSD)
│   ├── database/
│   │   ├── migrations/     # SQL migrations
│   │   └── queries/        # sqlc query definitions
│   ├── diagnostics/        # On-demand diagnostic tools
│   ├── platform/           # OS/hardware detection
│   ├── protocol/           # Shared types and metric definitions
│   ├── secret/             # AES-256-GCM encryption for stored secrets
│   ├── server/             # HTTP handlers, routing, middleware, alert evaluator
│   └── setup/              # Setup runner, prerequisites, TLS, migrations
├── deploy/
│   └── spectra-server.service   # Canonical systemd unit
└── .github/
    └── workflows/          # CI configuration
```

## Performance

Median time for one full collection cycle (the collector's `Collect` entry point)
or operation, via `go test -bench`, `-count=10`. Lower is better. Each platform's
collectors use native methods — Linux/ARM read `/proc`, sysfs, and syscalls; the
Windows agent uses WMI and Win32 APIs — so cross-platform differences in the
collection rows reflect those methods, not raw CPU alone. Pure-Go paths
(marshaling, batching, header construction) are directly comparable. A dash means
the benchmark does not apply on that platform: a collector that doesn't run there
(Pi hardware sensors off-Pi), or one whose collection entry point differs in shape
(the Linux temperature collector is constructed per host from its thermal-zone
paths rather than exposing a single `Collect` call, so it has no directly
comparable cycle benchmark).

Collection benchmarks measure the full cycle including the underlying data source,
so they include real I/O (e.g. walking `/proc` for every process) and carry more
run-to-run variance than micro-benchmarks — which is the intent, as it reflects
production cost.

| Operation | Desktop (Win) | i7-10700T | i5-6500T | Pi 4 | Pi Zero 2 W | Pi Zero W |
|---|---|---|---|---|---|---|
| CPU collect | 133.7 µs | 50.8 µs | 31.7 µs | 112.1 µs | 292.5 µs | 1.4 ms |
| Memory collect | 900 ns | 11.7 µs | 14.8 µs | 52.4 µs | 128.5 µs | 541.9 µs |
| Disk collect | 1.8 ms | 12.0 µs | 13.3 µs | 36.8 µs | 16.9 µs | 684.5 µs |
| Disk I/O collect | 9.0 µs | 34.8 µs | 43.5 µs | 165.2 µs | 421.1 µs | 1.6 ms |
| Network collect | 10.0 ms | 170.6 µs | 211.8 µs | 438.9 µs | 1.5 ms | 4.3 ms |
| Processes collect | 61.4 ms | 2.6 ms | 2.1 ms | 8.1 ms | 15.1 ms | 51.7 ms |
| Services collect | 92.2 µs | 7.3 ms | 7.0 ms | 21.2 ms | 50.2 ms | 161.7 ms |
| Temperature collect | 32.7 ms | — | — | — | — | — |
| WiFi collect | 3.9 ms | 9.4 µs | 12.5 µs | 41.1 µs | 79.4 µs | 26.3 ms |
| System collect | 39.6 ms | 1.5 ms | 868.4 µs | 4.6 ms | 7.1 ms | 32.7 ms |
| Docker collect | 7.2 ms | 89.0 µs | 99.4 µs | 334.0 µs | 868.1 µs | 3.6 ms |
| Pi throttle decode | — | — | — | 2.2 ms | 4.5 ms | 12.9 ms |
| Agent construct | 1.6 ms | 59.1 µs | 65.6 µs | 259.2 µs | 553.1 µs | 3.0 ms |
| Set request headers | 268 ns | 478 ns | 647 ns | 3.2 µs | 9.4 µs | 26.7 µs |
| Send batch (small) | 129.9 µs | 125.0 µs | 142.8 µs | 419.4 µs | 1.3 ms | 4.1 ms |
| Marshal CPU envelope | 2.5 µs | 2.5 µs | 3.2 µs | 14.3 µs | 40.2 µs | 134.1 µs |
| Handle /overview | — | 5.3 µs | 6.2 µs | 37.7 µs | 84.3 µs | 358.0 µs |
| Handle metrics POST | — | 5.4 µs | 5.9 µs | 33.9 µs | 77.4 µs | 378.5 µs |

Notable cross-platform differences are real and reflect the collection method:
Windows service enumeration (Win32 API) is far faster than parsing `systemctl`
output, while WMI-backed network, process, and thermal queries are
correspondingly heavier than the Linux `/proc` equivalents. Server-package
benchmarks (`/overview`, metrics ingest) are shown for Linux only, since the
server deploys on Linux. Database storage runs roughly 1–2 GB compressed for 11
agents at 30-day retention.

## TODO

- [ ] Metric aggregation and rollup for long-term trends
- [ ] Network interface rate calculations
- [ ] Proxmox host metrics via `pvesh`
- [ ] Formal migration tool (golang-migrate or similar)
- [ ] Log aggregation
- [ ] OpenAPI specification

## Dependencies

Core:
- `golang.org/x/sys` — Low-level system calls
- `github.com/tklauser/go-sysconf` — System configuration values
- `github.com/docker/docker` — Docker API client
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `golang.org/x/crypto` — bcrypt for user passwords
- `github.com/wneessen/go-mail` — SMTP delivery for alert email

Windows-specific:
- `github.com/yusufpapurcu/wmi` — WMI queries

Code generation:
- `github.com/sqlc-dev/sqlc` — Type-safe SQL

## Contributing

1. **Fork and branch** — create a feature branch for your changes
2. **Style** — run `gofmt` and `go vet`
3. **Testing** — include unit tests; run `go test ./...` before submitting
4. **Benchmarks** — for high-frequency collectors, include benchmark comparisons

## License

MIT License — see [LICENSE](LICENSE) for details.
