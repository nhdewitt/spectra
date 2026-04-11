import { useState } from "react";
import { themeVars } from "../theme";
import { formatBytes, formatUptime, statusColor } from "../utils";
import { StatBlock, TimeRangePicker } from "../components";
import { MetricsTab } from "../components/MetricsTab";
import type { OverviewAgent, RangeSelection } from "../types";
import { api } from "../api";
import { usePolling } from "../hooks/usePolling";
import { ProcessesTab } from "../components/ProcessesTab";
import { ServicesTab } from "../components/ServicesTab";
import { ApplicationsTab } from "../components/ApplicationsTab";
import { UpdatesTab } from "../components/UpdatesTab";
import { ContainersTab } from "../components/ContainersTab";
import { DiagnosticsPanel } from "../components/DiagnosticsPanel";

const TABS = ["metrics", "processes", "services", "containers", "apps", "updates", "diagnostics"] as const;

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
    const { data: systemInfo } = usePolling(
        () => api.agentSystemLatest(agent.id),
        30_000
    );
    const lastSeen = liveAgent?.last_seen ?? agent.last_seen;
    
    const isTimeSeriesTab = activeTab === "metrics" || activeTab === "containers";

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
                            alignItems: "center",
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
                        {liveAgent?.ip_address && ` · ${liveAgent.ip_address}`}
                        {agent.version && ` · ${agent.version}`}
                    </div>
                </div>
            </div>

            {/* System info stats row */}
            <div
                style={{
                    display: "flex",
                    gap: 24,
                    marginBottom: 20,
                    flexWrap: "wrap",
                }}
            >
                {systemInfo && (
                    <>
                        <StatBlock label="Uptime" value={formatUptime(systemInfo.uptime)} />
                        <StatBlock
                            label="Boot"
                            value={systemInfo.boot_time
                                ? new Date(Number(systemInfo.boot_time) * 1000).toLocaleDateString(undefined, {
                                    month: "short",
                                    day: "numeric",
                                    hour: "2-digit",
                                    minute: "2-digit",
                                })
                                : null}
                        />
                        <StatBlock label="Users" value={String(systemInfo.user_count)} />
                        <StatBlock label="Processes" value={String(systemInfo.process_count)} />
                    </>
                )}
                {liveAgent && (
                    <>
                        <StatBlock label="CPU" value={liveAgent.cpu_model ?? null} />
                        <StatBlock label="RAM" value={formatBytes(liveAgent.ram_total)} />
                    </>
                )}
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

            {/* Tab content */}
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
                {activeTab === "containers" && <ContainersTab agentId={agent.id} rangeSel={rangeSel} />}
                {activeTab === "apps" && <ApplicationsTab agentId={agent.id} />}
                {activeTab === "updates" && <UpdatesTab agentId={agent.id} />}
                {activeTab === "diagnostics" && <DiagnosticsPanel agentId={agent.id} />}
            </div>
        </div>
    );
}