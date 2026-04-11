import React, { useState, useCallback, useEffect, useRef, useMemo } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { tableHeaderStyle, tableCellStyle, tableMutedCellStyle, LoadingSpinner } from "./ui";
import { themeVars } from "../theme";
import type { CommandResponse, CommandEntry } from "../types";

interface DiagnosticsPanelProps {
    agentId: string;
}

function useCommandPoller(cmdId: string | null) {
    const [entry, setEntry] = useState<CommandEntry | null>(null);
    const [error, setError] = useState<string | null>(null);
    const intervalRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

    useEffect(() => {
        if (!cmdId) {
            setEntry(null);
            setError(null);
            return;
        }

        const poll = async () => {
            try {
                const result = await api.commandResult(cmdId);
                setEntry(result);
                if (result.done) {
                    clearInterval(intervalRef.current);
                }
            } catch (err) {
                setError(err instanceof Error ? err.message : "Failed to poll");
                clearInterval(intervalRef.current);
            }
        };

        poll();
        intervalRef.current = setInterval(poll, 1000);

        return () => clearInterval(intervalRef.current);
    }, [cmdId]);

    return { entry, error };
}

interface LogEntry {
    timestamp: number;
    source: string;
    level: string;
    message: string;
    pid?: number;
    process_name?: string;
}

function levelColor(level: string): string {
    switch (level) {
        case "EMERGENCY":
        case "ALERT":
        case "CRITICAL":
            return themeVars.danger;
        case "ERROR":
            return themeVars.danger;
        case "WARNING":
            return themeVars.warn;
        case "NOTICE":
            return themeVars.accent;
        default:
            return themeVars.textMuted;
    }
};

function LogResults({
    entries,
    onClose,
}: {
    entries: LogEntry[];
    onClose: () => void;
}) {
    const [activeLevel, setActiveLevel] = useState<string | null>(null);
    const [copiedIdx, setCopiedIdx] = useState<number | null>(null);

    const levels = useMemo(() => {
        const counts: Record<string, number> = {};
        for (const e of entries) {
            counts[e.level] = (counts[e.level] || 0) + 1;
        }
        return counts;
    }, [entries]);

    const filtered = activeLevel
        ? entries.filter((e) => e.level === activeLevel)
        : entries;

    if (entries.length === 0) {
        return (
            <div
                style={{
                    position: "fixed",
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    background: "rgba(0, 0, 0, 0.6)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    zIndex: 100,
                }}
                onClick={(e) => {
                    if (e.target === e.currentTarget) onClose();
                }}
            >
                <div
                    style={{
                        background: themeVars.bg,
                        border: `1px solid ${themeVars.border}`,
                        padding: "32px 48px",
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "center",
                        gap: 12,
                    }}
                >
                    <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 13 }}>
                        No log entries found at this severity level.
                    </div>
                    <button
                        onClick={onClose}
                        style={{
                            padding: "6px 14px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.text,
                            background: themeVars.accentDim,
                            border: `1px solid ${themeVars.accent}`,
                            cursor: "pointer",
                            textTransform: "uppercase",
                            letterSpacing: "0.03em",
                        }}
                    >
                        Close
                    </button>
                </div>
            </div>
        );
    }

    return (
        <div
            style={{
                position: "fixed",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                background: "rgba(0, 0, 0, 0.6)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                zIndex: 100,
            }}
            onClick={(e) => {
                if (e.target === e.currentTarget) onClose();
            }}
        >
            <div
                style={{
                    background: themeVars.bg,
                    border: `1px solid ${themeVars.border}`,
                    width: "90%",
                    maxWidth: 1000,
                    maxHeight: "85vh",
                    display: "flex",
                    flexDirection: "column",
                }}
            >
                {/* Header */}
                <div
                    style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "center",
                        padding: "12px 16px",
                        borderBottom: `1px solid ${themeVars.border}`,
                    }}
                >
                    <div
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 13,
                            fontWeight: 600,
                            color: themeVars.text,
                        }}
                    >
                        System Logs ({filtered.length}
                        {activeLevel ? ` ${activeLevel}` : ""} of {entries.length})
                    </div>
                    <button
                        onClick={onClose}
                        style={{
                            background: "none",
                            border: "none",
                            color: themeVars.textMuted,
                            fontSize: 18,
                            cursor: "pointer",
                            fontFamily: themeVars.font,
                        }}
                    >
                        ×
                    </button>
                </div>

                {/* Severity filters */}
                <div
                    style={{
                        display: "flex",
                        gap: 4,
                        padding: "8px 16px",
                        borderBottom: `1px solid ${themeVars.border}`,
                        flexWrap: "wrap",
                    }}
                >
                    <button
                        onClick={() => setActiveLevel(null)}
                        style={{
                            padding: "3px 10px",
                            fontSize: 10,
                            fontFamily: themeVars.font,
                            color: !activeLevel ? themeVars.text : themeVars.textMuted,
                            background: !activeLevel ? themeVars.accentDim : "transparent",
                            border: `1px solid ${!activeLevel ? themeVars.accent : themeVars.border}`,
                            cursor: "pointer",
                            textTransform: "uppercase",
                            letterSpacing: "0.03em",
                        }}
                    >
                        All ({entries.length})
                    </button>
                    {Object.entries(levels)
                        .sort(([a], [b]) => severityOrder(a) - severityOrder(b))
                        .map(([level, count]) => (
                            <button
                                key={level}
                                onClick={() =>
                                    setActiveLevel(activeLevel === level ? null : level)
                                }
                                style={{
                                    padding: "3px 10px",
                                    fontSize: 10,
                                    fontFamily: themeVars.font,
                                    color:
                                        activeLevel === level
                                            ? themeVars.text
                                            : levelColor(level),
                                    background:
                                        activeLevel === level
                                            ? themeVars.accentDim
                                            : "transparent",
                                    border: `1px solid ${activeLevel === level ? themeVars.accent : themeVars.border}`,
                                    cursor: "pointer",
                                    textTransform: "uppercase",
                                    letterSpacing: "0.03em",
                                }}
                            >
                                {level} ({count})
                            </button>
                        ))}
                </div>

                {/* Log entries */}
                <div style={{ flex: 1, overflowY: "auto", padding: "0 16px" }}>
                    {filtered.map((e, i) => (
                        <div
                            key={i}
                            onClick={() => {
                                const text = `${new Date(e.timestamp * 1000).toISOString()} [${e.level}] ${e.source} ${e.message}`;
                                navigator.clipboard.writeText(text);
                                setCopiedIdx(i);
                                setTimeout(() => setCopiedIdx(null), 1500);
                            }}
                            style={{
                                padding: "4px 0",
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                borderBottom: `1px solid ${themeVars.border}`,
                                display: "grid",
                                gridTemplateColumns: "130px 70px 100px 1fr",
                                gap: 8,
                                background:
                                    i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                                cursor: "pointer",
                                outline: copiedIdx === i ? `1px solid ${themeVars.ok}` : "none",
                            }}
                        >
                            <span style={{ color: themeVars.textDim, whiteSpace: "nowrap" }}>
                                {new Date(e.timestamp * 1000).toLocaleString(undefined, {
                                    month: "short",
                                    day: "numeric",
                                    hour: "2-digit",
                                    minute: "2-digit",
                                    second: "2-digit",
                                })}
                            </span>
                            <span
                                style={{
                                    color: levelColor(e.level),
                                    fontWeight: 600,
                                    whiteSpace: "nowrap",
                                }}
                            >
                                {e.level}
                            </span>
                            <span
                                style={{
                                    color: themeVars.accent,
                                    whiteSpace: "nowrap",
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                }}
                            >
                                {e.source}
                            </span>
                            <span
                                style={{
                                    color: themeVars.text,
                                    whiteSpace: "nowrap",
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                }}
                                title={e.message}
                            >
                                {e.message}
                            </span>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}

function severityOrder(level: string): number {
    const order: Record<string, number> = {
        EMERGENCY: 0,
        ALERT: 1,
        CRITICAL: 2,
        ERROR: 3,
        WARNING: 4,
        NOTICE: 5,
        INFO: 6,
        DEBUG: 7,
    };
    return order[level] ?? 99;
}

interface DiskReport {
    root: string;
    top_dirs: Array<{ path: string; size: number; count?: number }>;
    top_files: Array<{ path: string; size: number }>;
    scanned_dirs: number;
    scanned_files: number;
    error_count: number;
    partial: boolean;
    duration_ms: number;
}

function DiskResults({ report }: { report: DiskReport }) {
    return (
        <div>
            {/* Summary */}
            <div
                style={{
                    display: "flex",
                    gap: 16,
                    marginBottom: 12,
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                }}
            >
                <span>Root: {report.root}</span>
                <span>Scanned: {report.scanned_dirs} dirs, {report.scanned_files} files</span>
                <span>Duration: {report.duration_ms}ms</span>
                {report.error_count > 0 && (
                    <span style={{ color: themeVars.warn }}>
                        {report.error_count} error{report.error_count === 1 ? "" : "s"}
                    </span>
                )}
                {report.partial && (
                    <span style={{ color: themeVars.warn }}>Partial scan</span>
                )}
            </div>

            {/* Top directories */}
            {report.top_dirs.length > 0 && (
                <div style={{ marginBottom: 16 }}>
                    <div
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.textDim,
                            textTransform: "uppercase",
                            letterSpacing: "0.04em",
                            marginBottom: 6,
                        }}
                    >
                        Largest Directories
                    </div>
                    <table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
                        <thead>
                            <tr>
                                <th style={tableHeaderStyle}>Path</th>
                                <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Size</th>
                                <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Files</th>
                            </tr>
                        </thead>
                        <tbody>
                            {report.top_dirs.map((d, i) => (
                                <tr
                                    key={d.path}
                                    style={{
                                        background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                                    }}
                                >
                                    <td style={tableCellStyle}>{d.path}</td>
                                    <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                        {formatBytes(d.size)}
                                    </td>
                                    <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                        {d.count ?? "—"}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Top files */}
            {report.top_files.length > 0 && (
                <div>
                    <div
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.textDim,
                            textTransform: "uppercase",
                            letterSpacing: "0.04em",
                            marginBottom: 6,
                        }}
                    >
                        Largest Files
                    </div>
                    <table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
                        <thead>
                            <tr>
                                <th style={tableHeaderStyle}>Path</th>
                                <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Size</th>
                            </tr>
                        </thead>
                        <tbody>
                            {report.top_files.map((f, i) => (
                                <tr
                                    key={f.path}
                                    style={{
                                        background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                                    }}
                                >
                                    <td style={tableCellStyle}>{f.path}</td>
                                    <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                        {formatBytes(f.size)}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}

interface NetworkReport {
    action: string;
    target?: string;
    raw_output?: string;
    netstat?: Array<{
        proto: string;
        local_addr: string;
        local_port: number;
        remote_addr: string;
        remote_port: number;
        state: string;
    }>;
    ping_results?: Array<{
        seq: number;
        success: boolean;
        rtt: number;
        response: string;
        peer: string;
    }>;
}

function NetworkResults({
    report,
    onClose,
}: {
    report: NetworkReport;
    onClose: () => void;
}) {
    // Ping and traceroute stay simple — no modal needed
    if (report.action === "ping" && report.ping_results) {
        return (
            <div>
                <div
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                        marginBottom: 8,
                    }}
                >
                    Ping {report.target}
                </div>
                {report.ping_results.map((p, i) => (
                    <div
                        key={i}
                        style={{
                            padding: "3px 8px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: p.success ? themeVars.ok : themeVars.danger,
                            background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                        }}
                    >
                        seq={p.seq} {p.response} from {p.peer}{" "}
                        {p.success && `rtt=${(p.rtt / 1_000_000).toFixed(2)}ms`}
                    </div>
                ))}
            </div>
        );
    }

    if (report.action === "traceroute" && report.raw_output) {
        return (
            <pre
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 11,
                    color: themeVars.text,
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-all",
                    margin: 0,
                    padding: 8,
                    background: themeVars.surfaceHover,
                    border: `1px solid ${themeVars.border}`,
                    maxHeight: 400,
                    overflowY: "auto",
                }}
            >
                {report.raw_output}
            </pre>
        );
    }

    if (report.action === "netstat" && report.netstat) {
        return <NetstatModal entries={report.netstat} onClose={onClose} />;
    }

    return (
        <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
            No results.
        </div>
    );
}

function NetstatModal({
    entries,
    onClose,
}: {
    entries: Array<{
        proto: string;
        local_addr: string;
        local_port: number;
        remote_addr: string;
        remote_port: number;
        state: string;
    }>;
    onClose: () => void;
}) {
    const [protoFilter, setProtoFilter] = useState<string | null>(null);
    const [stateFilter, setStateFilter] = useState<string | null>(null);
    const [search, setSearch] = useState("");

    const protos = useMemo(() => {
        const counts: Record<string, number> = {};
        for (const e of entries) {
            counts[e.proto] = (counts[e.proto] || 0) + 1;
        }
        return counts;
    }, [entries]);

    const states = useMemo(() => {
        const counts: Record<string, number> = {};
        for (const e of entries) {
            if (e.state) counts[e.state] = (counts[e.state] || 0) + 1;
        }
        return counts;
    }, [entries]);

    const filtered = useMemo(() => {
        return entries.filter((e) => {
            if (protoFilter && e.proto !== protoFilter) return false;
            if (stateFilter && e.state !== stateFilter) return false;
            if (search) {
                const q = search.toLowerCase();
                const line = `${e.local_addr}:${e.local_port} ${e.remote_addr}:${e.remote_port}`.toLowerCase();
                if (!line.includes(q)) return false;
            }
            return true;
        });
    }, [entries, protoFilter, stateFilter, search]);

    const stateColor = (state: string): string => {
        switch (state) {
            case "ESTABLISHED":
                return themeVars.ok;
            case "LISTEN":
                return themeVars.accent;
            case "TIME_WAIT":
            case "CLOSE_WAIT":
                return themeVars.warn;
            case "SYN_SENT":
            case "SYN_RECV":
                return themeVars.warn;
            default:
                return themeVars.textMuted;
        }
    };

    const filterBtnStyle = (active: boolean): React.CSSProperties => ({
        padding: "3px 10px",
        fontSize: 10,
        fontFamily: themeVars.font,
        color: active ? themeVars.text : themeVars.textMuted,
        background: active ? themeVars.accentDim : "transparent",
        border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
        cursor: "pointer",
        textTransform: "uppercase",
        letterSpacing: "0.03em",
    });

    return (
        <div
            style={{
                position: "fixed",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                background: "rgba(0, 0, 0, 0.6)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                zIndex: 100,
            }}
            onClick={(e) => {
                if (e.target === e.currentTarget) onClose();
            }}
        >
            <div
                style={{
                    background: themeVars.bg,
                    border: `1px solid ${themeVars.border}`,
                    width: "90%",
                    maxWidth: 900,
                    maxHeight: "85vh",
                    display: "flex",
                    flexDirection: "column",
                }}
            >
                {/* Header */}
                <div
                    style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "center",
                        padding: "12px 16px",
                        borderBottom: `1px solid ${themeVars.border}`,
                    }}
                >
                    <div
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 13,
                            fontWeight: 600,
                            color: themeVars.text,
                        }}
                    >
                        Netstat ({filtered.length} of {entries.length})
                    </div>
                    <button
                        onClick={onClose}
                        style={{
                            background: "none",
                            border: "none",
                            color: themeVars.textMuted,
                            fontSize: 18,
                            cursor: "pointer",
                            fontFamily: themeVars.font,
                        }}
                    >
                        ×
                    </button>
                </div>

                {/* Filters */}
                <div
                    style={{
                        display: "flex",
                        gap: 12,
                        padding: "8px 16px",
                        borderBottom: `1px solid ${themeVars.border}`,
                        flexWrap: "wrap",
                        alignItems: "center",
                    }}
                >
                    {/* Protocol filter */}
                    <div style={{ display: "flex", gap: 4 }}>
                        <button
                            onClick={() => setProtoFilter(null)}
                            style={filterBtnStyle(!protoFilter)}
                        >
                            All
                        </button>
                        {Object.entries(protos).map(([proto, count]) => (
                            <button
                                key={proto}
                                onClick={() =>
                                    setProtoFilter(protoFilter === proto ? null : proto)
                                }
                                style={filterBtnStyle(protoFilter === proto)}
                            >
                                {proto} ({count})
                            </button>
                        ))}
                    </div>

                    {/* State filter */}
                    <div style={{ display: "flex", gap: 4 }}>
                        <button
                            onClick={() => setStateFilter(null)}
                            style={filterBtnStyle(!stateFilter)}
                        >
                            All States
                        </button>
                        {Object.entries(states)
                            .sort(([, a], [, b]) => b - a)
                            .map(([state, count]) => (
                                <button
                                    key={state}
                                    onClick={() =>
                                        setStateFilter(stateFilter === state ? null : state)
                                    }
                                    style={filterBtnStyle(stateFilter === state)}
                                >
                                    {state} ({count})
                                </button>
                            ))}
                    </div>

                    {/* Search */}
                    <input
                        type="text"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="Filter by address..."
                        style={{
                            padding: "3px 8px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.text,
                            background: themeVars.surface,
                            border: `1px solid ${themeVars.border}`,
                            flex: "0 1 180px",
                        }}
                    />
                </div>

                {/* Table */}
                <div style={{ flex: 1, overflowY: "auto" }}>
                    <table
                        style={{
                            width: "100%",
                            borderCollapse: "collapse",
                            textAlign: "left",
                        }}
                    >
                        <thead>
                            <tr>
                                <th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Proto</th>
                                <th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Local</th>
                                <th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Remote</th>
                                <th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>State</th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.map((n, i) => (
                                <tr
                                    key={i}
                                    style={{
                                        background:
                                            i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                                    }}
                                >
                                    <td style={tableMutedCellStyle}>{n.proto}</td>
                                    <td style={tableCellStyle}>
                                        {n.local_addr}:{n.local_port}
                                    </td>
                                    <td style={tableCellStyle}>
                                        {n.remote_addr}:{n.remote_port}
                                    </td>
                                    <td
                                        style={{
                                            ...tableCellStyle,
                                            color: stateColor(n.state),
                                        }}
                                    >
                                        {n.state}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
}

function CommandResultDisplay({ entry, tool, onClose }: { entry: CommandEntry; tool: string; onClose: () => void; }) {
    const isOverlayType = tool === "logs" || tool === "netstat";

    if (!entry.done && isOverlayType) {
        return (
            <div
                style={{
                    position: "fixed",
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    background: "rgba(0, 0, 0, 0.6)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    zIndex: 100,
                }}
                onClick={(e) => {
                    if (e.target === e.currentTarget) onClose();
                }}
            >
                <div
                    style={{
                        background: themeVars.bg,
                        border: `1px solid ${themeVars.border}`,
                        padding: "32px 48px",
                    }}
                >
                    <LoadingSpinner />
                </div>
            </div>
        );
    }

    if (!entry.done) return <LoadingSpinner />;
    if (entry.result?.error) {
        return (
            <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                Error: {entry.result.error}
            </div>
        );
    }

    const payload = entry.result?.payload;

    if (entry.type === "FETCH_LOGS") {
        return <LogResults entries={(payload as LogEntry[]) ?? []} onClose={onClose} />
    }

    if (!payload) {
        return (
            <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                No data returned.
            </div>
        );
    }

    switch (entry.type) {
        case "DISK_USAGE":
            return <DiskResults report={payload as DiskReport} />;
        case "NETWORK_DIAG":
            return <NetworkResults report={payload as NetworkReport} onClose={onClose} />;
        default:
            return (
                <pre
                    style={{
                        fontFamily: themeVars.font,
                        fontSize: 11,
                        color: themeVars.text,
                        whiteSpace: "pre-wrap",
                        margin: 0,
                    }}
                >
                    {JSON.stringify(payload, null, 2)}
                </pre>
            );
    }
}

export function DiagnosticsPanel({ agentId }: DiagnosticsPanelProps) {
    const [activeCmd, setActiveCmd] = useState<string | null>(null);
    const [activeTool, setActiveTool] = useState<string | null>(null);
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const [pingTarget, setPingTarget] = useState("");
    const [tracerouteTarget, setTracerouteTarget] = useState("");
    const [diskPath, setDiskPath] = useState("");
    const [diskTopN, setDiskTopN] = useState(20);
    const [logLevel, setLogLevel] = useState("WARNING");

    const LOG_LEVELS = ["DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY"];

    const { entry, error: pollError } = useCommandPoller(activeCmd);

    const runCommand = useCallback(
        async (fn: () => Promise<CommandResponse>, tool: string) => {
            setSending(true);
            setSendError(null);
            setActiveTool(tool);
            try {
                const res = await fn();
                setActiveCmd(res.command_id);
            } catch (err) {
                setSendError(err instanceof Error ? err.message : "Failed to send");
            } finally {
                setSending(false);
            }
        },
        []
    );

    const inputStyle: React.CSSProperties = {
        padding: "4px 8px",
        fontSize: 12,
        fontFamily: themeVars.font,
        color: themeVars.text,
        background: themeVars.surface,
        border: `1px solid ${themeVars.border}`,
    };

    const btnStyle: React.CSSProperties = {
        padding: "6px 14px",
        fontSize: 11,
        fontFamily: themeVars.font,
        color: themeVars.text,
        background: themeVars.accentDim,
        border: `1px solid ${themeVars.accent}`,
        cursor: "pointer",
        textTransform: "uppercase",
        letterSpacing: "0.03em",
        width: 110,
        textAlign: "center",
    };

    const btnDisabled: React.CSSProperties = {
        ...btnStyle,
        opacity: 0.5,
        cursor: "default",
    };

    const rowStyle: React.CSSProperties = {
        display: "flex",
        gap: 8,
        alignItems: "center",
        marginBottom: 8,
        justifyContent: "flex-start",
    };

    const isRunning = sending || (activeCmd !== null && !entry?.done);

    return (
        <div>
            {/* Logs */}
            <div style={rowStyle}>
                <select
                    value={logLevel}
                    onChange={(e) => setLogLevel(e.target.value)}
                    disabled={isRunning}
                    style={{
                        ...inputStyle, width: 250, fontSize: 11, padding: "5px 8px",
                        textTransform: "uppercase", letterSpacing: "0.03em",
                        opacity: isRunning ? 0.5 : 1,
                    }}
                >
                    {LOG_LEVELS.map((l) => <option key={l} value={l}>{l}</option>)}
                </select>
                <button
                    onClick={() => runCommand(() => api.triggerLogs(agentId, logLevel), "logs")}
                    disabled={isRunning}
                    style={isRunning ? btnDisabled : btnStyle}
                >
                    Fetch Logs
                </button>
            </div>

            {/* Netstat */}
            <div style={rowStyle}>
                <div style={{ width: 250 }} />
                <button
                    onClick={() => runCommand(() => api.triggerNetwork(agentId, "netstat"), "netstat")}
                    disabled={isRunning}
                    style={isRunning ? btnDisabled : btnStyle}
                >
                    Netstat
                </button>
            </div>

            {/* Ping */}
            <div style={rowStyle}>
                <input type="text" value={pingTarget} onChange={(e) => setPingTarget(e.target.value)}
                    placeholder="Ping target (IP or hostname)"
                    onKeyDown={(e) => e.key === "Enter" && pingTarget.trim() &&
                        runCommand(() => api.triggerNetwork(agentId, "ping", pingTarget.trim()), "ping")}
                    style={{ ...inputStyle, width: 250 }} />
                <button
                    onClick={() => runCommand(() => api.triggerNetwork(agentId, "ping", pingTarget.trim()), "ping")}
                    disabled={isRunning || !pingTarget.trim()}
                    style={isRunning || !pingTarget.trim() ? btnDisabled : btnStyle}
                >
                    Ping
                </button>
            </div>

            {/* Traceroute */}
            <div style={rowStyle}>
                <input type="text" value={tracerouteTarget} onChange={(e) => setTracerouteTarget(e.target.value)}
                    placeholder="Traceroute target"
                    onKeyDown={(e) => e.key === "Enter" && tracerouteTarget.trim() &&
                        runCommand(() => api.triggerNetwork(agentId, "traceroute", tracerouteTarget.trim()), "traceroute")}
                    style={{ ...inputStyle, width: 250 }} />
                <button
                    onClick={() => runCommand(() => api.triggerNetwork(agentId, "traceroute", tracerouteTarget.trim()), "traceroute")}
                    disabled={isRunning || !tracerouteTarget.trim()}
                    style={isRunning || !tracerouteTarget.trim() ? btnDisabled : btnStyle}
                >
                    Traceroute
                </button>
            </div>

            {/* Disk */}
            <div style={{ ...rowStyle, marginBottom: 16 }}>
                <div style={{ display: "flex", gap: 8 }}>
                    <input type="text" value={diskPath} onChange={(e) => setDiskPath(e.target.value)}
                        placeholder="Path to scan"
                        title="Path to scan (e.g. /, /home, C:\)"
                        style={{ ...inputStyle, width: 182 }} />
                    <input type="number" value={diskTopN} onChange={(e) => setDiskTopN(Number(e.target.value) || 20)}
                        min={5} max={100}
                        style={{ ...inputStyle, width: 60 }} />
                </div>
                <button
                    onClick={() => runCommand(() => api.triggerDisk(agentId, diskPath.trim() || "/", diskTopN), "disk")}
                    disabled={isRunning}
                    style={isRunning ? btnDisabled : btnStyle}
                >
                    Scan Disk
                </button>
            </div>

            {/* Error */}
            {(sendError || pollError) && (
                <div
                    style={{
                        color: themeVars.danger,
                        fontFamily: themeVars.font,
                        fontSize: 12,
                        marginBottom: 12,
                    }}
                >
                    {sendError || pollError}
                </div>
            )}

            {/* Results */}
            {activeCmd && (
                <div
                    style={{
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        padding: 16,
                    }}
                >
                    <div
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 12,
                            fontWeight: 600,
                            color: themeVars.textMuted,
                            textTransform: "uppercase",
                            letterSpacing: "0.04em",
                            marginBottom: 12,
                        }}
                    >
                        {activeTool} results
                    </div>
                    {entry ? (
                        <CommandResultDisplay
                            entry={entry}
                            tool={activeTool ?? ""}
                            onClose={() => setActiveCmd(null)}
                        />
                    ) : (
                        <LoadingSpinner />
                    )}
                </div>
            )}
        </div>
    );
}