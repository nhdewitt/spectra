import type {
    User,
    Agent,
    OverviewAgent,
    CPUMetric,
    MemoryMetric,
    DiskMetric,
    DiskIOMetric,
    NetworkMetric,
    TemperatureMetric,
    SystemMetric,
    ContainerMetric,
    WifiMetric,
    PiMetric,
    Process,
    Service,
    Application,
    Updates,
    RangeSelection,
    ProcessSort,
    PlatformInfo,
    ProvisionResponse,
    CommandResponse,
    CommandEntry,
    ManagedUser,
    AgentLabel,
    LabelKey,
    AlertChannel,
    ChannelType,
    ChannelConfig,
    AlertRule,
    RuleWithChannels,
    AlertScope,
    ConditionType,
    ConditionParams,
    RuleResponse,
    AlertEvent,
    SMTPConfig,
    SMTPConfigUpdate,
} from "./types";

declare global {
    interface Window {
        __spectraLogout?: () => void;
    }
}

const API_BASE = "/api/v1";
const LABEL_KEY_PATTERN = /^[a-z][a-z0-9_]{0,62}$/;
const MAX_LABEL_VALUE_LEN = 255;

export class HttpError extends Error {
    constructor(public status: number, message: string) {
        super(message);
        this.name = "HttpError";
    }
}

/**
 * Low-level fetch wrapper. Prepends API_BASE, includes credentials, and
 * surfaces non-2xx responses as HttpError. A 401 triggers the registered
 * logout handler (if any) before throwing. 204/empty bodies resolve to
 * null cast to T.
 */
async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
        credentials: "include",
        headers: { "Content-Type": "application/json", ...options.headers },
        ...options,
    });

    if (res.status === 401) {
        window.__spectraLogout?.();
        window.__spectraLogout = undefined;
        throw new HttpError(401, "Unauthorized");
    }

    if (!res.ok) {
        const text = await res.text();
        throw new HttpError(res.status, text || res.statusText);
    }

    if (res.status === 204) return null as T;
    const body = await res.text();
    if (!body) return null as T;
    return JSON.parse(body) as T;
}

/**
 * Builds the time-window query string for metric endpoints. Quick ranges
 * use ?range=1h; customer windows use ?start=...&end=... ISO timestamps.
 */
function rangeQuery(sel: RangeSelection): string {
    if (sel.type === "custom") {
        return `start=${encodeURIComponent(sel.start)}&end=${encodeURIComponent(sel.end)}`;
    }
    return `range=${sel.range}`;
}

/**
 * Pre-validates a label key against the same regex the server enforces.
 * Throws immediately on invalid input. Mirrors internal/labels/reserved.go's
 * keyPattern; if the server-side pattern changes, update here too.
 */
function validateLabelKey(key: string): void {
    if (!LABEL_KEY_PATTERN.test(key)) {
        throw new Error(
            `invalid label key: must match [a-z][a-z0-9_]{0,62} (got: ${JSON.stringify(key)})`
        );
    }
}

/**
 * Pre-validates a label value: non-empty and within length limit. Does not
 * check UTF-8 validity - JavaScript strings are always valid UTF-16, and
 * the server handles any edge cases on its side.
 */
function validateLabelValue(value: string): void {
    if (value === "") {
        throw new Error("label value cannot be empty");
    }
    if (value.length > MAX_LABEL_VALUE_LEN) {
        throw new Error(`label value exceeds ${MAX_LABEL_VALUE_LEN} characters`);
    }
}

const defaultRange: RangeSelection = { type: "quick", range: "1h" };

type MetricRequestOptions = {
    signal?: AbortSignal;
};

export const api = {
    // Auth

    /** POST /auth/login - establishes a session cookie. */
    login: (username: string, password: string) =>
        apiFetch<User>("/auth/login", {
            method: "POST",
            body: JSON.stringify({ username, password }),
        }),

    /** POST /auth/logout - clears the session cookie server-side. */
    logout: () => apiFetch<null>("/auth/logout", { method: "POST" }),

    /** GET /auth/me - returns the current user, or 401 if not authenticated. */
    me: () => apiFetch<User>("/auth/me"),

    // Overview / fleet

    /** GET /overview - fleet summary with latest metric snapshots per agent. */
    overview: () => apiFetch<OverviewAgent[]>("/overview"),

    /**
     * GET /overview/sparklines - recent CPU/mem/disk series per agent for the
     * overview table's inline sparklines. Map keyed by agent ID.
     */
    sparklines: () =>
        apiFetch<Record<string, { cpu: number[]; mem: number[], disk: number[] }>>("/overview/sparklines"),

    // Agent detail

    /** GET /agents/{id} - full agent record (host info, last seen, etc.). */
    agent: (id: string) => apiFetch<Agent>(`/agents/${id}`),

    /** DELETE /agents/{id} - admin+. Cascades all metric data and labels. */
    deleteAgent: (id: string) => apiFetch<null>(`/agents/${id}`, { method: "DELETE" }),

    // Agent config

    /** GET /agents/{id}/config - full config blob (ignored_filesystems, log_level, etc.). */
    agentConfig: (id: string) =>
        apiFetch<Record<string, unknown>>(`/agents/${id}/config`),

    /**
     * PUT /agents/{id}/config - admin+. Sets a single key in the agent's config blob.
     * Use the dedicated label endpoints below for labels; those are separate from
     * this generic config.
     */
    setAgentConfig: (id: string, key: string, value: unknown) =>
        apiFetch<null>(`/agents/${id}/config`, {
            method: "PUT",
            body: JSON.stringify({ key, value }),
        }),

    /** DELETE /agents/{id}/config?key=... - admin+. Removes one config key. */
    deleteAgentConfig: (id: string, key: string) =>
        apiFetch<null>(`/agents/${id}/config?key=${encodeURIComponent(key)}`, {
            method: "DELETE",
        }),

    // Time-series metrics

    /** GET /agents/{id}/cpu - CPU usage, load averages, iowait. */
    agentCPU: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<CPUMetric[]>(`/agents/${id}/cpu?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/memory - RAM and swap utilization. */
    agentMemory: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<MemoryMetric[]>(`/agents/${id}/memory?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/disk - per-mountpoint usage and inode stats. */
    agentDisk: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<DiskMetric[]>(`/agents/${id}/disk?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/diskio - per-device read/write bytes, ops, latency. */
    agentDiskIO: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<DiskIOMetric[]>(`/agents/${id}/diskio?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/network - per-interface rx/tx counters and link info. */
    agentNetwork: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<NetworkMetric[]>(`/agents/${id}/network?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/temperature - per-sensor temperature readings. */
    agentTemperature: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<TemperatureMetric[]>(`/agents/${id}/temperature?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/system - uptime, process count, user count. */
    agentSystem: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<SystemMetric[]>(`/agents/${id}/system?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/containers - per-container CPU/mem/net for docker/podman/LXC. */
    agentContainers: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<ContainerMetric[]>(`/agents/${id}/containers?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/wifi - SSID, signal, bitrate, link quality. */
    agentWifi: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<WifiMetric[]>(`/agents/${id}/wifi?${rangeQuery(sel)}`, opts),

    /** GET /agents/{id}/pi - Pi-specific: clocks, voltages, throttle/undervoltage flags. */
    agentPi: (id: string, sel: RangeSelection = defaultRange, opts: MetricRequestOptions = {}) =>
        apiFetch<PiMetric[]>(`/agents/${id}/pi?${rangeQuery(sel)}`, opts),

    // Current state

    /**
     * GET /agents/{id}/processes - latest process list, sorted by CPU/MEM.
     * Limit caps the number of rows returned (default 20)
     */
    agentProcesses: (id: string, sort: ProcessSort = "cpu", limit = 20) =>
        apiFetch<Process[]>(`/agents/${id}/processes?sort=${sort}&limit=${limit}`),

    /** GET /agents/{id}/services - systemd/launchd/service status. */
    agentServices: (id: string) =>
        apiFetch<Service[]>(`/agents/${id}/services`),

    /** GET /agents/{id}/applications - installed packages (nightly refresh). */
    agentApplications: (id: string) =>
        apiFetch<Application[]>(`/agents/${id}/applications`),

    /** GET /agents/{id}/updates - pending package updates (nightly refresh). */
    agentUpdates: (id: string) =>
        apiFetch<Updates>(`/agents/${id}/updates`),

    /** GET /agents/{id}/system/latest - most recent system snapshot. */
    agentSystemLatest: (id: string) =>
        apiFetch<SystemMetric>(`/agents/${id}/system/latest`),

    // Provisioning (admin+)

    /** GET /admin/platforms - list of platforms that have agent binaries available. */
    platforms: () => apiFetch<PlatformInfo[]>("/admin/platforms"),

    /**
     * POST /admin/provision - generate a one-time registration token and
     * platform-specific install instructions for a new agent.
     */
    provision: (platform: string) =>
        apiFetch<ProvisionResponse>("/admin/provision", {
            method: "POST",
            body: JSON.stringify({ platform }),
        }),

    /** POST /admin/tokens - generate a one-time registration token. */
    generateToken: () =>
        apiFetch<{ token: string }>("/admin/tokens", { method: "POST" }),

    /**
     * GET /agents/{id}/upgrade-instructions - returns platform-specific steps to
     * upgrade the agent binary. Empty steps mean the server can't determine the
     * platform (agent never fully registered or pre-phase with missing OS/arch);
     * UI should handle gracefully.
     */
    upgradeInstructions: (id: string) =>
        apiFetch<{ type: string; steps: string }>(`/agents/${id}/upgrade-instructions`),

    /**
     * GET /agents/{id}/uninstall-instructions - returns platform-specific steps to
     * remove the agent. Same empty-steps semantics as upgradeInstructions.
     */
    uninstallInstructions: (id: string) =>
        apiFetch<{ type: string; steps: string }>(`/agents/${id}/uninstall-instructions`),

    // Diagnostics (admin+)

    /** POST /admin/logs - queue a log-fetch command on the agent. */
    triggerLogs: (agentId: string, level: string = "WARNING") =>
        apiFetch<CommandResponse>(`/admin/logs?agent=${agentId}&level=${level}`, {
            method: "POST",
        }),

    /** POST /admin/disk - queue a disk-scan command (top-N largest under path). */
    triggerDisk: (agentId: string, path: string, topN: number) =>
        apiFetch<CommandResponse>(`/admin/disk?agent=${agentId}&top_n=${topN}${path ? `&path=${encodeURIComponent(path)}` : ""}`, {
            method: "POST",
        }),

    /** POST /admin/network - queue a network diagnostic command (ping/traceroute/etc.). */
    triggerNetwork: (agentId: string, action: string, target?: string) =>
        apiFetch<CommandResponse>(`/admin/network?agent=${agentId}&action=${action}${target ? `&target=${encodeURIComponent(target)}` : ""}`, {
            method: "POST",
        }),

    /**
     * GET /admin/commands/{id} - poll for the result of a previously queued diagnostic
     * command. Returns status (pending/running/done/failed) and output once complete.
     */
    commandResult: (cmdId: string) =>
        apiFetch<CommandEntry>(`/admin/commands/${cmdId}`),

    // Fleet management (admin+)

    /**
     * POST /admin/agents/purge - removes agents not seen in over 7 days. Cascades all their
     * metric data and labels.
     */
    purgeOfflineAgents: () =>
        apiFetch<{ purged: number }>("/admin/agents/purge", { method: "POST" }),

    /** POST /admin/tokens/revoke - invalidate all pending registration tokens. */
    revokeAllTokens: () =>
        apiFetch<null>("/admin/tokens/revoke", { method: "POST" }),

    /**
     * POST /admin/update - queue a binary self-update for the given agents. Returns counts
     * of queued (will update), skipped (already current), and failed (couldn't enqueue).
     */
    pushUpdate: (agentIds: string[]) =>
        apiFetch<{ queued: number; skipped: number; failed: number; }>("/admin/update", {
            method: "POST",
            body: JSON.stringify({ agent_ids: agentIds }),
        }),

    // User config (per-authenticated user)

    /** GET /user/config - the current user's config blog (starred agents, theme, etc.). */
    userConfig: () =>
        apiFetch<Record<string, unknown>>("/user/config"),

    /** PUT /user/config - set a single key on the current user's config. */
    setUserConfig: (key: string, value: unknown) =>
        apiFetch<null>("/user/config", {
            method: "PUT",
            body: JSON.stringify({ key, value }),
        }),

    /** DELETE /user/config?key=... - remove a single key. */
    deleteUserConfig: (key: string) =>
        apiFetch<null>(`/user/config?key=${encodeURIComponent(key)}`, {
            method: "DELETE",
        }),

    // User management (admin+ for list/create/delete; superadmin for role)

    /** GET /admin/users - list all users with their roles. */
    listUsers: () =>
        apiFetch<ManagedUser[]>("/admin/users"),

    /** POST /admin/users - create a new user. */
    createUser: (username: string, password: string, role: string) =>
        apiFetch<null>("/admin/users", {
            method: "POST",
            body: JSON.stringify({ username, password, role }),
        }),

    /** DELETE /admin/users/{id} - remove a user account. */
    deleteUser: (id: string) =>
        apiFetch<null>(`/admin/users/${id}`, { method: "DELETE" }),

    /** PUT /admin/users/{id}/role - superadmin only. Change a user's role. */
    updateUserRole: (id: string, role: string) =>
        apiFetch<null>(`/admin/users/${id}/role`, {
            method: "PUT",
            body: JSON.stringify({ role }),
        }),

    // Server info

    /** GET /version - server version, commit hash, build date. */
    version: () => apiFetch<{ version: string; commit: string; date: string; }>("/version"),

    // Labels
    
    /**
     * GET /agents/{id}/labels
     * Returns all labels (auto + user) for one agent, auto first.
     */
    agentLabels: (id: string): Promise<AgentLabel[]> =>
        apiFetch(`/agents/${id}/labels`),

    /**
     * PUT /admin/agents/{id}/labels/{key} - admin+. Upserts a user label.
     * 
     * Throws synchronously if key/value fail client-side validation. Server
     * returns 403 for reserved keys, 400 for other validation failures, 409
     * if an auto label currently holds the key.
     */
    setAgentLabel: (id: string, key: string, value: string): Promise<AgentLabel> => {
        validateLabelKey(key);
        validateLabelValue(value);
        return apiFetch(`/admin/agents/${id}/labels/${key}`, {
            method: "PUT",
            body: JSON.stringify({ value }),
        });
    },

    /**
     * DELETE /admin/agents/{id}/labels/{key} - admin+. Removes a user label.
     * 
     * Server returns 403 if the key is auto-sourced, 404 if not found, 204
     * on success. Resolves to void.
     */
    deleteAgentLabel: (id: string, key: string): Promise<void> => {
        validateLabelKey(key);
        return apiFetch(`/admin/agents/${id}/labels/${key}`, {
            method: "DELETE",
        });
    },

    /**
     * GET /labels/keys - distinct label keys across the fleet, with source.
     * Used by the filter UI's key picker; the source flag lets the UI mark
     * auto keys as read-only with editing.
     */
    labelKeys: (): Promise<LabelKey[]> =>
        apiFetch(`/labels/keys`),

    /**
     * GET /labels/values?key={key} - distinct values for a given key. Used
     * for the filter UI's value autocomplete and the rule editor.
     */
    labelValues: (key: string): Promise<string[]> => {
        validateLabelKey(key);
        return apiFetch(`/labels/values?key=${key}`);
    },

    /** GET /agents - list registered agents. */
    agents: () => apiFetch<Agent[]>("/agents"),

    // Alerting - channels

    /** GET /alerts/channels - all configured channels, ordered by name. */
    listAlertChannels: () =>
        apiFetch<AlertChannel[]>("/alerts/channels"),

    /** POST /alerts/channels - create a webhook or email channel. */
    createAlertChannel: (name: string, type: ChannelType, config: ChannelConfig) =>
        apiFetch<AlertChannel>("/alerts/channels", {
            method: "POST",
            body: JSON.stringify({ name, type, config }),
        }),

    /** PUT /alerts/channels/{id} - update a channel. */
    updateAlertChannel: (id: string, name: string, type: ChannelType, config: ChannelConfig) =>
        apiFetch<AlertChannel>(`/alerts/channels/${id}`, {
            method: "PUT",
            body: JSON.stringify({ name, type, config }),
        }),

    /** DELETE /alerts/channels/{id} - remove a channel and its rule associations. */
    deleteAlertChannel: (id: string) =>
        apiFetch<null>(`/alerts/channels/${id}`, { method: "DELETE" }),

    // Alerting - rules

    /** GET /alerts/rules - all rules, newest first. */
    listAlertRules: () =>
        apiFetch<AlertRule[]>("/alerts/rules"),

    /** GET /alerts/rules/{id} - a rule with its attached channels. */
    getAlertRule: (id: string) =>
        apiFetch<RuleWithChannels>(`/alerts/rules/${id}`),

    /**
     * POST /alerts/rules - create a rule and attach channels. scope, agent_id,
     * and condition_type are fixed at creation. Returns the rule plus warnings.
     */
    createAlertRule: (rule: {
        name: string;
        enabled: boolean;
        scope: AlertScope;
        agent_id?: string;
        condition_type: ConditionType;
        condition_params: ConditionParams;
        cooldown_seconds: number;
        channel_ids: string[];
    }) =>
        apiFetch<RuleResponse>("/alerts/rules", {
            method: "POST",
            body: JSON.stringify(rule),
        }),

    /**
     * PUT /alerts/rules/{id} - update mutable fields (name, enabled, 
     * condition_params, cooldown, channels) and re-sync channels.
     * 
     * scope, agent, and condition_type are immutable and ignored if sent.
     */
    updateAlertRule: (id: string, rule: {
        name: string;
        enabled: boolean;
        condition_params: ConditionParams;
        cooldown_seconds: number;
        channel_ids: string[];
    }) =>
        apiFetch<RuleResponse>(`/alerts/rules/${id}`, {
            method: "PUT",
            body: JSON.stringify(rule),
        }),

    /** PUT /alerts/rules/{id}/enabled - toggle a rule on or off. */
    setAlertRuleEnabled: (id: string, enabled: boolean) =>
        apiFetch<AlertRule>(`/alerts/rules/${id}/enabled`, {
            method: "PUT",
            body: JSON.stringify({ enabled }),
        }),

    /** DELETE /alerts/rules/{id} - remove a rule, its channels, and its events. */
    deleteAlertRule: (id: string) =>
        apiFetch<null>(`/alerts/rules/${id}`, { method: "DELETE" }),

    // Alerting - events

    /** GET /alerts/active - currently firing (unresolved) alerts. */
    activeAlerts: () =>
        apiFetch<AlertEvent[]>("/alerts/active"),

    /** GET /alerts/history - paginated event history across all agents. */
    alertHistory: (limit = 50, offset = 0) =>
        apiFetch<AlertEvent[]>(`/alerts/history?limit=${limit}&offset=${offset}`),

    /** GET /agents/{id}/alerts/history - paginated event history for one agent. */
    agentAlertHistory: (id: string, limit = 50, offset = 0) =>
        apiFetch<AlertEvent[]>(`/agents/${id}/alerts/history?limit=${limit}&offset=${offset}`),

    // SMTP transport (admin)

    /** GET /admin/smtp - current SMTP config (password redacted). */
    smtpConfig: () =>
        apiFetch<SMTPConfig>("/admin/smtp"),

    /**
     * PUT /admin/smtp - update SMTP config. password is three-state: omit to
     * leave unchanged, "" to clear, a value to set. Setting a password requires
     * the server's encryption key to be configured.
     */
    updateSMTPConfig: (config: SMTPConfigUpdate) =>
        apiFetch<SMTPConfig>("/admin/smtp", {
            method: "PUT",
            body: JSON.stringify(config),
        }),

    /**
     * POST /admin/smtp/test - send a test message using the supplied config
     * (live send). Resolves to {status:"sent"} or throws on failure.
     */
    testSMTPConfig: (config: SMTPConfigUpdate) =>
        apiFetch<{ status: string }>("/admin/smtp/test", {
            method: "POST",
            body: JSON.stringify(config),
        }),
};