import { useEffect } from "react";
import { themeVars } from "../theme";
import type { Anomaly } from "../anomaly";
import { worstSeverity } from "../metricAnomalies";
import type { MetricFamily } from "./MetricDetail";

export type FindingsMap = Partial<Record<MetricFamily, Anomaly[]>>;
export type ReportFindings = (family: MetricFamily, anomalies: Anomaly[]) => void;

function severityColor(severity: "crit" | "warn"): string {
	return severity === "crit" ? themeVars.danger : themeVars.warn;
}

/**
 * Report a panel's findings up to MetricsTab.
 * 
 * The caller must pass a memoized `anomalies` array. A fresh array each render
 * would fire the effect every render, and since the parent stores the result
 * in state, that is an infinite render loop.
 */
export function useReportFindings(
	family: MetricFamily,
	anomalies: Anomaly[],
	report?: ReportFindings
) {
	useEffect(() => {
		report?.(family, anomalies);
	}, [family, anomalies, report]);
}

/**
 * Count of findings for one panel, shown in the summary grid.
 * 
 * Renders nothing when clean.
 */
export function FindingsBadge({ anomalies }: { anomalies?: Anomaly[] }) {
	const severity = worstSeverity(anomalies ?? []);
	if (!severity) return null;

	const count = anomalies!.length;
	const color = severityColor(severity);

    return (
        <span
            title={anomalies!.map((a) => `${a.label}: ${a.detail}`).join("\n")}
            style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 4,
                padding: "1px 6px",
                marginLeft: 8,
                border: `1px solid ${color}`,
                background: `color-mix(in srgb, ${color} 12%, transparent)`,
                color,
                fontFamily: themeVars.font,
                fontSize: 9,
                letterSpacing: "0.04em",
                verticalAlign: "middle",
            }}
        >
            {count} {count === 1 ? "FINDING" : "FINDINGS"}
        </span>
    );
}

export function DetailsHint() {
    return (
        <span
            style={{
                fontFamily: themeVars.font,
                fontSize: 9,
                color: themeVars.textDim,
                letterSpacing: "0.06em",
                flexShrink: 0,
            }}
        >
            DETAILS →
        </span>
    );	
}

/**
 * Aggregate findings across every panel, above the grid.
 * 
 * Each line opens its family, so this doubles as triage. It answers
 * "what is wrong with this host" without requiring the user to open
 * six panels to find out.
 */
export function FindingsStrip({
	findings,
	titles,
	onOpen,
}: {
	findings: FindingsMap;
	titles: Record<MetricFamily, string>;
	onOpen: (family: MetricFamily) => void;
}) {
	const entries = (Object.entries(findings) as [MetricFamily, Anomaly[]][])
		.filter(([, list]) => list.length > 0)
		// Crit first, then moist findings
		.sort((a, b) => {
			const rank = (l: Anomaly[]) => (l.some((x) => x.severity === "crit") ? 0 : 1);
			return rank(a[1]) - rank(b[1]) || b[1].length - a[1].length;
		});

	if (entries.length === 0) return null;

	const anyCrit = entries.some(([, list]) => list.some((a) => a.severity === "crit"));
	const accent = severityColor(anyCrit ? "crit" : "warn");
	const total = entries.reduce((n, [, list]) => n + list.length, 0);

    return (
        <div
            role="status"
            style={{
                border: `1px solid ${accent}`,
                background: `color-mix(in srgb, ${accent} 8%, transparent)`,
                padding: "10px 12px",
                marginBottom: 16,
                fontFamily: themeVars.font,
                fontSize: 11,
            }}
        >
            <div
                style={{
                    color: accent,
                    fontWeight: 600,
                    letterSpacing: "0.04em",
                    textTransform: "uppercase",
                    fontSize: 10,
                    marginBottom: 6,
                }}
            >
                {total} {total === 1 ? "finding" : "findings"} across {entries.length}{" "}
                {entries.length === 1 ? "metric" : "metrics"}
            </div>
 
            <div style={{ display: "grid", gap: 4 }}>
                {entries.map(([family, list]) => (
                    <button
                        key={family}
                        type="button"
                        onClick={() => onOpen(family)}
                        style={{
                            display: "flex",
                            gap: 8,
                            alignItems: "baseline",
                            width: "100%",
                            padding: 0,
                            border: "none",
                            background: "transparent",
                            textAlign: "left",
                            fontFamily: themeVars.font,
                            fontSize: 11,
                            color: themeVars.textMuted,
                            cursor: "pointer",
                        }}
                    >
                        <span
                            style={{
                                color: severityColor(
                                    list.some((a) => a.severity === "crit") ? "crit" : "warn"
                                ),
                                fontWeight: 600,
                                minWidth: 100,
                                flexShrink: 0,
                            }}
                        >
                            {titles[family]}
                        </span>
                        <span style={{ flex: 1 }}>
                            {list.map((a) => a.label).join(", ")}
                        </span>
                        <span style={{ color: themeVars.textDim, flexShrink: 0 }}>→</span>
                    </button>
                ))}
            </div>
        </div>
    );
}