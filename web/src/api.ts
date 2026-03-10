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
    TimeRange,
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
    agentCPU: (id: string, range: TimeRange = "1h") =>
        apiFetch<CPUMetric[]>(`/agents/${id}/cpu?range=${range}`),
    agentMemory: (id: string, range: TimeRange = "1h") =>
        apiFetch<MemoryMetric[]>(`/agents/${id}/memory?range=${range}`),
    agentDisk: (id: string, range: TimeRange = "1h") =>
        apiFetch<DiskMetric[]>(`/agents/${id}/disk?range=${range}`),
    agentDiskIO: (id: string, range: TimeRange = "1h") =>
        apiFetch<DiskIOMetric[]>(`/agents/${id}/diskio?range=${range}`),
    agentNetwork: (id: string, range: TimeRange = "1h") =>
        apiFetch<NetworkMetric[]>(`/agents/${id}/network?range=${range}`),
    agentTemperature: (id: string, range: TimeRange = "1h") =>
        apiFetch<TemperatureMetric[]>(`/agents/${id}/temperature?range=${range}`),
    agentSystem: (id: string, range: TimeRange = "1h") =>
        apiFetch<SystemMetric[]>(`/agents/${id}/system?range=${range}`),
    agentContainers: (id: string, range: TimeRange = "1h") =>
        apiFetch<ContainerMetric[]>(`/agents/${id}/containers?range=${range}`),
    agentWifi: (id: string, range: TimeRange = "1h") =>
        apiFetch<WifiMetric[]>(`/agents/${id}/wifi?range=${range}`),
    agentPi: (id: string, range: TimeRange = "1h") =>
        apiFetch<PiMetric[]>(`/agents/${id}/pi?range=${range}`),

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