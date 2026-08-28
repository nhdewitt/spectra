import { themeVars } from "../theme";
import type { Anomaly } from "../anomaly";

function severityColor(severity: Anomaly["severity"]): string {
	return severity === "crit" ? themeVars.danger : themeVars.warn;
}

function formatTime(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return "";
	return d.toLocaleString(undefined, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
}

/**
 * Findings for one drill-in panel.
 * 
 * Renders nothing when clean rather than an "all good" box.
 */
export function AnomalyBanner({ anomalies }: { anomalies: Anomaly[] }) {
	if (anomalies.length === 0) return null;

	const worst = anomalies.some((a) => a.severity === "crit") ? "crit" : "warn";
	const accent = severityColor(worst);

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
                {anomalies.length} {anomalies.length === 1 ? "finding" : "findings"}
            </div>
 
            <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "grid", gap: 4 }}>
                {anomalies.map((a) => (
                    <li
                        key={`${a.key}-${a.kind}`}
                        style={{ display: "flex", gap: 8, alignItems: "baseline", color: themeVars.textMuted }}
                    >
                        <span
                            style={{
                                color: severityColor(a.severity),
                                fontWeight: 600,
                                minWidth: 90,
                                flexShrink: 0,
                            }}
                        >
                            {a.label}
                        </span>
                        <span style={{ flex: 1 }}>{a.detail}</span>
                        {a.peakTime && (
                            <span style={{ color: themeVars.textDim, flexShrink: 0 }}>
                                {formatTime(a.peakTime)}
                            </span>
                        )}
                    </li>
                ))}
            </ul>
        </div>
    );
}