# Spectra

Spectra is a system monitoring solution written in Go. Lightweight agents collect metrics from Linux, Windows, and FreeBSD hosts and transmit them to a central server backed by PostgreSQL with TimescaleDB for time-series storage. A React dashboard (in development) provides fleet-wide visibility and per-host drill-down.

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

## Motivation

Most monitoring solutions treat all hosts the same—abstracting away the hardware until it becomes a "black box." Spectra was built to bridge the gap between high-level application monitoring and low-level hardware diagnostics.

1. **Hardware Awareness:** A Raspberry Pi has different critical metrics (voltage, throttling, SD card wear) than an Intel server. Spectra treats these physical realities as first-class citizens.
2. **Unified Workloads:** Whether a workload is a Docker container, an LXC container, or a QEMU VM, Spectra abstracts them into a single entity for unified dashboards across heterogeneous clusters.
3. **Active Diagnostics:** Passive monitoring tells you *that* something is wrong; Spectra lets you investigate. Remote diagnostic primitives (ping, traceroute, disk usage analysis) are embedded directly in the agent, allowing troubleshooting without SSH/RDP access.

## Quick Start

**Prerequisites:** Go 1.24+, PostgreSQL 16+ with TimescaleDB

1. **Set up the database:**
```bash
createdb spectra
psql spectra -c "CREATE EXTENSION IF NOT EXISTS timescaledb;"
```

2. **Apply migrations:**
```bash
psql spectra -f internal/database/migrations/001_core.up.sql
psql spectra -f internal/database/migrations/002_metrics.up.sql
psql spectra -f internal/database/migrations/003_retention_and_indexes.up.sql
psql spectra -f internal/database/migrations/004_schema_additions.up.sql
psql spectra -f internal/database/migrations/005_current_metrics.up.sql
```

3. **Start the server:**
```bash
go run ./cmd/server -db "postgres://localhost:5432/spectra?sslmode=disable"
```

4. **Generate a registration token:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/tokens
```

5. **Start an agent:**
```bash
sudo go run ./cmd/agent -token <token>
```

The agent registers, receives credentials, and begins collecting metrics aligned to the next minute boundary.

## Configuration

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-db` | (required) | PostgreSQL connection string (or `DATABASE_URL` env) |
| `-port` | `8080` | HTTP listen port |

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

### API Endpoints

#### Dashboard (Read)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/overview` | All agents with current metrics |
| GET | `/api/v1/agents` | List registered agents |
| GET | `/api/v1/agents/{id}` | Agent details |
| DELETE | `/api/v1/agents/{id}` | Remove agent and cascade data |
| GET | `/api/v1/agents/{id}/cpu` | CPU metrics (time range) |
| GET | `/api/v1/agents/{id}/memory` | Memory metrics (time range) |
| GET | `/api/v1/agents/{id}/disk` | Disk metrics (time range) |
| GET | `/api/v1/agents/{id}/diskio` | Disk I/O metrics (time range) |
| GET | `/api/v1/agents/{id}/network` | Network metrics (time range) |
| GET | `/api/v1/agents/{id}/temperature` | Temperature metrics (time range) |
| GET | `/api/v1/agents/{id}/system` | System metrics (time range) |
| GET | `/api/v1/agents/{id}/container` | Container metrics (time range) |
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

#### Admin

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/tokens` | Generate registration token |
| POST | `/api/v1/admin/logs` | Trigger log fetch from agent |
| POST | `/api/v1/admin/disk` | Trigger disk usage scan |
| POST | `/api/v1/admin/network` | Trigger network diagnostic |

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
- **Clock alignment** — collectors start on minute boundaries for consistent charting
- **Metric caching** — buffers up to 1000 envelopes when server is unreachable
- **Retry with drain** — cached metrics sent first on reconnection
- **Gzip compression** — all metric batches compressed in transit
- **Platform detection** — ARM board identification, Windows 11 build detection

## Building

```bash
go build -o spectra-server ./cmd/server
go build -o spectra-agent ./cmd/agent
```

### Cross-Compilation

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

### Running as a Service

```ini
# /etc/systemd/system/spectra-agent.service
[Unit]
Description=Spectra Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/spectra-agent
Environment=SPECTRA_SERVER=http://10.1.0.23:8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now spectra-agent
```

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
│   └── server/             # Server entry point
├── internal/
│   ├── agent/              # Agent runtime, collector orchestration, caching
│   ├── collector/          # Metric collectors (Linux, Windows, FreeBSD)
│   ├── database/
│   │   ├── migrations/     # SQL migrations (001-005)
│   │   └── queries/        # sqlc query definitions
│   ├── diagnostics/        # On-demand diagnostic tools
│   ├── platform/           # OS/hardware detection
│   ├── protocol/           # Shared types and metric definitions
│   └── server/             # HTTP handlers, routing, middleware
└── .github/
    └── workflows/          # CI configuration
```

## Performance

Collector benchmarks on Intel i7-13700K (Windows) and Raspberry Pi 1 (Linux):

| Collector | i7-13700K | Pi 1 | Notes |
|-----------|-----------|------|-------|
| CPU | 30-60µs | 2.5ms | |
| Memory | 420ns | 930µs | |
| Disk | 0.7-1.6ms | 125µs | |
| Disk I/O | 9-11µs | 2.2ms | |
| Network | 1.1ms | 8.4ms | |
| Processes | 58ms | 71ms | Parallel on Windows |
| System | 45-52ms | – | WMI overhead on Windows |

Database storage estimate: ~1-2 GB compressed for 11 agents with 30-day retention.

## TODO

- [ ] React web dashboard (fleet overview, per-agent drill-down, charts)
- [ ] User authentication for dashboard API
- [ ] Alert rules engine with notifications
- [ ] Metric aggregation and rollup for long-term trends
- [ ] Agent auto-update mechanism
- [ ] Formal migration tool (golang-migrate or similar)
- [ ] Log aggregation

## Dependencies

Core:
- `golang.org/x/sys` — Low-level system calls
- `github.com/tklauser/go-sysconf` — System configuration values
- `github.com/docker/docker` — Docker API client
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `golang.org/x/crypto` — bcrypt for agent secrets

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