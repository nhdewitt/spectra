import { themeVars } from "./theme";
import type { OverviewAgent } from "./types";

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
 * Determine the health color of an agent based on last heartbeat timestamp.
 *
 * Thresholds:
 *   • < 2 minutes -> themeVars.ok
 *   • < 10 minutes -> themeVars.warn
 *   • >= 10 minutes -> themeVars.danger
 *   • Missing timestamp -> themeVars.textDim
 *
 * @param agent Agent object containing last_seen timestamp.
 * @returns     Hex color representing agent freshness state.
 */
export function statusColor(agent: { last_seen: string | null }): string {
    if (!agent.last_seen) return themeVars.textDim;
    const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
    if (ago < 120) return themeVars.ok;
    if (ago < 600) return themeVars.warn;
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
 * Determine the status rank of an agent for sorting purposes.
 * 
 * Rank:
 *  0 = online  (last_seen < 2 minutes ago)
 *  1 = stale   (last_seen < 10 minutes ago)
 *  2 = offline (last_seen >= 10 minutes ago or missing)
 * 
 * @param agent Agent with last_seen timestamp.
 * @returns     Numeric rank where lower = healthier.
 */
function statusRank(agent: OverviewAgent): number {
    if (!agent.last_seen) return 2;
    const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
    if (ago < 120) return 0;
    if (ago < 600) return 1;
    return 2;
}

/**
 * Sort agents by status group, then hostname, with stable tie-breakers.
 * 
 * Groups (in order): online, stale, offline.
 * Within each group, agents are sorted alphabetically by hostname.
 * If hostname matches, fall back to platform/OS and finally agent ID
 * so order stays deterministic across refreshes.
 */
export function sortAgentsByStatus(agents: OverviewAgent[]): OverviewAgent[] {
    return [...agents].sort((a, b) => {
        const rankDiff = statusRank(a) - statusRank(b);
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