# Spectra

Spectra is a system monitoring agent and server written in Go. It collects metrics from Linux and Windows hosts and transmits them to a central server for aggregation and analysis.

## Architecture

The agent runs on each monitored host, collecting metrics at configurable intervals and sending them in compressed batches to the server. The server receives metrics, processes them, and provides a command-and-control interface for on-demand diagnostics.

## Current Status

The agent and server are functional with the following capabilities implemented:

### Metric Collectors (Agent)

| Collector | Linux | Windows | Interval | Description |
|-----------|:-----:|:-------:|----------|-------------|
| CPU | ✓ | ✓ | 5s | Usage percentage, per-core usage, load averages |
| Memory | ✓ | ✓ | 10s | RAM total/used/available, swap usage |
| Disk | ✓ | ✓ | 60s | Per-mount usage, filesystem type, inodes (Linux) |
| Disk I/O | ✓ | ✓ | 5s | Read/write bytes, operations, latency |
| Network | ✓ | ✓ | 5s | Per-interface RX/TX bytes, packets, errors, drops |
| Processes | ✓ | ✓ | 15s | Top processes by CPU/memory, thread states |
| Services | ✓ | ✓ | 60s | systemd services (Linux), Windows services |
| Temperature | ✓ | ✓ | 10s | Hardware sensors via hwmon (Linux), WMI (Windows) |
| WiFi | ✓ | ✓ | 30s | Signal strength, link quality, SSID, bitrate |
| Containers | ✓ | ✓ | 60s | Docker containers and Proxmox guests (LXC/VM) |
| System | ✓ | ✓ | 300s | Uptime, boot time, process count, logged-in users |
| Applications | ✓ | ✓ | Nightly | Installed applications inventory |
| Raspberry Pi | ✓ | N/A | Various | CPU/GPU clocks, voltages, throttle state |

### Container Support

The container collector supports multiple runtimes:

| Source | Type | Requirements |
|--------|------|--------------|
| Docker | Containers | Docker daemon running |
| Proxmox | LXC | pvesh CLI available (runs on Proxmox node) |
| Proxmox | VM | pvesh CLI available (runs on Proxmox node) |

Container metrics include CPU usage, memory usage/limits, and network I/O. Proxmox collection uses parallel API calls for efficient gathering of guest metrics.

### On-Demand Diagnostics (Server-Triggered)

| Command | Linux | Windows | Description |
|---------|:-----:|:-------:|-------------|
| Fetch Logs | ✓ | ✓ | Retrieve system logs filtered by severity |
| Disk Usage | ✓ | ✓ | Scan directories for largest files/folders |
| List Mounts | ✓ | ✓ | Return available mount points |
| Network Ping | ✓ | ✓ | ICMP ping to specified target |
| Network Connect | ✓ | ✓ | TCP connection test to host:port |
| Netstat | ✓ | ✓ | Active network connections |
| Traceroute | ✓ | ✓ | Network path tracing |

### Server Components

- HTTP API for metric ingestion (`/api/v1/metrics`)
- Agent registration endpoint (`/api/v1/agent/register`)
- Command queue with long-polling (`/api/v1/agent/command`)
- Command result receiver (`/api/v1/agent/command_result`)
- Admin endpoints for triggering diagnostics

### Protocol

Metrics are transmitted as JSON with gzip compression. Each metric is wrapped in an envelope containing:

- `type`: Metric type identifier
- `timestamp`: Collection timestamp
- `hostname`: Source host identifier
- `data`: Metric-specific payload

## Building

Requirements: Go 1.24+

```bash
# Build agent
go build -o spectra-agent ./cmd/agent

# Build server
go build -o spectra-server ./cmd/server
```

### Cross-Compilation

```bash
# Linux agent from Windows/Mac
GOOS=linux GOARCH=amd64 go build -o spectra-agent ./cmd/agent

# Windows agent
GOOS=windows GOARCH=amd64 go build -o spectra-agent.exe ./cmd/agent

# ARM64 (Raspberry Pi 4, etc.)
GOOS=linux GOARCH=arm64 go build -o spectra-agent ./cmd/agent

# ARMv6 (Raspberry Pi 1/Zero)
GOOS=linux GOARCH=arm GOARM=6 go build -o spectra-agent ./cmd/agent
```

## Running

### Server

```bash
./spectra-server
# Listens on :8080 by default
```

### Agent

```bash
# Connect to server at default localhost:8080
./spectra-agent

# Connect to remote server
SPECTRA_SERVER=http://10.0.0.5:8080 ./spectra-agent

# Override hostname
HOSTNAME=webserver-01 ./spectra-agent

# Enable pprof debugging
./spectra-agent -debug
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SPECTRA_SERVER` | `http://127.0.0.1:8080` | Server URL |
| `HOSTNAME` | System hostname | Override reported hostname |

## API Reference

### Agent Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/metrics` | Submit metric batch |
| POST | `/api/v1/agent/register` | Register agent with host info |
| GET | `/api/v1/agent/command` | Long-poll for pending commands |
| POST | `/api/v1/agent/command_result` | Submit command execution results |

### Admin Endpoints

| Method | Path | Query Parameters | Description |
|--------|------|------------------|-------------|
| POST | `/admin/trigger_logs` | `hostname` | Request log fetch from agent |
| POST | `/admin/trigger_disk` | `hostname`, `path`, `top_n` | Request disk usage scan |
| POST | `/admin/trigger_network` | `hostname`, `action`, `target` | Request network diagnostic |

Network actions: `ping`, `connect`, `netstat`, `traceroute`

## Testing

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests for specific package
go test ./internal/collector/...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. -benchmem ./internal/collector/...
```

Tests use table-driven patterns with mock data. Platform-specific tests use build tags to run only on their target OS.

## Project Structure

```
spectra/
├── cmd/
│   ├── agent/          # Agent entry point
│   └── server/         # Server entry point
├── internal/
│   ├── agent/          # Agent runtime, collector orchestration
│   ├── collector/      # Metric collection implementations
│   ├── diagnostics/    # On-demand diagnostic tools
│   ├── protocol/       # Shared types and metric definitions
│   ├── sender/         # HTTP transport with compression
│   └── server/         # Server handlers, routing, storage
└── .github/
    └── workflows/      # CI configuration
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
| System | 45-52ms | N/A | WMI overhead on Windows |

All collectors are designed for minimal overhead at their configured intervals.

## Roadmap

### Phase 1: Proxmox Metrics ✓

Proxmox VE guest metrics via pvesh CLI with parallel collection.

### Phase 2: Database Integration

Replace in-memory storage with persistent database:

- [ ] Select database (PostgreSQL, TimescaleDB, or InfluxDB)
- [ ] Design schema for time-series metric storage
- [ ] Implement metric ingestion pipeline
- [ ] Add data retention policies
- [ ] Create indexes for common query patterns
- [ ] Implement historical data queries API

### Phase 3: Agent Registration System

Formalize agent lifecycle management:

- [ ] Agent authentication (API keys or certificates)
- [ ] Agent registration workflow with approval
- [ ] Agent heartbeat tracking
- [ ] Agent status (online/offline/stale)
- [ ] Agent metadata storage (OS, version, capabilities)
- [ ] Agent deregistration/cleanup

### Phase 4: Web Interface

Build administrative dashboard:

- [ ] Agent inventory view
- [ ] Real-time metric visualization
- [ ] Historical metric charts
- [ ] Alert configuration
- [ ] Diagnostic command interface
- [ ] User authentication
- [ ] Multi-tenancy support

### Future Considerations

- Alert rules engine with notification integrations
- Metric aggregation and rollup
- Agent auto-update mechanism
- Configuration management
- Log aggregation
- Distributed tracing integration

## Dependencies

Core:

- `golang.org/x/sys` - Low-level system calls
- `github.com/tklauser/go-sysconf` - System configuration values
- `github.com/docker/docker` - Docker API client

Windows-specific:

- `github.com/yusufpapurcu/wmi` - WMI queries

## License

MIT License - see [LICENSE](LICENSE) for details.