import { useId, useState, useRef } from "react";
import { themeVars } from "./theme";

interface SparklineProps {
    /** Array of numeric values to plot (0-100 percentages). */
    data: number[];
    /** Optional label for tooltip context ("CPU", "MEM', "DISK") */
    label?: string;
    width?: number;
    height?: number;
    /** Severity thresholds [elevated, warning, critical] for coloring. */
    thresholds?: [number, number, number];
    fillOpacity?: number;
    /** Fixed Y-axis range. Defaults to [0, 100] for percentage metrics. */
    yMin?: number;
    yMax?: number;
}

function severityColorForValue(
    value: number,
    thresholds: [number, number, number]
): string {
    if (value >= thresholds[2]) return themeVars.danger;
    if (value >= thresholds[1]) return themeVars.warn;
    return themeVars.textMuted;
}

/**
 * Minimal inline sparkline chart rendered as an SVG path.
 * 
 * The line color reflects the current (last) value's severity level.
 * Y-axis is fixed to yMin-yMax (default: 0-100) so 3% looks flat at the
 * bottom and 95% rides the top - giving immediate visual context of
 * absolute utilization, not just relative change.
 * 
 * Data is right-aligned: if fewer points than the chart width allows,
 * the line start partway though, growing from the right as data accumulates.
 * 
 * On hover, shows a tooltip with current value, peak, avg, and min over
 * // the data window (~5m at 10s polling).
 */
export function Sparkline({
    data,
    label,
    width = 80,
    height = 24,
    thresholds = [50, 80, 95],
    fillOpacity = 0.12,
    yMin = 0,
    yMax = 100,
}: SparklineProps) {
    const gradientId = useId();
    const containerRef = useRef<HTMLDivElement>(null);
    const [hovered, setHovered] = useState(false);

    if (data.length < 2) {
        return <svg width={width} height={height} />;
    }

    const padding = 1;
    const chartWidth = width - padding * 2;
    const chartHeight = height - padding * 2;
    const range = yMax - yMin || 1;

    // Current value determines line color
    const currentValue = data[data.length - 1] ?? 0;
    const color = severityColorForValue(currentValue, thresholds);

    // Tooltip stats
    const peak = Math.max(...data);
    const min = Math.min(...data);
    const avg = data.reduce((a, b) => a + b, 0) / data.length;

    // Right-align
    const maxPoints = 30;
    const xStep = chartWidth / (maxPoints - 1);
    const xOffset = (maxPoints - data.length) * xStep;

    const points = data.map((v, i) => {
        const clamped = Math.max(yMin, Math.min(yMax, v));
        const x = padding + xOffset + i * xStep;
        const strokePad = 1.5;
        const drawHeight = chartHeight - strokePad;
        const y = padding + drawHeight - ((clamped - yMin) / range) * drawHeight + strokePad / 2;
        return { x, y };
    });

    const linePath = points
        .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`)
        .join(" ");

    const firstPoint = points[0]!;
    const lastPoint = points[points.length - 1]!;
    const bottom = padding + chartHeight;
    const fillPath =
        linePath +
        ` L${lastPoint.x.toFixed(1)},${(bottom).toFixed(1)}` +
        ` L${firstPoint.x.toFixed(1)},${(bottom).toFixed(1)} Z`;

    return (
        <div
            ref={containerRef}
            style={{ position: "relative", display: "inline-block" }}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
        >
            <svg
                width={width}
                height={height}
                style={{
                    display: "block",
                    background: themeVars.surfaceHover,
                    border: `1px solid ${themeVars.border}`,
                    borderRadius: 2,
                }}
            >
                <defs>
                    <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor={color} stopOpacity={fillOpacity} />
                        <stop offset="100%" stopColor={color} stopOpacity="0" />
                    </linearGradient>
                </defs>
                <path d={fillPath} fill={`url(#${gradientId})`} />
                <path d={linePath} fill="none" stroke={color} strokeWidth="1.2" />
            </svg>

            {hovered && (
                <div
                    style={{
                        position: "absolute",
                        top: "calc(100% + 6px)",
                        left: "50%",
                        transform: "translateX(-50%)",
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        padding: "6px 10px",
                        fontFamily: themeVars.font,
                        fontSize: 11,
                        color: themeVars.text,
                        whiteSpace: "nowrap",
                        zIndex: 50,
                        pointerEvents: "none",
                        lineHeight: 1.6,
                    }}
                >
                    {label && (
                        <div
                            style={{
                                fontSize: 10,
                                color: themeVars.textDim,
                                letterSpacing: "0.04em",
                                textTransform: "uppercase",
                                marginBottom: 2,
                            }}
                        >
                            {label} · {data.length * 10}s
                        </div>
                    )}
                    <div>
                        <span style={{ color: themeVars.textMuted }}>now </span>
                        <span style={{ color: severityColorForValue(currentValue, thresholds) }}>
                            {currentValue.toFixed(1)}%
                        </span>
                    </div>
                    <div>
                        <span style={{ color: themeVars.textMuted }}>peak </span>
                        <span style={{ color: severityColorForValue(peak, thresholds) }}>
                            {peak.toFixed(1)}%
                        </span>
                    </div>
                    <div>
                        <span style={{ color: themeVars.textMuted }}>avg </span>
                        <span>{avg.toFixed(1)}%</span>
                    </div>
                    <div>
                        <span style={{ color: themeVars.textMuted }}>min </span>
                        <span>{min.toFixed(1)}%</span>
                    </div>
                </div>
            )}
        </div>
    );
}