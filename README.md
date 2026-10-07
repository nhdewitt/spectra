# Spectra

Spectra is a self-hosted infrastructure monitoring platform written in Go. Lightweight agents collect metrics from Linux, Windows, macOS, and FreeBSD hosts, including Raspberry Pis, Docker containers, and Proxmox guests. They send the metrics to a central server backed by PostgreSQL and TimescaleDB. A React dashboard embedded in the server binary provides a fleet-wide overview, per-host drill-down, alerting, remote diagnostics, and agent management.

- [Features](#features)
- [Architecture](#architecture)
- [Supported platforms](#supported-platforms)
- [Installing the server](#installing-the-server)
- [Server configuration](#server-configuration)
- [Deploying agents](#deploying-agents)
- [Updating](#updating)
- [What the agent collects](#what-the-agent-collects)
- [Alerting](#alerting)
- [On-demand diagnostics](#on-demand-diagnostics)
- [Users and access](#users-and-access)
- [Data storage](#data-storage)
- [API reference](#api-reference)
- [Development](#development)
- [Performance](#performance)
- [Known limitations](#known-limitations)

## Features

**Dashboard**
- Fleet overview with server-side paging, sorting by any column, status, OS, architecture, hardware, and label filters, hostname search, and table or card views
- Fleet-wide charts, a heatmap, per-host sparklines, and status counts
- Per-host drill-down: metric charts with time ranges from 5 minutes to 30 days, per-device and per-interface detail, anomaly findings, processes, services, containers, installed applications, and pending updates
- Starred agents for quick access, saved per user
- Configurable warning and critical thresholds for CPU, memory, disk, and temperature, plus stale and offline timers

**Agents**
- Single static binary per platform, with collectors aligned to minute boundaries
- Gzip-compressed JSON batches. Undelivered metrics are cached in memory (sized from host RAM) and written to disk on shutdown, then delivered when the server comes back.
- One-time token registration, provisioned from the dashboard with generated install steps for systemd, launchd, rc.d, and Windows services
- Self-update pushed from the dashboard, with SHA-256 verification against the server's release manifest
- Remote configuration from the dashboard: log level, and filesystem types and network interfaces to stop reporting

**Operations**
- Alert rules for offline agents, predicted disk exhaustion, and stopped services, delivered by webhook or email
- Remote diagnostics: log fetch, disk usage, ping, TCP connect, traceroute, and netstat
- Labels: automatic (`os`, `arch`, `hardware`, `agent_version`) and user-defined, usable as filters
- Three-tier roles (superadmin, admin, viewer), session login, account lockout, and optional TLS with a CA generated at setup
- Optional OpenTelemetry tracing of server requests and database queries

## Architecture

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Agent        │  │ Agent        │  │ Agent        │  │ Agent        │
│ Linux / Pi   │  │ Windows      │  │ macOS        │  │ FreeBSD      │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │   HTTP(S): gzip JSON batches, command long-poll     │
       └─────────────────┴────────┬────────┴─────────────────┘
                                  ▼
                     ┌─────────────────────────┐
                     │ spectra-server          │
                     │ API + embedded frontend │──── OTLP/HTTP (optional)
                     │ alert evaluator         │
                     └────────────┬────────────┘
                                  ▼
                     ┌─────────────────────────┐
                     │ PostgreSQL 16           │
                     │ + TimescaleDB           │
                     └─────────────────────────┘
```

Each agent sends its collected metrics every 5 seconds, up to 500 per request. The server writes each batch in one transaction: time-series metrics go into TimescaleDB hypertables, and current-state data (processes, services, applications, updates) replaces the previous rows. A `202` response means the whole batch is stored. On any failure the agent keeps the batch and retries it. After the commit, the server refreshes a `current_metrics` cache that backs the overview.

Agents also long-poll the server for commands (diagnostics and self-updates) and fetch their remote configuration at startup and every 60 seconds after. The alert evaluator runs on the server every 60 seconds.

## Supported platforms

**Agent.** `make release` builds these targets:

| Target | Binary | Notes |
|---|---|---|
| Linux amd64 | `spectra-agent-linux-amd64` | Static (`CGO_ENABLED=0`) |
| Linux arm64 | `spectra-agent-linux-arm64` | Raspberry Pi 3/4/5 on a 64-bit OS |
| Linux ARMv7 | `spectra-agent-linux-armv7` | 32-bit Raspberry Pi OS |
| Linux ARMv6 | `spectra-agent-linux-armv6` | Raspberry Pi 1 and Zero W |
| Windows amd64 | `spectra-agent-windows-amd64.exe` | Runs as a Windows service |
| macOS amd64, arm64 | `spectra-agent-darwin-amd64`, `-arm64` | Built with cgo when available, otherwise a static fallback |
| FreeBSD amd64 | `spectra-agent-freebsd-amd64` | |

**Server.** Linux with systemd. `spectra-setup` installs PostgreSQL 16 and TimescaleDB on Debian, Ubuntu, and the RHEL family (RHEL, CentOS, Fedora, Rocky Linux, AlmaLinux, Amazon Linux).

## Installing the server

### Requirements

To build: Go 1.27.1 or later, and Node.js 24 with npm (the version in `web/.nvmrc`). The frontend is built and embedded into the server binary. `scripts/bootstrap.sh` installs both, at the versions the repo pins, on Debian, Ubuntu, the RHEL family, and FreeBSD. Run it as root.

For a local database, `spectra-setup` installs PostgreSQL and TimescaleDB on the target host if they're missing. When interactive setup is pointed at a remote database, it skips the install, and that server must already run PostgreSQL 16 with TimescaleDB available. An [unattended setup](#unattended-setup) against a remote database should set `database.create: false` and `skip_prerequisites: true`.

### Guided setup

From a workstation, targeting a fresh host over SSH:

```bash
make setup DEPLOY_HOST=<server-ip>          # DEPLOY_USER=root and DEPLOY_PATH=/opt/spectra by default
```

Or on the host itself:

```bash
make setup
```

The local form runs only the privileged install steps with `sudo`. Both forms build `spectra-server` and `spectra-setup`, install them and the systemd unit (`deploy/spectra-server.service`), then run `spectra-setup`. The remote form runs it interactively over SSH. `spectra-setup`:

1. For a local database, installs PostgreSQL and TimescaleDB if needed and tunes TimescaleDB
2. For a local database, creates the database user and a UTF-8 database
3. Applies the schema migrations and creates the superadmin account
4. Generates a CA and server certificate if TLS is enabled
5. Generates the secret-encryption key
6. Writes `/etc/spectra/server.json`, then enables and starts `spectra-server`

Then open the external URL you chose during setup and log in as the superadmin.

### Unattended setup

`spectra-setup` also accepts a YAML file:

```bash
spectra-setup -from setup.yaml [-config /etc/spectra/server.json]
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
  external_url: https://192.0.2.10:8080
tls:
  enabled: true
  sans:
    - 192.0.2.10
skip_prerequisites: false
skip_service_start: false
```

| Key | Default | Notes |
|---|---|---|
| `database.host`, `port`, `name`, `user` | `localhost`, `5432`, `spectra`, `spectra` | `name` and `user` must be plain identifiers |
| `database.password` | (required) | |
| `database.ssl` | `disable` | `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |
| `database.create` | `false` | Create the user and database. Requires a local database host. |
| `admin.username`, `admin.password` | (required) | Password of at least 8 characters |
| `server.port` | `8080` | |
| `server.external_url` | Detected | The URL agents and browsers use to reach the server |
| `tls.enabled`, `tls.sans` | `false` | SANs are added to `localhost`, `127.0.0.1`, and `::1` |
| `skip_prerequisites` | `false` | Skip the PostgreSQL and TimescaleDB install |
| `skip_service_start` | `false` | Don't enable or start the systemd service |

### Files on the server

| Path | Contents |
|---|---|
| `/opt/spectra/spectra-server` | Server binary |
| `/opt/spectra/releases/` | Agent binaries and `checksums.sha256`, served for provisioning and updates |
| `/etc/spectra/server.json` | Server configuration |
| `/etc/spectra/spectra.env` | Environment for the service: secret key, tracing settings |
| `/etc/spectra/tls/` | `ca.crt`, `server.crt`, `server.key` when TLS is enabled |
| `/var/log/spectra/server.log` | Rotated server log |

## Server configuration

The server reads `/etc/spectra/server.json` (override with `-config`):

| Field | Description |
|---|---|
| `database_url` | PostgreSQL connection URL |
| `listen_port` | HTTP or HTTPS listen port |
| `external_url` | Base URL written into agent configs and update download links |
| `tls_cert`, `tls_key`, `tls_ca` | Certificate paths. With a cert and key set, the server serves HTTPS and hands `tls_ca` to new agents. |
| `trusted_proxies` | CIDRs or addresses of reverse proxies whose `X-Forwarded-For` and `X-Real-IP` headers are trusted. Not set by setup. |

Schema migrations run automatically every time the server starts.

### Secret encryption key

Spectra encrypts recoverable secrets at rest, currently the SMTP password, with AES-256-GCM. The key comes from `SPECTRA_SECRET_KEY`, a base64-encoded 32-byte value. Setup generates it into `/etc/spectra/spectra.env`, which the systemd unit loads with `EnvironmentFile=-/etc/spectra/spectra.env`.

Without a key the server still starts, with email delivery disabled. To generate one by hand:

```bash
openssl rand -base64 32
```

Setup never overwrites an existing key, and nothing rotates it, because changing the key makes existing encrypted values unrecoverable.

### Tracing

Tracing is off by default. Set `OTEL_EXPORTER_OTLP_ENDPOINT` (or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`) in `/etc/spectra/spectra.env` to export spans over OTLP/HTTP to a collector such as Jaeger. The standard `OTEL_*` variables (`OTEL_SERVICE_NAME`, `OTEL_TRACES_SAMPLER`, `OTEL_BSP_MAX_QUEUE_SIZE`, and so on) apply. The server traces API requests by route, the phases of metric ingest, and database queries. The command long-poll is not traced.

## Deploying agents

### Publish agent binaries

The dashboard serves agent binaries from the server's `releases/` directory. Build and publish them with:

```bash
make release                                   # all targets, plus checksums.sha256
make deploy-releases DEPLOY_HOST=<server-ip>   # stage and swap into /opt/spectra/releases
```

The server only offers binaries listed in `checksums.sha256` that match their checksum. It re-reads the manifest whenever the file changes, so publishing new binaries doesn't require a restart.

### Provision an agent

In **Agent Mgmt**, choose a platform. The server issues a one-time registration token valid for 24 hours and generates install steps for that platform. The steps install the binary, the CA certificate (when TLS is enabled), the config file, and a service: a systemd unit, a launchd daemon, an rc.d script, or a Windows service. On first start, the agent registers with the token and replaces it in the config file with its permanent ID and secret.

| | Linux | macOS | FreeBSD | Windows |
|---|---|---|---|---|
| Binary | `/usr/local/bin/spectra-agent` | `/usr/local/bin/spectra-agent` | `/usr/local/bin/spectra-agent` | `C:\Spectra\spectra-agent.exe` |
| Config | `/etc/spectra/agent.json` | `/usr/local/etc/spectra/agent.json` | `/usr/local/etc/spectra/agent.json` | `C:\Spectra\agent.json` |
| CA certificate | `/etc/spectra/ca.crt` | `/usr/local/etc/spectra/ca.crt` | `/usr/local/etc/spectra/ca.crt` | `C:\Spectra\ca.crt` |
| Service | `spectra-agent` (systemd) | `com.spectra.agent` (launchd) | `spectra_agent` (rc.d) | `SpectraAgent` |
| Log | `/var/log/spectra/agent.log` | `/var/log/spectra/agent.log` | `/var/log/spectra/agent.log` | `%ProgramData%\Spectra\logs\agent.log` |

The dashboard also generates upgrade and uninstall steps for each agent.

### Agent configuration

```json
{
  "server": "https://192.0.2.10:8080",
  "token": "<one-time token>",
  "ca_cert": "/etc/spectra/ca.crt",
  "tls_skip_verify": false
}
```

| Option | Description |
|---|---|
| `-config <path>` | Config file. The default is OS-specific: `/etc/spectra/agent.json`, `/usr/local/etc/spectra/agent.json` on FreeBSD, `C:\spectra\agent.json` on Windows. |
| `-debug` | Serve `net/http/pprof` on `127.0.0.1:6060` |
| `SPECTRA_SERVER` | Server URL, used only when no config file exists |
| `HOSTNAME` | Overrides the detected hostname |
| `GOMEMLIMIT` | Overrides the agent's own soft memory limit |

From **Agent Mgmt**, admins can set an agent's log level and uncheck filesystem types and network interfaces to stop it reporting them. The agent drops ignored filesystems and interfaces before sending, so nothing new is recorded for them. Data already stored stays until retention removes it. Changes apply within a minute.

### Docker hosts

Hosts running Docker Desktop should set `"log-level": "warn"` under Settings → Docker Engine. The collector requests stats for every container once a minute, and at the default verbosity Docker Desktop logs each request. The resulting log rotation rate is high enough to hit a rename failure that takes Docker Desktop down.

## Updating

```bash
make deploy DEPLOY_HOST=<server-ip>
```

`make deploy` runs `release`, `deploy-releases`, `build-server`, and `deploy-server`, in that order. Agent binaries are published before the server is replaced; both orders are safe, because the wire format tolerates fields one side doesn't know. `deploy-server` keeps a backup of the previous binary and restores it if the restart fails. This rollback covers the binary only: migrations run when the new binary starts, so a failed restart leaves the old binary running against the migrated schema.

To update agents, select them in **Agent Mgmt** and push an update. Each agent downloads its platform's binary, verifies its SHA-256 against the server's manifest, replaces itself, and exits for its service manager to restart it.

`make setup` and the deploy targets run locally when `DEPLOY_HOST` is unset, and over SSH when it is set.

## What the agent collects

| Collector | Linux | Windows | macOS | FreeBSD | Interval | Contents |
|---|:-:|:-:|:-:|:-:|---|---|
| CPU | ✓ | ✓ | ✓ | ✓ | 5s | Total and per-core usage, iowait, load averages |
| Memory | ✓ | ✓ | ✓ | ✓ | 10s | RAM, swap or pagefile, swap paging rate, commit charge |
| Disk | ✓ | ✓ | ✓ | ✓ | 60s | Per-mount usage, filesystem, inodes |
| Disk I/O | ✓ | ✓ | ✓ | ✓ | 5s | Throughput, IOPS, latency, busy time |
| Network | ✓ | ✓ | ✓ | ✓ | 5s | Per-interface bytes, packets, errors, drops, MTU, speed |
| Processes | ✓ | ✓ | ✓ | ✓ | 15s | Per-process CPU, memory, state, threads |
| Services | ✓ | ✓ | ✓ | ✓ | 60s | systemd, Windows services, launchd, rc.d |
| Temperature | ✓ | ✓ | — | ✓ | 10s | Linux thermal zones, WMI, sysctl |
| Wi-Fi | ✓ | ✓ | — | ✓ | 30s | SSID, signal, link quality, frequency, bitrate |
| Containers | ✓ | ✓ | ✓ | ✓ | 60s | Docker containers on any platform; Proxmox LXC and VM guests on Proxmox VE hosts via `pvesh` |
| System | ✓ | ✓ | ✓ | ✓ | 300s | Uptime, boot time, process and user counts |
| Applications | ✓ | ✓ | ✓ | ✓ | Startup, then daily at 02:00 | Installed software (dpkg, rpm, pacman, apk, pkg, the Windows registry, `system_profiler`) |
| Updates | ✓ | ✓ | ✓ | ✓ | Startup, then daily at 02:05 | Pending and security updates, reboot required |
| Raspberry Pi | ✓ | | | | 10–60s | Clocks, voltages, throttle and undervoltage flags, GPU memory |

Platform notes:
- **Swap paging** (pages in and out per second) is collected on Linux, macOS, and FreeBSD. Windows has no reliable source for it. **Commit charge** is collected on Linux and Windows.
- **macOS** reports no Wi-Fi or temperature data. SSIDs require a Location Services grant that a launchd daemon can't hold, and SMC sensor keys vary by model.
- **Services on FreeBSD** report whether a service is enabled, not whether it is running.
- **Applications and updates** are collected when the agent starts, then daily at 02:00 and 02:05 in the agent's local time.
- The agent drops non-finite values and handles counter resets before computing rates.

## Alerting

The server evaluates enabled rules every 60 seconds. An alert fires once per incident and notifies each of the rule's channels. It resolves on its own when the condition clears and doesn't re-notify while it stays active. `cooldown_seconds` keeps a rule from firing again too soon after it resolves.

| Condition | Fires when | Parameters |
|---|---|---|
| `agent_offline` | The agent hasn't reported within the timeout | `timeout_seconds` (default 300) |
| `disk_prediction` | A linear fit over the last 6 hours projects the mount to fill within the window | `mount`, `warn_hours` (default 72) |
| `service_down` | The named service is not running, or is no longer reported | `service_name` |

**Scope.** A rule is either `global` (every agent) or `agent` (one agent). An agent-scoped rule overrides any global rule with the same condition for that agent. `service_down` rules must be agent-scoped.

**Channels.**

| Type | Config | Delivery |
|---|---|---|
| `webhook` | `{"url": "..."}` | POSTs a JSON payload. Private (RFC 1918 and ULA) addresses are allowed; loopback, link-local, multicast, and broadcast targets are refused, including after redirects. |
| `email` | `{"to": "..."}` | Sent through the server-wide SMTP settings |

**SMTP.** Email uses one server-wide transport configured by admins. TLS modes are `starttls` (for example port 587), `implicit` (port 465), and `none` (an unauthenticated LAN relay). The password is stored encrypted, so setting one requires `SPECTRA_SECRET_KEY`. **Send Test Message** sends a real email with the unsaved settings, so you can check them before saving.

Viewers can see rules, channels, and alert history. Creating or changing them requires an admin.

## On-demand diagnostics

Admins can run these from **Diagnostics** on any agent. Results come back over the agent's command channel. Each run has a 60-second limit; self-updates are allowed up to 10 minutes.

| Command | Description |
|---|---|
| Fetch logs | System logs filtered by severity: journald and the kernel ring buffer, Windows Event Log, macOS unified log, FreeBSD `/var/log` files |
| Disk usage | Largest directories and files under a path |
| List mounts | Mount points known to the agent |
| Ping | ICMP echo |
| Connect | TCP connect test to a host and port |
| Traceroute | Network path |
| Netstat | Active connections |

## Users and access

| Role | Can |
|---|---|
| **viewer** | Read-only: agents, metrics, labels, alert rules and history |
| **admin** | Also: provision, configure, update, label, and delete agents; manage alert rules and channels; run diagnostics; change SMTP settings and thresholds; create users and delete viewers |
| **superadmin** | Also: change roles, and delete admins and other superadmins. The first account, created at setup. |

Sessions last 24 hours in an `HttpOnly`, `SameSite=Strict` cookie, which is marked `Secure` when the server runs TLS or its external URL uses `https`. Five failed logins from one address lock that address out for 15 minutes. The server won't delete or demote the last superadmin. Agents authenticate with a per-agent secret issued at registration.

## Data storage

Every metric type has its own TimescaleDB hypertable. Chunks are compressed after 7 days and dropped after 30. Detail queries return raw rows for ranges up to 1 hour and time buckets beyond that: 1 minute up to 6 hours, 5 minutes up to 24 hours, 15 minutes up to 7 days, and 1 hour up to 30 days. Per-core CPU detail therefore covers only ranges up to 1 hour.

Besides the hypertables, the schema includes `agents`, `agent_labels`, `agent_config`, `current_metrics`, the current-state tables (`current_processes`, `current_services`, `current_applications`, `current_updates`), `users`, `sessions`, `user_config`, `registration_tokens`, `status_thresholds`, `smtp_config`, and the alerting tables (`alert_rules`, `alert_channels`, `alert_rule_channels`, `alert_events`). Migrations live in `internal/database/migrations`, are embedded in the server, and are tracked in `schema_migrations`.

## API reference

All endpoints are under `/api/v1`. Dashboard endpoints use the session cookie; agent endpoints use the agent's ID and secret. Endpoints marked **admin** or **superadmin** require at least that role. Every other dashboard endpoint is open to any logged-in user.

Metric endpoints accept `?range=5m|15m|1h|6h|24h|7d|30d` (default `1h`) or `?start=<RFC3339>&end=<RFC3339>`. The start must fall within the 30-day retention window.

**Auth and server**

| Method | Path | Description |
|---|---|---|
| POST | `/auth/login` | Log in and set the session cookie |
| POST | `/auth/logout` | Log out |
| GET | `/auth/me` | Current user and role |
| GET | `/version` | Server version (no login required) |
| GET | `/thresholds` | Status thresholds |
| PUT | `/admin/thresholds` | Update thresholds (**admin**) |

**Overview**

| Method | Path | Description |
|---|---|---|
| GET | `/overview` | Every agent with current metrics |
| GET | `/overview/page` | Paged overview: `page`, `size` (up to 250), `sort`, `order`, `status`, `os`, `arch`, `search`, repeatable `label=key:value` and `id`, `count=true` |
| GET | `/overview/stats` | Fleet status counts |
| GET | `/overview/sparklines` | Recent CPU, memory, and disk series per agent |
| GET | `/overview/fleet/chart` | Fleet-wide chart series |
| GET | `/overview/heatmap` | Fleet heatmap |

**Agents**

| Method | Path | Description |
|---|---|---|
| GET | `/agents` | List agents |
| GET | `/agents/{id}` | Agent details |
| DELETE | `/agents/{id}` | Delete an agent and its data (**admin**) |
| GET | `/agents/{id}/{metric}` | Time series; `metric` is `cpu`, `memory`, `disk`, `diskio`, `network`, `temperature`, `system`, `containers`, `wifi`, or `pi` |
| GET | `/agents/{id}/system/latest` | Latest system sample |
| GET | `/agents/{id}/processes` | Top processes (`?sort=cpu\|memory&limit=1..100`, default 20) |
| GET | `/agents/{id}/services` | Current services |
| GET | `/agents/{id}/applications` | Installed applications |
| GET | `/agents/{id}/updates` | Pending updates |
| GET | `/agents/{id}/alerts/history` | Alert events for the agent |
| GET | `/agents/{id}/config` | Remote config |
| PUT, DELETE | `/agents/{id}/config` | Set or remove a config key (**admin**) |
| GET | `/agents/{id}/upgrade-instructions` | Platform-specific upgrade steps |
| GET | `/agents/{id}/uninstall-instructions` | Platform-specific uninstall steps |

**Labels**

| Method | Path | Description |
|---|---|---|
| GET | `/agents/labels` | Labels for every agent |
| GET | `/agents/{id}/labels` | Labels for one agent |
| GET | `/labels/keys` | Known label keys |
| GET | `/labels/values?key=` | Values for a key |
| PUT, DELETE | `/admin/agents/{id}/labels/{key}` | Set or remove a user label (**admin**). Automatic keys are reserved. |

**Provisioning and commands**

| Method | Path | Description |
|---|---|---|
| GET | `/admin/platforms` | Platforms with a verified agent binary |
| POST | `/admin/provision` | Token, config, and install steps for a platform (**admin**) |
| POST | `/admin/provision/config` | Download an agent config as JSON |
| GET | `/admin/releases/{filename}` | Download an agent binary (session or registration token) |
| POST | `/admin/tokens` | Generate a registration token (**admin**) |
| POST | `/admin/tokens/revoke` | Revoke all outstanding tokens (**admin**) |
| POST | `/admin/agents/purge` | Delete offline agents (**admin**) |
| POST | `/admin/update` | Push a self-update to agents (`{"agent_ids": [...]}`) (**admin**) |
| POST | `/admin/logs`, `/admin/disk`, `/admin/network` | Queue a diagnostic (**admin**) |
| GET | `/admin/commands/{id}` | Diagnostic status and result (**admin**) |

**Users**

| Method | Path | Description |
|---|---|---|
| GET | `/admin/users` | List users |
| POST | `/admin/users` | Create a user (**admin**) |
| DELETE | `/admin/users/{id}` | Delete a user (**admin**; see [Users and access](#users-and-access)) |
| PUT | `/admin/users/{id}/role` | Change a role (**superadmin**) |
| GET, PUT, DELETE | `/user/config` | The current user's saved preferences |

**Alerting**

| Method | Path | Description |
|---|---|---|
| GET | `/alerts/channels` | List channels |
| POST | `/alerts/channels` | Create a channel (**admin**) |
| PUT, DELETE | `/alerts/channels/{id}` | Update or delete a channel (**admin**) |
| GET | `/alerts/rules` | List rules |
| GET | `/alerts/rules/{id}` | A rule with its channels |
| POST | `/alerts/rules` | Create a rule (**admin**) |
| PUT, DELETE | `/alerts/rules/{id}` | Update or delete a rule (**admin**) |
| PUT | `/alerts/rules/{id}/enabled` | Enable or disable a rule (**admin**) |
| GET | `/alerts/active` | Active alerts |
| GET | `/alerts/history` | Alert history (`?limit=&offset=`) |
| GET | `/admin/smtp` | SMTP settings, password redacted (**admin**) |
| PUT | `/admin/smtp` | Update SMTP settings (**admin**) |
| POST | `/admin/smtp/test` | Send a test email without saving (**admin**) |

**Agent protocol**

| Method | Path | Description |
|---|---|---|
| POST | `/agent/register` | Register with a one-time token |
| POST | `/agent/metrics` | Submit a gzip-compressed metric batch |
| GET | `/agent/command` | Long-poll for a command |
| POST | `/agent/command/result` | Report a command result |
| GET | `/agent/config` | Fetch remote config |

## Development

### Building

| Target | Output |
|---|---|
| `make build-server` | Frontend (`npm ci && npm run build`) and `releases/spectra-server` |
| `make build-setup` | `releases/spectra-setup` |
| `make release` | Every agent target and `releases/checksums.sha256` |
| `make build-seed` | `releases/spectra-seed` |
| `make build-pprof` | `releases/spectra-pprof` |
| `make build-agent-prof` | `dist/spectra-agent-prof-<os>-<arch>`, a profiling agent (`PROF_GOOS`, `PROF_GOARCH`, `PROF_GOARM` to cross-compile) |
| `make clean` | Removes `releases/` |

Builds stamp the version from `git describe --tags`. Every build except macOS sets `CGO_ENABLED=0`, so Linux binaries don't depend on the build host's glibc.

The server embeds `web/dist`, which isn't committed. On a fresh clone, run `make build-server` (or `cd web && npm ci && npm run build`) before `go build ./...` or `go test ./...`. Otherwise the `web` package, and everything that imports it, fails to compile.

### Testing

```bash
go test ./...                              # Go tests
go test -race ./...                        # with the race detector
go test -tags spectraprof ./internal/agent/...
cd web && npm run test:run                 # frontend (Vitest)
```

Server handlers are tested against a `MockDB` that implements the `DB` interface. Platform-specific code is selected by build tags, so each OS runs its own collector tests.

CI (`.github/workflows/ci.yaml`) builds and tests the frontend, runs the Go suite on Ubuntu, Windows, and macOS (with the race detector on Linux), builds the profiling agent, and runs `govulncheck` and `staticcheck`. CI has no FreeBSD runner.

### Code generation

SQL queries live in `internal/database/queries` and are compiled by [sqlc](https://sqlc.dev) (`sqlc.yaml`). After changing a query, run `sqlc generate` and commit the generated files. Schema changes go in a new numbered migration in `internal/database/migrations`.

### Docker stack

`docker-compose.yml` runs a disposable stack for frontend and query work. TimescaleDB runs on a tmpfs volume, and the server is built from the `Dockerfile`, set up from `docker/setup.yaml`, and seeded with a synthetic fleet on every start.

```bash
docker compose up --build        # build, set up, seed 500 agents, serve on :8080
SEED_N=2000 docker compose up    # seed a larger fleet
docker compose down -v           # tear down
```

Log in at `http://localhost:8080` as `admin` with password `changeme123`. Not for production: TLS is off and the passwords are fixed. The database lives in memory, so each start runs setup and seeds again.

### Tools

- **`spectra-seed`** inserts a synthetic fleet for frontend and query testing: `spectra-seed -db <url> -n 500 -confirm`. Remove it with `-clean`.
- **`spectra-pprof`** and the `spectraprof` build tag collect CPU and heap profiles from agents into one store for comparison. For development only: it has no authentication and binds to `127.0.0.1:9091` by default.
- **`perf/`** is a Playwright harness that measures overview page performance.
- **`Makefile.bench`** builds and runs the collector benchmarks, natively or cross-compiled for the Pi targets.

### Project structure

```
cmd/
  agent/            agent entry point (Windows service support included)
  server/           server entry point
  setup/            interactive and unattended installer
  seed/             synthetic fleet generator
  spectra-pprof/    profile aggregator
internal/
  agent/            agent runtime: scheduling, sender, cache and spool, commands, self-update
  collector/        per-metric collectors, one file per OS
  database/         sqlc output, hand-written queries, migrations
  diagnostics/      logs, disk usage, ping, traceroute, netstat
  hostinfo/         OS, architecture, and hardware detection
  inventory/        installed applications and pending updates
  labels/           automatic and user label rules
  platform/         per-OS runtime details (thermal zones, systemctl path, Pi detection)
  protocol/         wire types shared by agent and server
  secret/           AES-256-GCM secret encryption
  server/           HTTP API, auth, alert evaluator, provisioning, releases
  setup/            setup runner, prerequisites, TLS, migrations
  telemetry/        OpenTelemetry setup and the pgx tracer
  logging/, fileutil/, util/, version/, winapi/, pprofd/
web/                React 19 + TypeScript + Vite frontend, embedded via web/embed.go
deploy/             spectra-server.service
docker/             unattended setup and entrypoint for the Docker stack
perf/               frontend performance harness
scripts/            bootstrap.sh: Go and Node.js for building from source
```

## Performance

Median time for one full collection cycle (the collector's `Collect` entry point) or operation, from `go test -bench` with `-count=10`. Lower is better. Each platform collects through its native interfaces: Linux and ARM read `/proc`, sysfs, and syscalls, and Windows uses WMI and Win32 APIs. So cross-platform differences in the collection rows reflect those methods, not raw CPU speed. Pure-Go paths (marshaling, batching, header construction) compare directly. A dash means the benchmark doesn't apply on that platform: either the collector doesn't run there (Pi sensors off a Pi), or its entry point has a different shape (the Linux temperature collector is built per host from its thermal-zone paths, so it has no comparable single-call benchmark).

Collection benchmarks include real I/O, such as walking `/proc` for every process. They vary more from run to run than micro-benchmarks, which is the intent: they reflect production cost.

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

Some cross-platform differences stand out, and each comes from the collection method. Windows service enumeration through the Win32 API is far faster than parsing `systemctl` output, while WMI-backed network, process, and thermal queries cost more than their Linux `/proc` equivalents. Server benchmarks (`/overview`, metric ingest) are shown for Linux only, since the server runs on Linux.

## Known limitations

- The server runs as a single instance. Migrations take no lock, so running replicas against one database is unsupported.
- `external_url` is fleet-wide. Every agent must reach the server at that address to download updates.
- Setup creates the TLS CA and server certificate together, with no in-place way to add a SAN. Regenerating them invalidates every agent's `ca.crt`.
- FreeBSD isn't covered by CI. Its collectors are tested by hand.

## License

MIT. See [LICENSE](LICENSE).