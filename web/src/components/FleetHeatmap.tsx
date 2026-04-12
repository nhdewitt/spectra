import React, { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import type { OverviewAgent, FleetHeatmapAgent } from "../types";
import { OSIcon } from "../icons";

type HeatmapMetric = "cpu" | "mem" | "disk";

interface FleetHeatmapProps {
    agents: OverviewAgent[];
}

interface LensState {
    agentIdx: number;
    hourIdx: number;
    x: number;
    y: number;
}

// --- Heatmap color scale ---

const HEATMAP_STOPS: [number, number, number][] = [
    [13, 31, 64],   // #0d1f40
    [21, 76, 153],  // #154c99
    [18, 140, 126], // #128c7e
    [234, 179, 8],  // #eab308
    [239, 68, 68],  // #ef4444
];

function heatColor(value: number | null): string {
    if (value == null) return "rgba(255,255,255,0.03)";

    const t = Math.min(Math.max(value / 100, 0), 1);
    const idx = t * (HEATMAP_STOPS.length - 1);
    const lo = Math.floor(idx);
    const hi = Math.min(lo + 1, HEATMAP_STOPS.length - 1);
    const frac = idx - lo;

    const loStop = HEATMAP_STOPS[lo]!;
    const hiStop = HEATMAP_STOPS[hi]!;

    const r = Math.round(loStop[0] + (hiStop[0] - loStop[0]) * frac);
    const g = Math.round(loStop[1] + (hiStop[1] - loStop[1]) * frac);
    const b = Math.round(loStop[2] + (hiStop[2] - loStop[2]) * frac);

    return `rgb(${r},${g},${b})`;
}

const GRADIENT_CSS = `linear-gradient(to right, ${HEATMAP_STOPS.map(
    ([r, g, b]) => `rgb(${r},${g},${b})`
).join(", ")})`;

// --- Helpers ---

const HOURS = 24;
const BUCKET_MS = 3600_000;
const ROW_HEIGHT = 22;
const ROW_GAP = 2;
const CELL_GAP = 2;
const CELL_RADIUS = 2;
const PAGE_SIZE = 20;

const LENS_ROWS = 3;
const LENS_COLS = 5;
const LENS_CELL = 44;
const LENS_GAP = 2;
const LENS_OFFSET_X = 20;

const METRIC_LABELS: Record<HeatmapMetric, string> = {
    cpu: "CPU",
    mem: "MEM",
    disk: "DISK",
};

function dayLabel(date: Date): string {
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    if (date.toDateString() === today.toDateString()) return "Today";
    if (date.toDateString() === yesterday.toDateString()) return "Yesterday";

    return date.toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
    });
}

function formatWindowLabel(start: Date, end: Date): string {
    const startLabel = dayLabel(start);
    const endLabel = dayLabel(end);

    const timeOpts: Intl.DateTimeFormatOptions = {
        hour: "2-digit",
        minute: "2-digit",
    };
    const startTime = start.toLocaleTimeString(undefined, timeOpts);
    const endTime = end.toLocaleTimeString(undefined, timeOpts);

    if (startLabel === endLabel) {
        return `${startLabel} ${startTime} – ${endTime}`;
    }
    return `${startLabel} ${startTime} – ${endLabel} ${endTime}`;
}

function getWindowBounds(offset: number): { start: Date; end: Date } {
    const now = new Date();
    const endHour = new Date(now);
    endHour.setMinutes(0, 0, 0);
    if (now.getMinutes() > 0 || now.getSeconds() > 0) {
        endHour.setHours(endHour.getHours() + 1);
    }

    const end = new Date(endHour.getTime() + offset * HOURS * BUCKET_MS);
    const start = new Date(end.getTime() - HOURS * BUCKET_MS);

    return { start, end };
}

interface GridRow {
    agentId: string;
    hostname: string;
    os: string;
    platform: string;
    values: (number | null)[];
}

// --- Heatmap ---

export function FleetHeatmap({ agents }: FleetHeatmapProps) {
    const [metric, setMetric] = useState<HeatmapMetric>("cpu");
    const [offset, setOffset] = useState(0);
    const [data, setData] = useState<FleetHeatmapAgent[]>([]);
    const [loading, setLoading] = useState(true);
    const [transitioning, setTransitioning] = useState(false);
    const [transitionDir, setTransitionDir] = useState<"left" | "right">("left");
    const [lens, setLens] = useState<LensState | null>(null);
    const [page, setPage] = useState(0);
    const [refreshTick, setRefreshTick] = useState(0);
    const prevDataRef = useRef<FleetHeatmapAgent[]>([]);
    const containerRef = useRef<HTMLDivElement>(null);

    const { start, end } = useMemo(() => getWindowBounds(offset), [offset, refreshTick]);
    const isLatest = offset === 0;

    const oldestAllowed = useMemo(() => {
        const d = new Date();
        d.setDate(d.getDate() - 30);
        return d;
    }, []);
    const canGoBack = start.getTime() > oldestAllowed.getTime();

    // Fetch data
    useEffect(() => {
        let cancelled = false;

        async function fetch() {
            setLoading(true);
            try {
                const result = await api.fleetHeatmap(
                    start.toISOString(),
                    end.toISOString()
                );
                if (!cancelled) {
                    prevDataRef.current = data;
                    setData(result);
                }
            } catch {
                // Keep previous data
            } finally {
                if (!cancelled) {
                    setLoading(false);
                    setTimeout(() => setTransitioning(false), 50);
                }
            }
        }

        fetch();
        return () => { cancelled = true };
    }, [start.getTime(), end.getTime()]);

    // Auto-refresh current window - bump tick every 60s when viewing current window
    useEffect(() => {
        if (offset !== 0) return;
        const interval = setInterval(() => setRefreshTick((t) => t + 1), 60_000);
        return () => clearInterval(interval);
    }, [offset]);

    // Get hostname/os/platform
    const agentInfoMap = useMemo(() => {
        const map = new Map<string, { hostname: string; os: string; platform: string }>();
        for (const a of agents) {
            map.set(a.id, { hostname: a.hostname, os: a.os, platform: a.platform });
        }
        return map;
    }, [agents]);

    const hourLabels = useMemo(() => {
        const labels: string[] = [];
        for (let i = 0; i < HOURS; i++) {
            const t = new Date(start.getTime() + i * BUCKET_MS);
            labels.push(
                t.toLocaleTimeString(undefined, {
                    hour: "2-digit",
                    minute: "2-digit",
                })
            );
        }
        return labels;
    }, [start.getTime()]);

    const gridData: GridRow[] = useMemo(() => {
        const startMs = start.getTime();
        return data
            .map((agent) => {
                const cells = agent[metric];
                const hourValues: (number | null)[] = new Array(HOURS).fill(null);
                for (const cell of cells) {
                    const cellTime = new Date(cell.bucket).getTime();
                    const hourIdx = Math.floor((cellTime - startMs) / BUCKET_MS);
                    if (hourIdx >= 0 && hourIdx < HOURS) {
                        hourValues[hourIdx] = cell.value;
                    }
                }
                const info = agentInfoMap.get(agent.agent_id);
                return {
                    agentId: agent.agent_id,
                    hostname: info?.hostname ?? agent.agent_id.slice(0, 8),
                    os: info?.os ?? "",
                    platform: info?.platform ?? "",
                    values: hourValues,
                };
            })
            .sort((a, b) => a.hostname.localeCompare(b.hostname));
    }, [data, metric, start.getTime(), agentInfoMap]);

    const totalPages = Math.max(1, Math.ceil(gridData.length / PAGE_SIZE));
    const pagedData = useMemo(
        () => gridData.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE),
        [gridData, page]
    );

    useEffect(() => {
        if (page >= totalPages) setPage(Math.max(0, totalPages - 1));
    }, [totalPages, page]);

    const navigateTime = useCallback((dir: -1 | 1) => {
        if (dir === 1 && isLatest) return;
        if (dir === -1 && !canGoBack) return;
        setTransitionDir(dir === -1 ? "left" : "right");
        setTransitioning(true);
        requestAnimationFrame(() => setOffset((prev) => prev + dir));
    }, [isLatest, canGoBack]);

    const lensData = useMemo(() => {
        if (!lens) return null;

        const { agentIdx, hourIdx } = lens;
        const rowStart = Math.max(0, agentIdx - 1);
        const rowEnd = Math.min(gridData.length - 1, agentIdx + 1);
        const colStart = Math.max(0, hourIdx - 2);
        const colEnd = Math.min(HOURS -1, hourIdx + 2);

        const rows: {
            hostname: string;
            os: string;
            platform: string;
            isCurrent: boolean;
            cells: { value: number | null; hour: string; isCurrent: boolean }[];
        }[] = [];

        for (let r = rowStart; r <= rowEnd; r++) {
            const row = gridData[r];
            if (!row) continue;
            const cells: { value: number | null; hour: string; isCurrent: boolean }[] = [];
            for (let c = colStart; c <= colEnd; c++) {
                cells.push({
                    value: row.values[c] ?? null,
                    hour: hourLabels[c] ?? "",
                    isCurrent: r === agentIdx && c === hourIdx,
                });
            }
            rows.push({
                hostname: row.hostname,
                os: row.os,
                platform: row.platform,
                isCurrent: r === agentIdx,
                cells,
            });
        }

        return { rows, colStart, colEnd };
    }, [lens, gridData, hourLabels]);

    const lensPosition = useMemo(() => {
        if (!lens || !containerRef.current) return { left: 0, top: 0 };

        const containerRect = containerRef.current.getBoundingClientRect();
        const lensWidth = LENS_COLS * (LENS_CELL + LENS_GAP) + 80 + 16;
        const lensHeight = LENS_ROWS * (LENS_CELL + LENS_GAP) + 30;

        let left = lens.x + LENS_OFFSET_X;
        let top = lens.y - lensHeight / 2;

        const maxLeft = containerRect.width - lensWidth - 8;
        const maxTop = containerRect.height - lensHeight - 8;

        if (left > maxLeft) left = lens.x - lensWidth - 10;
        if (left < 0) left = 8;
        if (top > maxTop) top = maxTop;
        if (top < 0) top = 8;

        return { left, top };
    }, [lens]);

    const handleCellHover = useCallback((e: React.MouseEvent, globalAgentIdx: number, hourIdx: number) => {
        if (!containerRef.current) return;
        const rect = containerRef.current.getBoundingClientRect();
        setLens({
            agentIdx: globalAgentIdx,
            hourIdx,
            x: e.clientX - rect.left,
            y: e.clientY - rect.top,
        });
    }, []);

    const navBtnStyle = (disabled: boolean): React.CSSProperties => ({
        padding: "4px 10px",
        fontSize: 14,
        fontFamily: themeVars.font,
        color: disabled ? themeVars.textDim : themeVars.text,
        background: "transparent",
        border: `1px solid ${disabled ? themeVars.border : themeVars.accent}`,
        cursor: disabled ? "default" : "pointer",
        opacity: disabled ? 0.4 : 1,
        lineHeight: 1,
    });

    const metricBtnStyle = (active: boolean): React.CSSProperties => ({
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

    const pageBtnStyle = (disabled: boolean): React.CSSProperties => ({
        padding: "2px 8px",
        fontSize: 10,
        fontFamily: themeVars.font,
        color: disabled ? themeVars.textDim : themeVars.textMuted,
        background: "transparent",
        border: `1px solid ${themeVars.border}`,
        cursor: disabled ? "default" : "pointer",
        opacity: disabled ? 0.4 : 1,
    });

    return (
        <div
            style={{
                marginBottom: 24,
                background: themeVars.surface,
                border: `1px solid ${themeVars.border}`,
                padding: 16,
            }}
        >
            {/* Toolbar */}
            <div
                style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    marginBottom: 12,
                    flexWrap: "wrap",
                    gap: 8,
                }}
            >
                {/* Time navigation */}
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <button
                        onClick={() => navigateTime(-1)}
                        disabled={!canGoBack}
                        style={navBtnStyle(!canGoBack)}
                        title="Previous 24 hours"
                    >
                        ◀
                    </button>
                    <div
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 12,
                            color: themeVars.text,
                            minWidth: 220,
                            textAlign: "center",
                        }}
                    >
                        {formatWindowLabel(start, end)}
                    </div>
                    <button
                        onClick={() => navigateTime(1)}
                        disabled={isLatest}
                        style={navBtnStyle(isLatest)}
                        title="Next 24 hours"
                    >
                        ▶
                    </button>
                    {!isLatest && (
                        <button
                            onClick={() => {
                                setTransitionDir("right");
                                setTransitioning(true);
                                requestAnimationFrame(() => setOffset(0));
                            }}
                            style={{
                                padding: "3px 8px",
                                fontSize: 10,
                                fontFamily: themeVars.font,
                                color: themeVars.accent,
                                background: "transparent",
                                border: `1px solid ${themeVars.accent}`,
                                cursor: "pointer",
                                textTransform: "uppercase",
                                letterSpacing: "0.03em",
                            }}
                        >
                            Now
                        </button>
                    )}
                </div>
 
                {/* Metric toggle */}
                <div style={{ display: "flex", gap: 4 }}>
                    {(["cpu", "mem", "disk"] as HeatmapMetric[]).map((m) => (
                        <button
                            key={m}
                            onClick={() => setMetric(m)}
                            style={metricBtnStyle(metric === m)}
                        >
                            {METRIC_LABELS[m]}
                        </button>
                    ))}
                </div>
            </div>
 
            {/* Heatmap grid + lens container */}
            <div
                ref={containerRef}
                style={{
                    position: "relative",
                    opacity: loading && data.length === 0 ? 0.4 : 1,
                    transition: "opacity 0.2s ease",
                }}
                onMouseLeave={() => setLens(null)}
            >
                <div
                    style={{
                        transform: transitioning
                            ? `translateX(${transitionDir === "left" ? "-4px" : "4px"})`
                            : "translateX(0)",
                        opacity: transitioning ? 0.6 : 1,
                        transition: "transform 0.2s ease, opacity 0.2s ease",
                    }}
                >
                    {gridData.length === 0 && !loading && (
                        <div
                            style={{
                                padding: "16px 0",
                                fontFamily: themeVars.font,
                                fontSize: 12,
                                color: themeVars.textDim,
                                textAlign: "center",
                            }}
                        >
                            No heatmap data available for this time range.
                        </div>
                    )}
 
                    {pagedData.map((row, pageRowIdx) => {
                        const globalIdx = page * PAGE_SIZE + pageRowIdx;
                        return (
                            <div
                                key={row.agentId}
                                style={{
                                    position: "relative",
                                    marginBottom: ROW_GAP,
                                    cursor: "default",
                                }}
                            >
                                {/* Overlay hostname */}
                                <div
                                    style={{
                                        position: "absolute",
                                        left: 6,
                                        top: "50%",
                                        transform: "translateY(-50%)",
                                        display: "inline-flex",
                                        alignItems: "center",
                                        gap: 3,
                                        fontSize: 9,
                                        fontFamily: themeVars.font,
                                        color: "rgba(255,255,255,0.85)",
                                        textShadow:
                                            "0 0 3px rgba(0,0,0,0.8), 0 0 6px rgba(0,0,0,0.5)",
                                        pointerEvents: "none",
                                        whiteSpace: "nowrap",
                                        letterSpacing: "0.02em",
                                        zIndex: 1,
                                    }}
                                >
                                    <OSIcon os={row.os} platform={row.platform} size={10} />
                                    {row.hostname}
                                </div>
 
                                {/* Cells */}
                                <div
                                    style={{
                                        display: "flex",
                                        gap: CELL_GAP,
                                        height: ROW_HEIGHT,
                                    }}
                                >
                                    {row.values.map((value, hourIdx) => (
                                        <div
                                            key={hourIdx}
                                            onMouseMove={(e) =>
                                                handleCellHover(e, globalIdx, hourIdx)
                                            }
                                            style={{
                                                flex: 1,
                                                background: heatColor(value),
                                                borderRadius: CELL_RADIUS,
                                                transition: "background 0.3s ease",
                                            }}
                                        />
                                    ))}
                                </div>
                            </div>
                        );
                    })}
                </div>
 
                {/* Magnifier lens */}
                {lens && lensData && (
                    <div
                        style={{
                            position: "absolute",
                            left: lensPosition.left,
                            top: lensPosition.top,
                            background: themeVars.bg,
                            border: "1px solid rgba(255,255,255,0.12)",
                            borderRadius: 6,
                            padding: 8,
                            zIndex: 50,
                            pointerEvents: "none",
                            boxShadow:
                                "0 4px 20px rgba(0,0,0,0.6), inset 0 0 24px 6px rgba(0,0,0,0.3)",
                        }}
                    >
                        {/* Glass highlight */}
                        <div
                            style={{
                                position: "absolute",
                                inset: 0,
                                borderRadius: 6,
                                background:
                                    "linear-gradient(160deg, rgba(255,255,255,0.08) 0%, transparent 40%)",
                                pointerEvents: "none",
                                zIndex: 2,
                            }}
                        />
 
                        {/* Hour labels */}
                        <div
                            style={{
                                display: "flex",
                                marginBottom: LENS_GAP,
                                alignItems: "flex-end",
                            }}
                        >
                            <div style={{ width: 70, flexShrink: 0 }} />
                            {lensData.rows[0]?.cells.map((cell, i) => (
                                <div
                                    key={i}
                                    style={{
                                        width: LENS_CELL,
                                        marginRight: LENS_GAP,
                                        fontSize: 8,
                                        fontFamily: themeVars.font,
                                        color: themeVars.textDim,
                                        textAlign: "center",
                                    }}
                                >
                                    {cell.hour}
                                </div>
                            ))}
                        </div>
 
                        {/* Rows */}
                        {lensData.rows.map((row, ri) => (
                            <div
                                key={ri}
                                style={{
                                    display: "flex",
                                    alignItems: "center",
                                    marginBottom: LENS_GAP,
                                }}
                            >
                                {/* Hostname */}
                                <div
                                    style={{
                                        width: 70,
                                        flexShrink: 0,
                                        fontSize: 9,
                                        fontFamily: themeVars.font,
                                        color: row.isCurrent
                                            ? themeVars.text
                                            : themeVars.textMuted,
                                        fontWeight: row.isCurrent ? 600 : 400,
                                        overflow: "hidden",
                                        textOverflow: "ellipsis",
                                        whiteSpace: "nowrap",
                                        display: "flex",
                                        alignItems: "center",
                                        gap: 3,
                                    }}
                                >
                                    {row.hostname}
                                    <OSIcon os={row.os} platform={row.platform} size={10} />
                                </div>
 
                                {/* Fisheye-scaled cells */}
                                {row.cells.map((cell, ci) => (
                                    <div
                                        key={ci}
                                        style={{
                                            width: LENS_CELL,
                                            height: LENS_CELL,
                                            marginRight: LENS_GAP,
                                            background: heatColor(cell.value),
                                            borderRadius: CELL_RADIUS,
                                            display: "flex",
                                            alignItems: "center",
                                            justifyContent: "center",
                                            fontSize: 10,
                                            fontFamily: themeVars.font,
                                            fontWeight: 600,
                                            color: cell.value != null && cell.value >= 50
                                                ? "#fff"
                                                : cell.value != null
                                                    ? "rgba(255,255,255,0.75)"
                                                    : "transparent",
                                            outline: cell.isCurrent
                                                ? `2px solid ${themeVars.text}`
                                                : "none",
                                            outlineOffset: -1,
                                        }}
                                    >
                                        {cell.value != null ? `${cell.value.toFixed(1)}%` : ""}
                                    </div>
                                ))}
                            </div>
                        ))}
                    </div>
                )}
            </div>
 
            {/* Pagination */}
            {totalPages > 1 && (
                <div
                    style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        gap: 8,
                        marginTop: 8,
                    }}
                >
                    <button
                        onClick={() => setPage((p) => Math.max(0, p - 1))}
                        disabled={page === 0}
                        style={pageBtnStyle(page === 0)}
                    >
                        ◀
                    </button>
                    <span
                        style={{
                            fontSize: 10,
                            fontFamily: themeVars.font,
                            color: themeVars.textMuted,
                        }}
                    >
                        {page + 1} / {totalPages}
                    </span>
                    <button
                        onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                        disabled={page >= totalPages - 1}
                        style={pageBtnStyle(page >= totalPages - 1)}
                    >
                        ▶
                    </button>
                </div>
            )}
 
            {/* Legend */}
            <div
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    marginTop: 10,
                }}
            >
                <span
                    style={{
                        fontSize: 9,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                    }}
                >
                    0%
                </span>
                <div
                    style={{
                        height: 8,
                        width: 120,
                        borderRadius: 2,
                        background: GRADIENT_CSS,
                    }}
                />
                <span
                    style={{
                        fontSize: 9,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                    }}
                >
                    100%
                </span>
                <div
                    style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 4,
                        marginLeft: 8,
                    }}
                >
                    <div
                        style={{
                            width: 8,
                            height: 8,
                            borderRadius: 2,
                            background: "rgba(255,255,255,0.03)",
                            border: `1px solid ${themeVars.border}`,
                        }}
                    />
                    <span
                        style={{
                            fontSize: 9,
                            fontFamily: themeVars.font,
                            color: themeVars.textDim,
                        }}
                    >
                        No data
                    </span>
                </div>
            </div>
        </div>
    );
}