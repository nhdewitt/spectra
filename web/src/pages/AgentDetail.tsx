import { useState } from "react";
import { theme } from "../theme";
import { statusColor } from "../utils";
import type { OverviewAgent, TimeRange } from "../types";

const TIME_RANGES: TimeRange[] = ["5m", "15m", "1h", "6h", "24h", "7d", "30d"];
const TABS = ["metrics", "processes", "services", "apps", "updates"] as const;

export function AgentDetail({
    agent,
    onBack,
}: {
    agent: OverviewAgent;
    onBack: () => void;
}) {
    const [timeRange, setTimeRange] = useState<TimeRange>("1h");
    const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>("metrics");

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
                        fontFamily: theme.font,
                        color: theme.textMuted,
                        background: "transparent",
                        border: `1px solid ${theme.border}`,
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
                        background: statusColor(agent),
                    }}
                />
                <div>
                    <div
                        style={{
                            fontFamily: theme.font,
                            fontSize: 18,
                            fontWeight: 600,
                            color: theme.text,
                        }}
                    >
                        {agent.hostname}
                    </div>
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: theme.font,
                            color: theme.textMuted,
                            marginTop: 2,
                        }}
                    >
                        {agent.os} · {agent.platform} · {agent.arch} · {agent.cpu_cores}{" "}
                        {agent.cpu_cores === 1 ? "core" : "cores"}
                    </div>
                </div>
            </div>

            {/* Time range selector */}
            <div style={{ display: "flex", gap: 4, marginBottom: 20 }}>
                {TIME_RANGES.map((r) => (
                    <button
                        key={r}
                        onClick={() => setTimeRange(r)}
                        style={{
                            padding: "5px 10px",
                            fontSize: 11,
                            fontFamily: theme.font,
                            color: timeRange === r ? theme.text : theme.textMuted,
                            background: timeRange === r ? theme.accentDim : "transparent",
                            border: `1px solid ${timeRange === r ? theme.accent : theme.border}`,
                            cursor: "pointer",
                        }}
                    >
                        {r}
                    </button>
                ))}
            </div>

            {/* Tab bar */}
            <div
                style={{
                    display: "flex",
                    gap: 4,
                    marginBottom: 20,
                    borderBottom: `1px solid ${theme.border}`,
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
                            fontFamily: theme.font,
                            color: activeTab === tab ? theme.text : theme.textMuted,
                            background: activeTab === tab ? theme.accentDim : "transparent",
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
                    background: theme.surface,
                    border: `1px solid ${theme.border}`,
                    padding: 32,
                    textAlign: "center",
                    fontFamily: theme.font,
                    color: theme.textDim,
                    fontSize: 13,
                }}
            >
                {activeTab === "metrics" && (
                    <div>
                        Chart panels for CPU, Memory, Disk, Network, Temperature will render here.
                        <br />
                        <span style={{ color: theme.textMuted }}>
                            Time range: {timeRange} · Agent: {agent.id}
                        </span>
                    </div>
                )}
                {activeTab === "processes" && "Process table will render here."}
                {activeTab === "services" && "Services table will render here."}
                {activeTab === "apps" && "Applications list will render here."}
                {activeTab === "updates" && "Pending updates will render here."}
            </div>
        </div>
    );
}