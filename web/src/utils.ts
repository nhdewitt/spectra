import { themeVars } from "./theme";
import type { OverviewAgent, Thresholds } from "./types";

export type AgentStatus = "online" | "warn" | "crit" | "stale" | "offline";

export interface AgentStatusResult {
    status: AgentStatus;
    reasons: string[];
}

/**
 * Convert a byte count into a human-readable string using binary (base-1024) units.
 *
 * Values are scaled to the largest unit where the magnitude is >= 1, rounded to
 * one decimal place. Nullish or zero values return "0 B".
 *
 * Examples:
 *   1536 -> "1.5 KB"
 *   1073741824 -> "1.0 GB"
 *
 * @param bytes Number of bytes to format.
 * @returns     Human-readable byte string with unit suffix.
 */
export function formatBytes(bytes: number | null | undefined): string {
    if (bytes === 0 || bytes == null) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i] ?? "TB"}`;
}

/**
 * Format uptime in seconds into a concise human-readable duration.
 *
 * Behavior:
 *   • Null/undefined/0 -> "—"
 *   • >= 1 day -> "Xd Yh"
 *   • < 1 day -> "Xh Ym"
 *
 * This intentionally favors compact display for dashboard contexts rather than
 * precise duration formatting.
 *
 * @param seconds   Uptime duration in seconds.
 * @returns         Compact uptime string.
 */
export function formatUptime(seconds: number | null | undefined): string {
    if (!seconds) return "—";
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    if (d > 0) return `${d}d ${h}h`;
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
}

/**
 * Determine the health color of an agent based on last heartbeat timestamp,
 * using the configured stale/offline windows.
 *
 * Thresholds:
 *   • < stale_seconds    -> themeVars.ok
 *   • < offline_seconds  -> themeVars.warn
 *   • >= offline_seconds -> themeVars.danger
 *   • Missing timestamp  -> themeVars.textDim
 *
 * @param agent Agent object containing last_seen timestamp.
 * @param t     Global status thresholds.
 * @returns     Hex color representing agent freshness state.
 */
export function statusColor(agent: { last_seen: string | null }, t: Thresholds): string {
    if (!agent.last_seen) return themeVars.textDim;
    const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
    if (ago < t.stale_seconds) return themeVars.ok;
    if (ago < t.offline_seconds) return themeVars.warn;
    return themeVars.danger;
}

/**
 * Determine the health color of an agent based on last heartbeat timestamp.
 *
 * Thresholds:
 *   • value >= thresholds[2] -> themeVars.danger
 *   • value >= thresholds[1] -> themeVars.warn
 *   • otherwise              -> themeVars.textMuted
 *
 * @param agent Object containing a last_seen timestamp.
 * @returns     Hex color representing agent freshness state.
 */
export function severityColor(
    value: number,
    thresholds: [number, number, number]
): string {
    if (value >= thresholds[2]) return themeVars.danger;
    if (value >= thresholds[1]) return themeVars.warn;
    return themeVars.textMuted;
}

/**
 * Sort agents descending by aggregate resource pressure score.
 *
 * The score is computed as the sum of:
 *   • CPU usage percentage
 *   • Maximum disk usage percentage
 *   • Memory usage percentage
 *
 * Nullish values are treated as zero. A shallow copy is returned to preserve
 * input immutability.
 *
 * @param agents    List of agents to sort.
 * @returns         New array sorted from highest to lowest severity score.
 */
export function sortAgentsBySeverity(agents: OverviewAgent[]): OverviewAgent[] {
    return [...agents].sort((a, b) => {
        const scoreA = 
            (a.cpu_usage ?? 0) + (a.disk_max_percent ?? 0) + (a.ram_percent ?? 0);
        const scoreB =
            (b.cpu_usage ?? 0) + (b.disk_max_percent ?? 0) + (b.ram_percent ?? 0);
        return scoreB - scoreA;
    });
}

/**
 * Determine the status rank of an agent for sorting purposes, using the
 * configured stale/offline windows.
 *
 * Rank:
 *  0 = online  (last_seen < stale_seconds ago)
 *  1 = stale   (last_seen < offline_seconds ago)
 *  2 = offline (last_seen >= offline_seconds ago or missing)
 *
 * @param agent Agent with last_seen timestamp.
 * @param t     Global status thresholds.
 * @returns     Numeric rank where lower = healthier.
 */
function statusRank(agent: OverviewAgent, t: Thresholds): number {
    if (!agent.last_seen) return 2;
    const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
    if (ago < t.stale_seconds) return 0;
    if (ago < t.offline_seconds) return 1;
    return 2;
}

/**
 * Sort agents by status group, then hostname, with stable tie-breakers.
 *
 * Groups (in order): online, stale, offline.
 * Within each group, agents are sorted alphabetically by hostname.
 * If hostname matches, fall back to platform/OS and finally agent ID
 * so order stays deterministic across refreshes.
 *
 * @param agents List of agents to sort.
 * @param t      Global status thresholds (for the stale/offline windows).
 */
export function sortAgentsByStatus(agents: OverviewAgent[], t: Thresholds): OverviewAgent[] {
    return [...agents].sort((a, b) => {
        const rankDiff = statusRank(a, t) - statusRank(b, t);
        if (rankDiff !== 0) return rankDiff;

        const hostDiff = a.hostname.localeCompare(b.hostname, undefined, {
            sensitivity: "base",
        });
        if (hostDiff !== 0) return hostDiff;

        const osDiff = (a.os ?? "").localeCompare(b.os ?? "", undefined, {
            sensitivity: "base",
        });
        if (osDiff !== 0) return osDiff;

        const archDiff = (a.arch ?? "").localeCompare(b.arch ?? "", undefined, {
            sensitivity: "base",
        });
        if (archDiff !== 0) return archDiff;

        return a.id.localeCompare(b.id);
    })
}

export function agentStatus(agent: OverviewAgent, t: Thresholds): AgentStatusResult {
    if (!agent.last_seen) return { status: "offline", reasons: ["No heartbeat received"] };
    const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
    if (ago >= t.offline_seconds) return { status: "offline", reasons: [`Last seen ${Math.floor(ago / 60)}m ago`] };
    if (ago >= t.stale_seconds) return { status: "offline", reasons: [`Last seen ${Math.floor(ago / 60)}m ago`] };

    const cpu = agent.cpu_usage ?? 0;
    const mem = agent.ram_percent ?? 0;
    const disk = agent.disk_max_percent ?? 0;
    const temp = agent.max_temp ?? 0;

    const reasons: string[] = [];
    if (cpu >= t.cpu_warn) reasons.push(`CPU at ${cpu.toFixed(1)}%`);
    if (mem >= t.mem_warn) reasons.push(`Memory at ${mem.toFixed(1)}%`);
    if (disk >= t.disk_warn) reasons.push(`Disk at ${disk.toFixed(1)}%`);
    if (temp >= t.temp_warn) reasons.push(`Temperature at ${temp.toFixed(0)}°C`);

    if (cpu >= t.cpu_crit || mem >= t.mem_crit || disk >= t.disk_crit || temp >= t.temp_crit) {
        return { status: "crit", reasons };
    }

    return (reasons.length > 0) ? { status: "warn", reasons } : { status: "online", reasons: [] };
}

export function agentStatusColor(status: AgentStatus): string {
    switch (status) {
        case "online": return themeVars.ok;
        case "warn": return themeVars.warn;
        case "crit": return themeVars.danger;
        case "stale": return themeVars.warn;
        case "offline": return themeVars.textDim;
    }
}

export function copyToClipboard(text: string): Promise<void> {
    if (navigator.clipboard) {
        return navigator.clipboard.writeText(text);
    } else {
        legacyCopy(text);
        return Promise.resolve();
    }
}

function legacyCopy(text: string) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    document.body.removeChild(textarea);
}

export function timeAgo(dateStr: string | null): string {
    if (!dateStr) return "—";
    const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
    if (seconds < 60) return "just now";
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days}d ago`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months}mo ago`;
    const years = Math.floor(months / 12);
    return `${years}y ago`;
}

/**
 * Link speed for display.
 * 
 * The agent normalizes every platform to bits per second before sending:
 *  - Linux reads /sys/class/net/<if>/speed (megabits) and multiplies by 1e6
 *  - Windows reports ReceiveLinkSpeed in bps
 *  - BSD uses Baudrate
 * 
 * Returns null for 0, which is what every collector reports when the speed
 * is unavailable, so callers can omit the field instead of printing 0bps.
 */
export function formatNetworkRate(bitsPerSecond: number): string | null {
    if (!Number.isFinite(bitsPerSecond) || bitsPerSecond <= 0) return null;

    const units: [number, string][] = [
        [1e12, "Tbps"],
        [1e9, "Gbps"],
        [1e6, "Mbps"],
        [1e3, "Kbps"],
    ];

    for (const [limit, suffix] of units) {
        if (bitsPerSecond >= limit) {
            // Number() drops a trailing .0, so 1Gbps stays 1Gbps while
            // 2.5Gbps keeps the fraction
            return `${Number((bitsPerSecond / limit).toFixed(1))}${suffix}`;
        }
    }
    return `${bitsPerSecond}bps`;
}