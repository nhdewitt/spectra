import { useMemo } from "react";
import {
    ResponsiveContainer,
    LineChart,
    AreaChart,
    Line,
    Area,
    XAxis,
    YAxis,
    Tooltip,
    CartesianGrid,
    Legend,
    ReferenceLine,
} from "recharts";
import { themeVars, getTheme, getThemeName } from "../theme";
import { LoadingText } from "./ui";
import { RangeSelection } from "../types";

type MetricRow = {
    time: string;
    [key: string]: string | number | null | undefined;
}

export interface SeriesDef {
    key: string;
    label: string;
    color?: string;
    area?: boolean;
    yAxisId?: string;
    hidden?: boolean;
}

export interface RefLine {
    y: number;
    label?: string;
    color?: string;
}

export interface MetricChartProps<T extends { time: string }> {
    title: string;
    data: T[];
    series: SeriesDef[];
    loading?: boolean;
    error?: string | null;
    unit?: string;
    yDomain?: [number, number];
    height?: number;
    formatter?: (value: number, key: string) => string;
    secondaryY?: {
        id: string;
        unit?: string;
        domain?: [number, number];
    };
    refLines?: RefLine[];
    rangeSel?: RangeSelection;
}

const GRID_OPACITY = 0.15;

const AXIS_STYLE = {
    fontSize: 10,
    fontFamily: themeVars.font,
    fill: themeVars.textDim,
};

const LEGEND_STYLE = {
    fontSize: 10,
    fontFamily: themeVars.font,
};

function palette(): string[] {
    const t = getTheme(getThemeName());
    return [
        t.accent,
        t.ok,
        t.warn,
        t.danger,
        "#a78bfa",
        "#f472b6",
        "#fb923c",
        "#38bdf8",
    ];
}

function formatTime(iso: string): string {
    const d = new Date(iso);
    const h = d.getHours().toString().padStart(2, "0");
    const m = d.getMinutes().toString().padStart(2, "0");
    return `${h}:${m}`;
}

function formatTimeFull(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
    });
}

function formatTimeByRange(iso: string, sel?: RangeSelection): string {
    const d = new Date(iso);
    const isLongRange =
        sel?.type === "custom" ||
        (sel?.type === "quick" && ["7d", "30d"].includes(sel.range));

    if (isLongRange) {
        return d.toLocaleDateString(undefined, {
            month: "short",
            day: "numeric",
        });
    }

    const h = d.getHours().toString().padStart(2, "0");
    const m = d.getMinutes().toString().padStart(2, "0");

    if (sel?.type === "quick" && sel.range === "24h") {
        return `${d.toLocaleDateString(undefined, { month: "short", day: "numeric" })} ${h}:${m}`;
    }

    return `${h}:${m}`;
}

function formatCompactNumber(v: number): string {
    if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
    if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
    return v.toFixed(1);  
}

function formatYAxisTick(v: number, unit?: string): string {
    if (unit === "%") return `${v}%`;
    return formatCompactNumber(v);
}

function ChartToolTip({
    active,
    payload,
    label,
    unit,
    formatter,
}: {
    active?: boolean;
    payload?: Array<{ name: string; value: number; color: string }>;
    label?: string;
    unit?: string;
    formatter?: (value: number, key: string) => string;
}) {
    if (!active || !payload?.length) return null;

    return (
        <div
            style={{
                background: themeVars.surface,
                border: `1px solid ${themeVars.border}`,
                padding: "8px 12px",
                fontFamily: themeVars.font,
                fontSize: 11,
            }}
        >
            <div style={{ color: themeVars.textDim, marginBottom: 4 }}>
                {label ? formatTimeFull(label) : ""}
            </div>
            {payload.map((entry) => (
                <div
                    key={entry.name}
                    style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 6,
                        marginBottom: 2,
                    }}
                >
                    <span
                        style={{
                            width: 8,
                            height: 8,
                            borderRadius: "50%",
                            background: entry.color,
                            flexShrink: 0,
                        }}
                    />
                    <span style={{ color: themeVars.textMuted }}>
                        {entry.name}
                    </span>
                    <span style={{ color: themeVars.text, fontWeight: 500 }}>
                        {formatter
                            ? formatter(entry.value, entry.name)
                            : formatCompactNumber(entry.value)}
                        {unit && !formatter ? ` ${unit}` : ""}
                    </span>
                </div>
            ))}
        </div>
    );
}

export function MetricChart<T extends { time: string} >({
    title,
    data,
    series,
    loading = false,
    error = null,
    unit,
    yDomain,
    height = 220,
    formatter,
    secondaryY,
    refLines,
    rangeSel,
}: MetricChartProps<T>) {
    const colors = useMemo(() => palette(), []);

    const resolvedSeries = useMemo(
        () =>
            series.map((s, i) => ({
                ...s,
                color: s.color ?? colors[i % colors.length],
            })),
        [series, colors]
    );

    const hasAreas = useMemo(
        () => resolvedSeries.some((s) => s.area),
        [resolvedSeries]
    );

    const ChartComponent = hasAreas ? AreaChart : LineChart;

    return (
        <div style={{ marginBottom: 24 }}>
            {/* Title */}
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 12,
                    fontWeight: 600,
                    color: themeVars.textMuted,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                    marginBottom: 8,
                }}
            >
                {title}
            </div>

            {/* Series */}
            {loading && <LoadingText />}
            {!loading && error && (
                <div
                    style={{
                        padding: 24,
                        color: themeVars.danger,
                        fontFamily: themeVars.font,
                        fontSize: 12,
                    }}
                >
                    {error}
                </div>
            )}
            {!loading && !error && data.length === 0 && (
                <div
                    style={{
                        padding: 24,
                        color: themeVars.textDim,
                        fontFamily: themeVars.font,
                        fontSize: 12,
                        textAlign: "center",
                    }}
                >
                    No data for this range.
                </div>
            )}

            {/* Chart */}
            {!loading && !error && data.length > 0 && (
                <div
                    style={{
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        padding: "12px 8px 4px 0",
                    }}
                >
                    <ResponsiveContainer width="100%" height={height}>
                        <ChartComponent data={data as Record<string, unknown>[]}>
                            <CartesianGrid
                                stroke={themeVars.border}
                                strokeOpacity={GRID_OPACITY}
                                strokeDasharray="3 3"
                            />
                            <XAxis
                                dataKey="time"
                                tickFormatter={(iso: string) => formatTimeByRange(iso, rangeSel)}
                                tick={AXIS_STYLE}
                                stroke={themeVars.border}
                                minTickGap={40}
                            />
                            <YAxis
                                yAxisId="left"
                                domain={yDomain ?? ["auto", "auto"]}
                                tick={AXIS_STYLE}
                                stroke={themeVars.border}
                                width={48}
                                tickFormatter={(v: number) => formatYAxisTick(v, unit)}
                            />
                            {secondaryY && (
                                <YAxis
                                    yAxisId={secondaryY.id}
                                    orientation="right"
                                    domain={secondaryY.domain ?? ["auto", "auto"]}
                                    tick={AXIS_STYLE}
                                    stroke={themeVars.border}
                                    width={48}
                                    tickFormatter={
                                        secondaryY.unit
                                            ? (v: number) => formatYAxisTick(v, secondaryY.unit)
                                            : undefined
                                    }
                                />
                            )}
                            <Tooltip
                                content={
                                    <ChartToolTip
                                        unit={unit}
                                        formatter={formatter}
                                    />
                                }
                            />
                            {resolvedSeries.length > 1 && (
                                <Legend wrapperStyle={LEGEND_STYLE} />
                            )}
                            {refLines?.map((rl, i) => (
                                <ReferenceLine
                                    key={i}
                                    y={rl.y}
                                    yAxisId="left"
                                    stroke={rl.color ?? themeVars.textDim}
                                    strokeDasharray="6 3"
                                    strokeOpacity={0.5}
                                    ifOverflow="extendDomain"
                                    label={{
                                        value: rl.label ?? "",
                                        position: "insideTopRight",
                                        fill: themeVars.textDim,
                                        fontSize: 10,
                                        fontFamily: themeVars.font,
                                        offset: 5,
                                    }}
                                />
                            ))}
                            {resolvedSeries.map((s) =>
                                s.area ? (
                                    <Area
                                        key={s.key}
                                        type="monotone"
                                        dataKey={s.key}
                                        name={s.label}
                                        stroke={s.color}
                                        fill={s.color}
                                        fillOpacity={0.1}
                                        strokeWidth={1.5}
                                        dot={false}
                                        yAxisId={s.yAxisId ?? "left"}
                                        hide={s.hidden}
                                        connectNulls={false}
                                    />
                                ) : (
                                    <Line
                                        key={s.key}
                                        type="monotone"
                                        dataKey={s.key}
                                        name={s.label}
                                        stroke={s.color}
                                        strokeWidth={1.5}
                                        dot={false}
                                        yAxisId={s.yAxisId ?? "left"}
                                        hide={s.hidden}
                                        connectNulls={false}
                                    />
                                )
                            )}
                        </ChartComponent>
                    </ResponsiveContainer>
                </div>
            )}
        </div>
    );
}