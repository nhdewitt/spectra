import React, { useCallback, useMemo, useState } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { usePolling } from "../hooks";
import { FleetHeatmap, FleetChart, LoadingSpinner } from "../components";

type FleetView = "heatmap" | "chart";

export function Fleet() {
    const [view, setView] = useState<FleetView>("heatmap");
    const fetcher = useCallback(() => api.overview(), []);
    const { data, loading, error } = usePolling(fetcher, 10_000);
    const agents = data ?? [];

    const agentNames = useMemo(() => Object.fromEntries(agents.map((a) => [a.id, a.hostname])), [agents]);

    if (loading && agents.length === 0) return <LoadingSpinner />;

    if (error) {
        return (
            <div style={{ padding: 24, color: themeVars.danger, fontFamily: themeVars.font, }}>
                {error}
            </div>
        );
    }

    const toggleStyle = (active: boolean): React.CSSProperties => ({
        padding: "3px 10px",
        fontSize: 10,
        fontFamily: themeVars.font,
        color: active ? themeVars.text : themeVars.textMuted,
        background: active ? themeVars.accentDim : "transparent",
        border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
        cursor: "pointer",
        letterSpacing: "0.03em",
    })

    return (
        <div style={{ padding: 24 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: 16 }}>
                <button onClick={() => setView("heatmap")} style={toggleStyle(view === "heatmap")}>
                    HEATMAP
                </button>
                <button onClick={() => setView("chart")} style={toggleStyle(view === "chart")}>
                    CHART
                </button>
            </div>

            <div style={{ minHeight: 400 }}>
                {view === "heatmap" && <FleetHeatmap agents={agents} />}
                {view === "chart" && <FleetChart agentNames={agentNames} />}
            </div>
        </div>
    );
}