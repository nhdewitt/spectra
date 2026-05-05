import { useState, useEffect, useCallback } from "react";
import { themeVars } from "../theme";
import { formatBytes, formatUptime, agentStatus, agentStatusColor } from "../utils";
import { StatBlock, TimeRangePicker } from "../components";
import { MetricsTab } from "../components/MetricsTab";
import { OSIcon } from "../icons";
import type { OverviewAgent, RangeSelection } from "../types";
import { api } from "../api";
import { usePolling } from "../hooks/usePolling";
import { ProcessesTab } from "../components/ProcessesTab";
import { ServicesTab } from "../components/ServicesTab";
import { ApplicationsTab } from "../components/ApplicationsTab";
import { UpdatesTab } from "../components/UpdatesTab";
import { ContainersTab } from "../components/ContainersTab";

const TABS = ["metrics", "processes", "services", "containers", "apps", "updates"] as const;

interface AgentDetailProps {
    agent: OverviewAgent;
    agents: OverviewAgent[];
    onSelectAgent: (agent: OverviewAgent) => void;
    onBack: () => void;
    starredIds: string[];
    onToggleStar: (agentId: string) => void;
}

export function AgentDetail({
    agent,
    agents,
    onSelectAgent,
    onBack,
    starredIds,
    onToggleStar,
}: AgentDetailProps) {
    const [rangeSel, setRangeSel] = useState<RangeSelection>({ type: "quick", range: "1h" });
    const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>("metrics");
    const [dropdownOpen, setDropdownOpen] = useState(false);

    const { data: liveAgent } = usePolling(useCallback(() => api.agent(agent.id), [agent.id]), 30_000);
    const { data: systemInfo } = usePolling(useCallback(() => api.agentSystemLatest(agent.id), [agent.id]), 30_000);
    
    const lastSeen = liveAgent?.last_seen ?? agent.last_seen;
    const { status } = agentStatus(agent);
    const isStarred = starredIds.includes(agent.id);
    const isTimeSeriesTab = activeTab === "metrics" || activeTab === "containers";

    // Close dropdown on escape
    useEffect(() => {
        if (!dropdownOpen) return;
        const handler = (e: KeyboardEvent) => {
            if (e.key === "Escape") setDropdownOpen(false);
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [dropdownOpen]);

    return (
        <div style={{ padding: 24 }}>
            {/* Header */}
            <div
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 12,
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

                <OSIcon os={agent.os} platform={agent.platform} size={18} />

                <div
                    style={{
                        width: 10,
                        height: 10,
                        borderRadius: "50%",
                        background: agentStatusColor(status),
                        flexShrink: 0,
                    }}
                />

                {/* Hostname + dropdown */}
                <div style={{ position: "relative" }}>
                    <button
                        onClick={() => setDropdownOpen((v) => !v)}
                        style={{
                            display: "flex",
                            alignItems: "center",
                            gap: 6,
                            fontFamily: themeVars.font,
                            fontSize: 18,
                            fontWeight: 600,
                            color: themeVars.text,
                            background: "transparent",
                            border: "none",
                            cursor: "pointer",
                            padding: 0,
                        }}
                    >
                        {agent.hostname}
                        <span style={{ fontSize: 10, color: themeVars.textDim }}>▾</span>
                    </button>

                    {dropdownOpen && (
                        <>
                            <div
                                style={{
                                    position: "fixed",
                                    top: 0,
                                    left: 0,
                                    right: 0,
                                    bottom: 0,
                                    zIndex: 99,
                                }}
                                onClick={() => setDropdownOpen(false)}
                            />
                            <div
                                style={{
                                    position: "absolute",
                                    top: "calc(100% + 4px)",
                                    left: 0,
                                    background: themeVars.surface,
                                    border: `1px solid ${themeVars.border}`,
                                    zIndex: 100,
                                    minWidth: 220,
                                    maxHeight: 300,
                                    overflowY: "auto",
                                }}
                            >
                                {agents
                                    .slice()
                                    .sort((a, b) => a.hostname.localeCompare(b.hostname))
                                    .map((a) => (
                                        <button
                                            key={a.id}
                                            onClick={() => {
                                                onSelectAgent(a);
                                                setDropdownOpen(false);
                                            }}
                                            style={{
                                                display: "flex",
                                                alignItems: "center",
                                                gap: 8,
                                                width: "100%",
                                                padding: "8px 12px",
                                                fontSize: 12,
                                                fontFamily: themeVars.font,
                                                color: a.id === agent.id ? themeVars.text : themeVars.textMuted,
                                                background: a.id === agent.id ? themeVars.accentDim : "transparent",
                                                border: "none",
                                                cursor: "pointer",
                                                textAlign: "left",
                                            }}
                                        >
                                            <span
                                                style={{
                                                    width: 6,
                                                    height: 6,
                                                    borderRadius: "50%",
                                                    background: agentStatusColor(agentStatus(a).status),
                                                    flexShrink: 0,
                                                }}
                                            />
                                            <OSIcon os={a.os} platform={a.platform} size={12} />
                                            <span style={{ flex: 1 }}>{a.hostname}</span>
                                            <span style={{ fontSize: 10, color: themeVars.textDim }}>
                                                {a.platform}
                                            </span>
                                        </button>
                                    ))}
                            </div>
                        </>
                    )}
                </div>

                {/* Status badge */}
                <span
                    style={{
                        fontSize: 9,
                        fontFamily: themeVars.font,
                        fontWeight: 600,
                        color: agentStatusColor(status),
                        background: `color-mix(in srgb, ${agentStatusColor(status)} 15%, transparent)`,
                        border: `1px solid ${agentStatusColor(status)}`,
                        padding: "2px 6px",
                        letterSpacing: "0.04em",
                    }}
                >
                    {status.toUpperCase()}
                </span>

                {/* Reboot badge */}
                {agent.reboot_required && (
                    <span
                        style={{
                            fontSize: 9,
                            fontFamily: themeVars.font,
                            color: themeVars.warn,
                            background: `color-mix(in srgb, ${themeVars.warn} 15%, transparent)`,
                            border: `1px solid ${themeVars.warn}`,
                            padding: "2px 6px",
                            letterSpacing: "0.04em",
                            fontWeight: 600,
                        }}
                    >
                        REBOOT
                    </span>
                )}

                {/* Star toggle */}
                <button
                    onClick={() => onToggleStar(agent.id)}
                    title={isStarred ? "Remove from quick access" : "Add to quick access"}
                    style={{
                        background: "none",
                        border: "none",
                        cursor: "pointer",
                        fontSize: 16,
                        color: isStarred ? themeVars.warn : themeVars.textDim,
                        padding: "0 4px",
                        transition: "color 0.15s ease",
                    }}
                >
                    {isStarred ? "★" : "☆"}
                </button>
            </div>

            {/* Agent info row */}
            <div
                style={{
                    fontSize: 12,
                    fontFamily: themeVars.font,
                    color: themeVars.textMuted,
                    marginBottom: 20,
                    display: "flex",
                    gap: 16,
                    flexWrap: "wrap",
                }}
            >
                <span>{agent.os}</span>
                <span>{agent.platform}</span>
                <span>{agent.arch}</span>
                <span>{agent.cpu_cores} {agent.cpu_cores === 1 ? "core" : "cores"}</span>
                {liveAgent?.ip_address && <span>{liveAgent.ip_address}</span>}
                {agent.version && <span>{agent.version}</span>}
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
            </div>
        </div>
    );
}