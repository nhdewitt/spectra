import React, { useState } from "react";
import { themeVars } from "../theme";
import { OSIcon } from "../icons";
import { useAgentSearch } from "../hooks/useAgentSearch";
import { useThresholds } from "../ThresholdsContext";
import { agentStatus, agentStatusColor } from "../utils";
import type { OverviewAgent } from "../types";

interface AgentSearchListProps {
	onSelectAgent: (agent: OverviewAgent) => void;
	currentAgentId?: string;
	autoFocus?: boolean;
	placeholder?: string;
	maxHeight?: number;
}

/**
 * Search input + scrollable results list, backed by useAgentSearch.
 * Shared by AgentDetail's switcher, Diagnostics' picker, and App.tsx's
 * inline "select and agent" fallback - each wraps this in whatever
 * trigger/positioning chrome fits its own context.
 */
export function AgentSearchList({
	onSelectAgent,
	currentAgentId,
	autoFocus = true,
	placeholder = "Search agents...",
	maxHeight = 300,
}: AgentSearchListProps) {
	const [query, setQuery] = useState("");
	const { agents, loading, error } = useAgentSearch(query);
	const thresholds = useThresholds();

	const emptyStyle: React.CSSProperties = {
		padding: "8px 12px",
		fontSize: 11,
		fontFamily: themeVars.font,
		color: themeVars.textDim,
	};

    return (
        <div>
            <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={placeholder}
                autoFocus={autoFocus}
                style={{
                    width: "100%",
                    padding: "6px 10px",
                    fontSize: 12,
                    fontFamily: themeVars.font,
                    color: themeVars.text,
                    background: themeVars.surface,
                    border: `1px solid ${themeVars.border}`,
                    boxSizing: "border-box",
                }}
            />
            <div style={{ maxHeight, overflowY: "auto", marginTop: 4 }}>
                {loading && agents.length === 0 && <div style={emptyStyle}>Searching…</div>}
                {error && <div style={{ ...emptyStyle, color: themeVars.danger }}>{error}</div>}
                {!loading && !error && agents.length === 0 && <div style={emptyStyle}>No agents match.</div>}
                {agents.map((a) => (
                    <button
                        key={a.id}
                        onClick={() => onSelectAgent(a)}
                        style={{
                            display: "flex",
                            alignItems: "center",
                            gap: 8,
                            width: "100%",
                            padding: "8px 12px",
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            color: a.id === currentAgentId ? themeVars.text : themeVars.textMuted,
                            background: a.id === currentAgentId ? themeVars.accentDim : "transparent",
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
                                background: agentStatusColor(agentStatus(a, thresholds).status),
                                flexShrink: 0,
                            }}
                        />
                        <OSIcon os={a.os} platform={a.platform} size={12} />
                        <span style={{ flex: 1 }}>{a.hostname}</span>
                        <span style={{ fontSize: 10, color: themeVars.textDim }}>{a.platform}</span>
                    </button>
                ))}
            </div>
        </div>
    );
}