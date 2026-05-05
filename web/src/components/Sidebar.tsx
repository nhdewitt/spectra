import { useState, useEffect, useCallback } from "react";
import { themeVars } from "../theme";
import { SpectraLogo } from "./SpectraLogo";
import { statusColor } from "../utils";
import type { Page, OverviewAgent, User } from "../types";
import { api } from "../api";

interface SidebarProps {
    user: User;
    currentPage: Page;
    onNavigate: (page: Page) => void;
    selectedAgent: OverviewAgent | null;
    onSelectAgent: (agent: OverviewAgent) => void;
    agents: OverviewAgent[];
    onlineCount: number;
    totalCount: number;
    starredIds: string[];
}

const NAV_ICON: Record<string, string> = {
  overview: "■",
  detail: "◆",
  diagnostics: "○",
  agents: "☐",
  users: "☐",
};

interface NavItem {
    key: Page;
    label: string;
    indent?: boolean;
    adminOnly?: boolean;
}

export function Sidebar({
    user,
    currentPage,
    onNavigate,
    selectedAgent,
    onSelectAgent,
    agents,
    onlineCount,
    totalCount,
    starredIds,
}: SidebarProps) {
    const [detailExpanded, setDetailExpanded] = useState(currentPage === "detail" || currentPage === "diagnostics");

    // Expand detail section when navigating to detail or diagnostics
    useEffect(() => {
        if (currentPage === "detail" || currentPage === "diagnostics") {
            setDetailExpanded(true);
        }
    }, [currentPage]);

    const starredAgents = agents
        .filter((a) => starredIds.includes(a.id))
        .sort((a, b) => a.hostname.localeCompare(b.hostname));

    const navItems: NavItem[] = [
        { key: "overview", label: "Fleet Overview" },
        { key: "detail", label: "Agent Detail" },
        { key: "diagnostics", label: "Diagnostics", indent: true },
        { key: "agents", label: "Agent Mgmt" },
        { key: "users", label: "User Mgmt", adminOnly: true },
    ];

    const isAdmin = user.role === "admin" || user.role === "superadmin";

    return (
        <div
        style={{
            width: 170,
            minHeight: "100vh",
            background: themeVars.surface,
            borderRight: `1px solid ${themeVars.border}`,
            display: "flex",
            flexDirection: "column",
            fontFamily: themeVars.font,
            flexShrink: 0,
        }}
        >
        {/* Logo */}
        <div
            style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            padding: "16px 16px 20px",
            cursor: "pointer",
            }}
            onClick={() => onNavigate("overview")}
        >
            <SpectraLogo size={24} />
            <span
            style={{
                fontSize: 14,
                fontWeight: 600,
                color: themeVars.text,
                letterSpacing: "-0.02em",
            }}
            >
            Spectra
            </span>
        </div>

        {/* Navigation */}
        <div style={{ padding: "0 12px", marginBottom: 24 }}>
            <div
            style={{
                fontSize: 9,
                color: themeVars.textDim,
                letterSpacing: "0.08em",
                textTransform: "uppercase",
                marginBottom: 8,
                padding: "0 4px",
            }}
            >
            Navigation
            </div>

            {navItems.map((item) => {
            if (item.adminOnly && !isAdmin) return null;

            // Hide diagnostics when detail is collapsed
            if (item.key === "diagnostics" && !detailExpanded) return null;

            const isActive = currentPage === item.key;

            // Detail item is special — has a collapse toggle
            if (item.key === "detail") {
                return (
                <button
                    key={item.key}
                    onClick={() => {
                    setDetailExpanded((v) => !v);
                    onNavigate("detail");
                    }}
                    style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    width: "100%",
                    padding: "7px 8px",
                    fontSize: 12,
                    color: isActive ? themeVars.accent : themeVars.textMuted,
                    background: isActive ? themeVars.accentDim : "transparent",
                    border: "none",
                    cursor: "pointer",
                    textAlign: "left",
                    fontFamily: themeVars.font,
                    }}
                >
                    <span style={{ fontSize: 10, width: 14, textAlign: "center" }}>
                    {NAV_ICON[item.key]}
                    </span>
                    {item.label}
                </button>
                );
            }

            return (
                <button
                key={item.key}
                onClick={() => onNavigate(item.key)}
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    width: "100%",
                    padding: "7px 8px",
                    paddingLeft: item.indent ? 30 : 8,
                    fontSize: 12,
                    color: isActive ? themeVars.accent : themeVars.textMuted,
                    background: isActive ? themeVars.accentDim : "transparent",
                    border: "none",
                    cursor: "pointer",
                    textAlign: "left",
                    fontFamily: themeVars.font,
                }}
                >
                <span style={{ fontSize: 10, width: 14, textAlign: "center" }}>
                    {NAV_ICON[item.key]}
                </span>
                {item.label}
                </button>
            );
            })}
        </div>

        {/* Quick Access */}
        {starredAgents.length > 0 && (
            <div style={{ padding: "0 12px", marginBottom: 24 }}>
            <div
                style={{
                fontSize: 9,
                color: themeVars.textDim,
                letterSpacing: "0.08em",
                textTransform: "uppercase",
                marginBottom: 8,
                padding: "0 4px",
                }}
            >
                Quick Access
            </div>

            {starredAgents.map((agent) => (
                <button
                key={agent.id}
                onClick={() => {
                    onSelectAgent(agent);
                    onNavigate("detail");
                }}
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    width: "100%",
                    padding: "5px 8px",
                    fontSize: 11,
                    color:
                    selectedAgent?.id === agent.id
                        ? themeVars.text
                        : themeVars.textMuted,
                    background:
                    selectedAgent?.id === agent.id
                        ? themeVars.accentDim
                        : "transparent",
                    border: "none",
                    cursor: "pointer",
                    textAlign: "left",
                    fontFamily: themeVars.font,
                }}
                >
                <span
                    style={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    background: statusColor(agent),
                    flexShrink: 0,
                    }}
                />
                <span
                    style={{
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    }}
                >
                    {agent.hostname}
                </span>
                </button>
            ))}
            </div>
        )}

        {/* Spacer */}
        <div style={{ flex: 1 }} />

        {/* Footer */}
        <div style={{ padding: "12px 16px", borderTop: `1px solid ${themeVars.border}` }}>
            <div
            style={{
                fontSize: 10,
                color: themeVars.textDim,
                marginBottom: 6,
            }}
            >
            v0.1.0
            </div>
            <button
            onClick={() => onNavigate("settings")}
            style={{
                fontSize: 11,
                color: currentPage === "settings" ? themeVars.accent : themeVars.textMuted,
                background: "none",
                border: "none",
                cursor: "pointer",
                padding: 0,
                fontFamily: themeVars.font,
            }}
            >
            {user.username}
            </button>
        </div>
        </div>
    );
}

export { type SidebarProps };