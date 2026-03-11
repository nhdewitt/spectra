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
} from "./types";

declare global {
    interface Window {
        __spectraLogout?: () => void;
    }
}

const API_BASE = "/api/v1";

export class HttpError extends Error {
    constructor(public status: number, message: string) {
        super(message);
        this.name = "HttpError";
    }
}

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
        credentials: "include",
        headers: { "Content-Type": "application/json", ...options.headers },
        ...options,
    });

    if (res.status === 401) {
        window.__spectraLogout?.();
        throw new HttpError(401, "Unauthorized");
    }

    if (!res.ok) {
        const text = await res.text();
        throw new HttpError(res.status, text || res.statusText);
    }

    if (res.status === 204) return null as T;
    return res.json() as Promise<T>;
}

function rangeQuery(sel: RangeSelection): string {
    if (sel.type === "custom") {
        return `start=${encodeURIComponent(sel.start)}&end=${encodeURIComponent(sel.end)}`;
    }
    return `range=${sel.range}`;
}

const defaultRange: RangeSelection = { type: "quick", range: "1h" };

export const api = {
    // Auth
    login: (username: string, password: string) =>
        apiFetch<User>("/auth/login", {
            method: "POST",
            body: JSON.stringify({ username, password }),
        }),

    logout: () => apiFetch<null>("/auth/logout", { method: "POST" }),
    me: () => apiFetch<User>("/auth/me"),

    // Overview
    overview: () => apiFetch<OverviewAgent[]>("/overview"),
    sparklines: () =>
        apiFetch<Record<string, { cpu: number[]; mem: number[], disk: number[] }>>("/overview/sparklines"),

    // Agent detail
    agent: (id: string) => apiFetch<Agent>(`/agents/${id}`),
    deleteAgent: (id: string) => apiFetch<null>(`/agents/${id}`, { method: "DELETE" }),

    // Time-series metrics
    agentCPU: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<CPUMetric[]>(`/agents/${id}/cpu?${rangeQuery(sel)}`),
    agentMemory: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<MemoryMetric[]>(`/agents/${id}/memory?${rangeQuery(sel)}`),
    agentDisk: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<DiskMetric[]>(`/agents/${id}/disk?${rangeQuery(sel)}`),
    agentDiskIO: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<DiskIOMetric[]>(`/agents/${id}/diskio?${rangeQuery(sel)}`),
    agentNetwork: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<NetworkMetric[]>(`/agents/${id}/network?${rangeQuery(sel)}`),
    agentTemperature: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<TemperatureMetric[]>(`/agents/${id}/temperature?${rangeQuery(sel)}`),
    agentSystem: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<SystemMetric[]>(`/agents/${id}/system?${rangeQuery(sel)}`),
    agentContainers: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<ContainerMetric[]>(`/agents/${id}/containers?${rangeQuery(sel)}`),
    agentWifi: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<WifiMetric[]>(`/agents/${id}/wifi?${rangeQuery(sel)}`),
    agentPi: (id: string, sel: RangeSelection = defaultRange) =>
        apiFetch<PiMetric[]>(`/agents/${id}/pi?${rangeQuery(sel)}`),

    // Current state
    agentProcesses: (id: string, sort: ProcessSort = "cpu", limit = 20) =>
        apiFetch<Process[]>(`/agents/${id}/processes?sort=${sort}&limit=${limit}`),
    agentServices: (id: string) =>
        apiFetch<Service[]>(`/agents/${id}/services`),
    agentApplications: (id: string) =>
        apiFetch<Application[]>(`/agents/${id}/applications`),
    agentUpdates: (id: string) =>
        apiFetch<Updates>(`/agents/${id}/updates`),

    // Admin / Provisioning
    platforms: () => apiFetch<PlatformInfo[]>("/admin/platforms"),
    provision: (platform: string) =>
        apiFetch<ProvisionResponse>("/admin/provision", {
            method: "POST",
            body: JSON.stringify({ platform }),
        }),
    generateToken: () =>
        apiFetch<{ token: string }>("/admin/tokens", { method: "POST" }),
};