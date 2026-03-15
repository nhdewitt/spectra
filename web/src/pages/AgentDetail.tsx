import { useState } from "react";
import { themeVars } from "../theme";
import { statusColor } from "../utils";
import { TimeRangePicker } from "../components";
import { MetricsTab } from "../components/MetricsTab";
import type { OverviewAgent, RangeSelection } from "../types";
import { api } from "../api";
import { usePolling } from "../hooks/usePolling";
import { ProcessesTab } from "../components/ProcessesTab";
import { ServicesTab } from "../components/ServicesTab";
import { ApplicationsTab } from "../components/ApplicationsTab";
import { UpdatesTab } from "../components/UpdatesTab";

const TABS = ["metrics", "processes", "services", "apps", "updates"] as const;

export function AgentDetail({
    agent,
    onBack,
}: {
    agent: OverviewAgent;
    onBack: () => void;
}) {
    const [rangeSel, setRangeSel] = useState<RangeSelection>({ type: "quick", range: "1h" });
    const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>("metrics");
    const { data: liveAgent } = usePolling(
        () => api.agent(agent.id),
        30_000
    );
    const lastSeen = liveAgent?.last_seen ?? agent.last_seen;

    const rangeLabel =
        rangeSel.type === "quick"
            ? rangeSel.range
            : `${new Date(rangeSel.start).toLocaleString()} — ${new Date(rangeSel.end).toLocaleString()}`;
    
    const isTimeSeriesTab = activeTab === "metrics";

    return (
        <div style={{ padding: 24 }}>
            {/* Header */}
            <div
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 16,
                    marginBottom: 24,
                }}
            >
                <button
                    onClick={onBack}
                    style={{
                        padding: "6px 12px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.textMuted,
                        background: "transparent",
                        border: `1px solid ${themeVars.border}`,
                        cursor: "pointer",
                    }}
                >
                    ← BACK
                </button>
                <div
                    style={{
                        width: 10,
                        height: 10,
                        borderRadius: "50%",
                        background: statusColor({ last_seen: lastSeen }),
                    }}
                />
                <div>
                    <div
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 18,
                            fontWeight: 600,
                            color: themeVars.text,
                        }}
                    >
                        {agent.hostname}
                    </div>
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            color: themeVars.textMuted,
                            marginTop: 2,
                        }}
                    >
                        {agent.os} · {agent.platform} · {agent.arch} · {agent.cpu_cores}{" "}
                        {agent.cpu_cores === 1 ? "core" : "cores"}
                    </div>
                </div>
            </div>

            {/* Time range picker */}
            <div
                style={{
                    marginBottom: 20,
                    opacity: isTimeSeriesTab ? 1 : 0.3,
                    pointerEvents: isTimeSeriesTab ? "auto" : "none",
                    filter: isTimeSeriesTab ? "none" : "blur(1px)",
                    transition: "opacity 0.2s, filter 0.2s",
                }}
            >
                <TimeRangePicker value={rangeSel} onChange={setRangeSel} />
            </div>

            {/* Tab bar */}
            <div
                style={{
                    display: "flex",
                    gap: 4,
                    marginBottom: 20,
                    borderBottom: `1px solid ${themeVars.border}`,
                    paddingBottom: 8,
                }}
            >
                {TABS.map((tab) => (
                    <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        style={{
                            padding: "6px 14px",
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            color: activeTab === tab ? themeVars.text : themeVars.textMuted,
                            background: activeTab === tab ? themeVars.accentDim : "transparent",
                            border: "none",
                            cursor: "pointer",
                            textTransform: "uppercase",
                            letterSpacing: "0.03em",
                        }}
                    >
                        {tab}
                    </button>
                ))}
            </div>

            {/* Tab content (placeholder) */}
            <div
                style={{
                    background: themeVars.surface,
                    border: `1px solid ${themeVars.border}`,
                    padding: 32,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    fontSize: 13,
                }}
            >
                {activeTab === "metrics" && (
                    <MetricsTab agentId={agent.id} rangeSel={rangeSel} cores={agent.cpu_cores} />
                )}
                {activeTab === "processes" && <ProcessesTab agentId={agent.id} />}
                {activeTab === "services" && <ServicesTab agentId={agent.id} />}
                {activeTab === "apps" && <ApplicationsTab agentId={agent.id} />}
                {activeTab === "updates" && <UpdatesTab agentId={agent.id} />}
            </div>
        </div>
    );
}