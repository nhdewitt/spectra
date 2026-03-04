import { useId } from "react";
import { theme } from "./theme";

interface SparklineProps {
    /** Array of numeric values to plot (0-100 percentages). */
    data: number[];
    width?: number;
    height?: number;
    /** Severity thresholds [elevated, warning, critical] for coloring. */
    thresholds?: [number, number, number];
    fillOpacity?: number;
    /** Fixed Y-axis range. Defaults to [0, 100] for percentage metrics. */
    yMin?: number;
    yMax?: number;
}

const SEVERITY_COLORS = {
    normal: "#a3a3a3",
    warning: "#eab308",
    critical: "#ef4444",
};

function severityColorForValue(
    value: number,
    thresholds: [number, number, number]
): string {
    if (value >= thresholds[2]) return SEVERITY_COLORS.critical;
    if (value >= thresholds[1]) return SEVERITY_COLORS.warning;
    return SEVERITY_COLORS.normal;
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
 */
export function Sparkline({
    data,
    width = 80,
    height = 24,
    thresholds = [50, 80, 95],
    fillOpacity = 0.12,
    yMin = 0,
    yMax = 100,
}: SparklineProps) {
    const gradientId = useId();

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

    // Right-align
    const maxPoints = 30;
    const xStep = chartWidth / (maxPoints - 1);
    const xOffset = (maxPoints - data.length) * xStep;

    const points = data.map((v, i) => {
        const clamped = Math.max(yMin, Math.min(yMax, v));
        const x = padding + xOffset + i * xStep;
        const y = padding + chartHeight - ((clamped - yMin) / range) * chartHeight;
        return { x, y };
    });

    const linePath = points
        .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`)
        .join(" ");

    const firstPoint = points[0]!;
    const lastPoint = points[points.length - 1]!;
    const fillPath =
        linePath +
        ` L${lastPoint.x.toFixed(1)},${(padding + chartHeight).toFixed(1)}` +
        ` L${firstPoint.x.toFixed(1)},${(padding + chartHeight).toFixed(1)} Z`;

    return (
        <svg width={width} height={height} style={{ display: "block" }}>
            <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={color} stopOpacity={fillOpacity} />
                    <stop offset="100%" stopColor={color} stopOpacity="0" />
                </linearGradient>
            </defs>
            <path d={fillPath} fill={`url(#${gradientId})`} />
            <path d={linePath} fill="none" stroke={color} strokeWidth="1.2" />
        </svg>
    );
}