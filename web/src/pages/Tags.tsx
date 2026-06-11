import { useState, useEffect, useCallback, useMemo } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { LoadingSpinner } from "../components";
import { LabelChip } from "../components/LabelChip";
import type { AgentLabel, LabelKey, OverviewAgent, User } from "../types";

interface TagsProps {
	user: User;
}

interface ValueGroup {
	value: string;
	agentIds: string[];
	agentHostnames: string[];
}

interface KeySummary {
	key: string;
	source: "auto" | "user";
	agentCount: number;
	values: ValueGroup[];
}

const btnStyle: React.CSSProperties = {
	padding: "6px 14px",
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.accentDim,
	border: `1px solid ${themeVars.accent}`,
	cursor: "pointer",
	textTransform: "uppercase",
	letterSpacing: "0.03em",
};

const ghostBtnStyle: React.CSSProperties = {
	...btnStyle,
	color: themeVars.textMuted,
	background: "transparent",
	borderColor: themeVars.border,
};

const inputStyle: React.CSSProperties = {
	padding: "6px 10px",
	fontSize: 12,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.surface,
	border: `1px solid ${themeVars.border}`,
	width: "100%",
	boxSizing: "border-box",
};

const selectStyle: React.CSSProperties = {
    padding: "5px 8px",
    fontSize: 11,
    fontFamily: themeVars.font,
    color: themeVars.text,
    background: themeVars.bg,
    border: `1px solid ${themeVars.border}`,
    cursor: "pointer",
};

const sectionHeaderStyle: React.CSSProperties = {
	fontSize: 12,
	fontFamily: themeVars.font,
	color: themeVars.textMuted,
	textTransform: "uppercase",
	letterSpacing: "0.04em",
	marginBottom: 12,
};

const fieldLabelStyle: React.CSSProperties = {
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.textMuted,
	textTransform: "uppercase",
	letterSpacing: "0.03em",
	marginBottom: 4,
	display: "block",
};

// --- Apply Tag Panel ---

function ApplyTagPanel({
	agents,
	knownKeys,
    labelsByAgent,
	onApplied,
}: {
	agents: OverviewAgent[];
	knownKeys: LabelKey[];
    labelsByAgent: Map<string, AgentLabel[]>;
	onApplied: (agentIds: string[]) => void;
}) {
	const [keyInput, setKeyInput] = useState("");
	const [valueInput, setValueInput] = useState("");
	const [availableValues, setAvailableValues] = useState<string[]>([]);
	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

	const [agentSearch, setAgentSearch] = useState("");
    const [osFilter, setOsFilter] = useState("all");
    const [archFilter, setArchFilter] = useState("all");
    const [hwFilter, setHwFilter] = useState("all");
    const [labelKeyFilter, setLabelKeyFilter] = useState("any");
    const [labelValueFilter, setLabelValueFilter] = useState("any");

	const [applying, setApplying] = useState(false);
	const [result, setResult] = useState<string | null>(null);
	const [error, setError] = useState<string | null>(null);

	const userKeys = useMemo(() => knownKeys.filter((k) => k.source === "user"), [knownKeys]);

    useEffect(() => {
        const k = keyInput.trim();
        if (!k || !userKeys.some((uk) => uk.key === k)) {
            setAvailableValues([]);
            return;
        }
        const timer = setTimeout(() => {
            api.labelValues(k).then(setAvailableValues).catch(() => setAvailableValues([]));
        }, 150);
        return () => clearTimeout(timer);
    }, [keyInput, userKeys]);

    const { osOptions, archOptions } = useMemo(() => {
        const os = new Set<string>();
        const arch = new Set<string>();

        for (const a of agents) {
            if (a.os) os.add(a.os);
            if (a.arch) arch.add(a.arch);
        }

        return {
            osOptions: Array.from(os).sort(),
            archOptions: Array.from(arch).sort(),
        };
    }, [agents]);

    const hwOptions = useMemo(() => {
        const hw = new Set<string>();

        for (const labels of labelsByAgent.values()) {
            for (const l of labels) {
                if (l.key === "hardware") hw.add(l.value);
            }
        }

        return Array.from(hw).sort();
    }, [labelsByAgent]);

    const labelFilterValues = useMemo(() => {
        if (labelKeyFilter === "any") return [];
        const s = new Set<string>();
        for (const labels of labelsByAgent.values()) {
            for (const l of labels) {
                if (l.key === labelKeyFilter) s.add(l.value);
            }
        }
        return Array.from(s).sort();
    }, [labelKeyFilter, labelsByAgent]);

    // Reset the value filter when the key filter changes.
    useEffect(() => {
        setLabelValueFilter("any");
    }, [labelKeyFilter]);
 
    const filteredAgents = useMemo(() => {
        let result = agents;

        if (agentSearch) {
            const q = agentSearch.toLowerCase();
            result = result.filter((a) => a.hostname.toLowerCase().includes(q));
        }
        if (osFilter !== "all") {
            result = result.filter((a) => a.os === osFilter);
        }
        if (archFilter !== "all") {
            result = result.filter((a) => a.arch === archFilter);
        }
        if (hwFilter !== "all") {
            result = result.filter((a) => {
                const labels = labelsByAgent.get(a.id) ?? [];
                return labels.some((l) => l.key === "hardware" && l.value === hwFilter);
            });
        }
        if (labelKeyFilter !== "any") {
            result = result.filter((a) => {
                const labels = labelsByAgent.get(a.id) ?? [];
                if (labelValueFilter === "any") {
                    return labels.some((l) => l.key === labelKeyFilter);
                }
                return labels.some((l) => l.key === labelKeyFilter && l.value === labelValueFilter);
            });
        }

        return result;
    }, [
        agents,
        agentSearch,
        osFilter,
        archFilter,
        hwFilter,
        labelKeyFilter,
        labelValueFilter,
        labelsByAgent,
    ]);
 
    const toggleAgent = useCallback((agentId: string) => {
        setSelectedIds((prev) => {
            const next = new Set(prev);
            if (next.has(agentId)) next.delete(agentId);
            else next.add(agentId);
            return next;
        });
    }, []);
 
    const selectAllFiltered = useCallback(() => {
        setSelectedIds((prev) => {
            const visible = filteredAgents.map((a) => a.id);
            const allSelected =
                visible.length > 0 &&
                visible.every((id) => prev.has(id)) &&
                prev.size === visible.length;
            if (allSelected) return new Set();
            return new Set(visible);
        });
    }, [filteredAgents]);
 
    const handleApply = useCallback(async () => {
        const k = keyInput.trim();
        const v = valueInput.trim();
        if (!k || !v || selectedIds.size === 0) return;
 
        setApplying(true);
        setResult(null);
        setError(null);
 
        const ids = Array.from(selectedIds);
        const results = await Promise.allSettled(
            ids.map((id) => api.setAgentLabel(id, k, v))
        );
 
        let succeeded = 0;
        let failed = 0;
        const firstError = results.find((r) => r.status === "rejected") as
            | PromiseRejectedResult
            | undefined;
        for (const r of results) {
            if (r.status === "fulfilled") succeeded++;
            else failed++;
        }
 
        setApplying(false);
 
        if (failed === ids.length && firstError) {
            // Total failure — show server error (likely reserved key, validation).
            setError(
                firstError.reason instanceof Error
                    ? firstError.reason.message
                    : "Failed to apply tag"
            );
        } else if (failed > 0) {
            setResult(`${succeeded} tagged, ${failed} failed`);
            onApplied(ids);
        } else {
            setResult(`${succeeded} agent${succeeded !== 1 ? "s" : ""} tagged with ${k}=${v}`);
            onApplied(ids);
            setKeyInput("");
            setValueInput("");
            setSelectedIds(new Set());
            setTimeout(() => setResult(null), 4000);
        }
    }, [keyInput, valueInput, selectedIds, onApplied]);
 
    const handleClear = useCallback(() => {
        setKeyInput("");
        setValueInput("");
        setSelectedIds(new Set());
        setAgentSearch("");
        setOsFilter("all");
        setArchFilter("all");
        setHwFilter("all");
        setLabelKeyFilter("any");
        setLabelValueFilter("any");
        setError(null);
        setResult(null);
    }, []);
 
    const canApply =
        !applying &&
        keyInput.trim().length > 0 &&
        valueInput.trim().length > 0 &&
        selectedIds.size > 0;
 
    const allVisibleSelected =
        filteredAgents.length > 0 &&
        filteredAgents.every((a) => selectedIds.has(a.id)) &&
        selectedIds.size === filteredAgents.length;
 
    return (
        <div
            style={{
                background: themeVars.surface,
                border: `1px solid ${themeVars.border}`,
                padding: 20,
                marginBottom: 20,
            }}
        >
            <div style={sectionHeaderStyle}>Apply a tag</div>
 
            {/* Key + Value row */}
            <div
                style={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr",
                    gap: 12,
                    marginBottom: 16,
                }}
            >
                <div>
                    <label style={fieldLabelStyle}>Key</label>
                    <input
                        type="text"
                        list="tags-key-list"
                        value={keyInput}
                        onChange={(e) => {
                            setKeyInput(e.target.value);
                            setError(null);
                            setResult(null);
                        }}
                        placeholder="e.g. env"
                        style={inputStyle}
                    />
                    <datalist id="tags-key-list">
                        {userKeys.map((k) => (
                            <option key={k.key} value={k.key} />
                        ))}
                    </datalist>
                </div>
                <div>
                    <label style={fieldLabelStyle}>Value</label>
                    <input
                        type="text"
                        list="tags-value-list"
                        value={valueInput}
                        onChange={(e) => {
                            setValueInput(e.target.value);
                            setError(null);
                            setResult(null);
                        }}
                        onKeyDown={(e) => {
                            if (e.key === "Enter" && canApply) handleApply();
                        }}
                        placeholder="e.g. prod"
                        style={inputStyle}
                    />
                    <datalist id="tags-value-list">
                        {availableValues.map((v) => (
                            <option key={v} value={v} />
                        ))}
                    </datalist>
                </div>
            </div>
 
            {/* Agent picker */}
            <div style={{ marginBottom: 12 }}>
                <div
                    style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "center",
                        marginBottom: 4,
                    }}
                >
                    <label style={fieldLabelStyle}>Apply to agents</label>
                    <span
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.accent,
                            fontWeight: 500,
                        }}
                    >
                        {selectedIds.size} selected
                    </span>
                </div>
 
                {/* Picker filter row */}
                <div
                    style={{
                        display: "flex",
                        gap: 6,
                        marginBottom: 8,
                        flexWrap: "wrap",
                        alignItems: "center",
                    }}
                >
                    <input
                        type="text"
                        value={agentSearch}
                        onChange={(e) => setAgentSearch(e.target.value)}
                        placeholder="Search agents..."
                        style={{ ...inputStyle, width: 200, flexShrink: 0 }}
                    />
                    <select
                        value={osFilter}
                        onChange={(e) => setOsFilter(e.target.value)}
                        style={selectStyle}
                    >
                        <option value="all">All OS</option>
                        {osOptions.map((o) => (
                            <option key={o} value={o}>{o}</option>
                        ))}
                    </select>
                    <select
                        value={archFilter}
                        onChange={(e) => setArchFilter(e.target.value)}
                        style={selectStyle}
                    >
                        <option value="all">All Arch</option>
                        {archOptions.map((a) => (
                            <option key={a} value={a}>{a}</option>
                        ))}
                    </select>
                    {hwOptions.length > 0 && (
                        <select
                            value={hwFilter}
                            onChange={(e) => setHwFilter(e.target.value)}
                            style={selectStyle}
                        >
                            <option value="all">All Hardware</option>
                            {hwOptions.map((h) => (
                                <option key={h} value={h}>{h}</option>
                            ))}
                        </select>
                    )}
                </div>
 
                {/* Existing-label filter row */}
                {userKeys.length > 0 && (
                    <div
                        style={{
                            display: "flex",
                            gap: 6,
                            marginBottom: 8,
                            alignItems: "center",
                            flexWrap: "wrap",
                        }}
                    >
                        <span
                            style={{
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: themeVars.textMuted,
                                textTransform: "uppercase",
                                letterSpacing: "0.03em",
                            }}
                        >
                            Has label:
                        </span>
                        <select
                            value={labelKeyFilter}
                            onChange={(e) => setLabelKeyFilter(e.target.value)}
                            style={selectStyle}
                        >
                            <option value="any">(any)</option>
                            {userKeys.map((k) => (
                                <option key={k.key} value={k.key}>{k.key}</option>
                            ))}
                        </select>
                        {labelKeyFilter !== "any" && (
                            <>
                                <span style={{ color: themeVars.textDim, fontSize: 12 }}>=</span>
                                <select
                                    value={labelValueFilter}
                                    onChange={(e) => setLabelValueFilter(e.target.value)}
                                    style={selectStyle}
                                >
                                    <option value="any">(any value)</option>
                                    {labelFilterValues.map((v) => (
                                        <option key={v} value={v}>{v}</option>
                                    ))}
                                </select>
                            </>
                        )}
                    </div>
                )}
 
                {/* Agent list */}
                <div
                    style={{
                        border: `1px solid ${themeVars.border}`,
                        background: themeVars.bg,
                        maxHeight: 220,
                        overflowY: "auto",
                    }}
                >
                    {filteredAgents.length === 0 ? (
                        <div
                            style={{
                                padding: 12,
                                fontSize: 12,
                                fontFamily: themeVars.font,
                                color: themeVars.textMuted,
                                textAlign: "center",
                            }}
                        >
                            No agents match the current filters.
                        </div>
                    ) : (
                        filteredAgents.map((a) => {
                            const isChecked = selectedIds.has(a.id);
                            return (
                                <label
                                    key={a.id}
                                    style={{
                                        display: "flex",
                                        alignItems: "center",
                                        gap: 8,
                                        padding: "6px 10px",
                                        borderBottom: `1px solid ${themeVars.border}`,
                                        fontSize: 12,
                                        fontFamily: themeVars.font,
                                        color: themeVars.text,
                                        cursor: "pointer",
                                        background: isChecked
                                            ? themeVars.accentDim
                                            : "transparent",
                                    }}
                                >
                                    <input
                                        type="checkbox"
                                        checked={isChecked}
                                        onChange={() => toggleAgent(a.id)}
                                        style={{ cursor: "pointer" }}
                                    />
                                    <span style={{ fontWeight: 500, flex: 1 }}>
                                        {a.hostname}
                                    </span>
                                    <span
                                        style={{
                                            fontSize: 11,
                                            color: themeVars.textMuted,
                                        }}
                                    >
                                        {a.platform} · {a.arch}
                                    </span>
                                </label>
                            );
                        })
                    )}
                </div>
 
                {filteredAgents.length > 0 && (
                    <div
                        style={{
                            marginTop: 6,
                            display: "flex",
                            justifyContent: "space-between",
                            alignItems: "center",
                        }}
                    >
                        <button
                            onClick={selectAllFiltered}
                            style={{
                                ...ghostBtnStyle,
                                padding: "3px 10px",
                                fontSize: 10,
                            }}
                        >
                            {allVisibleSelected ? "Deselect All Visible" : "Select All Visible"}
                        </button>
                        <span
                            style={{
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: themeVars.textMuted,
                            }}
                        >
                            {filteredAgents.length} of {agents.length} shown
                        </span>
                    </div>
                )}
            </div>
 
            {/* Result / error */}
            {error && (
                <div
                    style={{
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.danger,
                        marginBottom: 8,
                    }}
                >
                    {error}
                </div>
            )}
            {result && !error && (
                <div
                    style={{
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: result.includes("failed") ? themeVars.warn : themeVars.ok,
                        marginBottom: 8,
                    }}
                >
                    {result}
                </div>
            )}
 
            {/* Actions */}
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <button
                    onClick={handleApply}
                    disabled={!canApply}
                    style={{
                        ...btnStyle,
                        opacity: canApply ? 1 : 0.4,
                        cursor: canApply ? "pointer" : "default",
                    }}
                >
                    {applying
                        ? "Applying..."
                        : selectedIds.size > 0
                            ? `Apply to ${selectedIds.size} Agent${selectedIds.size !== 1 ? "s" : ""}`
                            : "Apply"}
                </button>
                <button onClick={handleClear} style={ghostBtnStyle}>
                    Clear
                </button>
                <span
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textMuted,
                        marginLeft: "auto",
                    }}
                >
                    Auto-source keys (os, arch, hardware, agent_version) are reserved.
                </span>
            </div>
        </div>
    );
}

// -- All Tags Panel ---

function ValueRow({
    keyName,
    value,
    agentHostnames,
    agentIds,
    canDelete,
    onDelete,
}: {
    keyName: string;
    value: string;
    agentHostnames: string[];
    agentIds: string[];
    canDelete: boolean;
    onDelete: () => Promise<void>;
}) {
    const [confirming, setConfirming] = useState(false);
    const [deleting, setDeleting] = useState(false);
 
    const synthetic: AgentLabel = {
        key: keyName,
        value,
        source: "user",
        updated_at: "",
    };
 
    const hostList =
        agentHostnames.length <= 6
            ? agentHostnames.join(", ")
            : `${agentHostnames.slice(0, 6).join(", ")}, +${agentHostnames.length - 6} more`;
 
    const handleConfirm = async () => {
        setDeleting(true);
        try {
            await onDelete();
        } finally {
            setDeleting(false);
            setConfirming(false);
        }
    };
 
    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                padding: "6px 0",
                fontSize: 12,
                fontFamily: themeVars.font,
            }}
        >
            <LabelChip label={synthetic} />
            <span
                style={{
                    color: themeVars.textDim,
                    flex: 1,
                    minWidth: 0,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                }}
                title={agentHostnames.join(", ")}
            >
                {agentIds.length} agent{agentIds.length !== 1 ? "s" : ""} — {hostList}
            </span>
            {canDelete && !confirming && (
                <button
                    onClick={() => setConfirming(true)}
                    title={`Remove ${keyName}=${value} from all agents`}
                    style={{
                        ...ghostBtnStyle,
                        padding: "3px 8px",
                        fontSize: 10,
                        color: themeVars.danger,
                        borderColor: themeVars.border,
                    }}
                >
                    Remove
                </button>
            )}
            {canDelete && confirming && (
                <>
                    <span
                        style={{
                            fontSize: 10,
                            color: themeVars.danger,
                            fontFamily: themeVars.font,
                        }}
                    >
                        Remove from {agentIds.length} agent{agentIds.length !== 1 ? "s" : ""}?
                    </span>
                    <button
                        onClick={handleConfirm}
                        disabled={deleting}
                        style={{
                            ...btnStyle,
                            padding: "3px 10px",
                            fontSize: 10,
                            color: "#fff",
                            background: themeVars.danger,
                            borderColor: themeVars.danger,
                            opacity: deleting ? 0.6 : 1,
                        }}
                    >
                        {deleting ? "..." : "Confirm"}
                    </button>
                    <button
                        onClick={() => setConfirming(false)}
                        disabled={deleting}
                        style={{
                            ...ghostBtnStyle,
                            padding: "3px 10px",
                            fontSize: 10,
                        }}
                    >
                        Cancel
                    </button>
                </>
            )}
        </div>
    );
}
 
function KeyRow({
    summary,
    expanded,
    onToggleExpand,
    onDeleteValue,
}: {
    summary: KeySummary;
    expanded: boolean;
    onToggleExpand: () => void;
    onDeleteValue: (value: string, agentIds: string[]) => Promise<void>;
}) {
    const isAuto = summary.source === "auto";
 
    return (
        <div
            style={{
                borderBottom: `1px solid ${themeVars.border}`,
            }}
        >
            <div
                onClick={onToggleExpand}
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    padding: "10px 0",
                    cursor: "pointer",
                    fontFamily: themeVars.font,
                }}
            >
                <span
                    style={{
                        fontSize: 10,
                        color: themeVars.textDim,
                        width: 12,
                        display: "inline-block",
                    }}
                >
                    {expanded ? "▾" : "▸"}
                </span>
                <span
                    style={{
                        fontSize: 13,
                        fontWeight: 500,
                        color: isAuto ? themeVars.textDim : themeVars.text,
                    }}
                >
                    {summary.key}
                </span>
                <span
                    style={{
                        fontSize: 10,
                        color: themeVars.textDim,
                        textTransform: "uppercase",
                        letterSpacing: "0.04em",
                    }}
                >
                    {summary.source} · {summary.agentCount} agent
                    {summary.agentCount !== 1 ? "s" : ""} · {summary.values.length} value
                    {summary.values.length !== 1 ? "s" : ""}
                </span>
                {isAuto && (
                    <span
                        style={{
                            marginLeft: "auto",
                            fontSize: 10,
                            color: themeVars.textDim,
                            fontFamily: themeVars.font,
                        }}
                        title="Auto labels are read-only"
                    >
                        read-only
                    </span>
                )}
            </div>
 
            {expanded && (
                <div style={{ paddingLeft: 20, paddingBottom: 6 }}>
                    {summary.values.map((v) => (
                        <ValueRow
                            key={v.value}
                            keyName={summary.key}
                            value={v.value}
                            agentIds={v.agentIds}
                            agentHostnames={v.agentHostnames}
                            canDelete={!isAuto}
                            onDelete={() => onDeleteValue(v.value, v.agentIds)}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}
 
function AllTagsPanel({
    summaries,
    onDeleteValue,
}: {
    summaries: KeySummary[];
    onDeleteValue: (key: string, value: string, agentIds: string[]) => Promise<void>;
}) {
    const [filter, setFilter] = useState("");
    const [expanded, setExpanded] = useState<Set<string>>(new Set());
 
    const filtered = useMemo(() => {
        if (!filter) return summaries;
        const q = filter.toLowerCase();
        return summaries.filter(
            (s) =>
                s.key.toLowerCase().includes(q) ||
                s.values.some((v) => v.value.toLowerCase().includes(q))
        );
    }, [summaries, filter]);
 
    const toggleKey = useCallback((key: string) => {
        setExpanded((prev) => {
            const next = new Set(prev);
            if (next.has(key)) next.delete(key);
            else next.add(key);
            return next;
        });
    }, []);
 
    return (
        <div
            style={{
                background: themeVars.surface,
                border: `1px solid ${themeVars.border}`,
                padding: 20,
            }}
        >
            <div
                style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    marginBottom: 12,
                }}
            >
                <div style={{ ...sectionHeaderStyle, marginBottom: 0 }}>All tags</div>
                <input
                    type="text"
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                    placeholder="Filter keys..."
                    style={{
                        ...inputStyle,
                        width: 180,
                        fontSize: 11,
                        padding: "4px 8px",
                    }}
                />
            </div>
 
            {filtered.length === 0 && (
                <div
                    style={{
                        padding: "20px 0",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                        textAlign: "center",
                    }}
                >
                    {summaries.length === 0
                        ? "No tags yet. Apply one above to get started."
                        : "No keys match the filter."}
                </div>
            )}
 
            {filtered.map((s) => (
                <KeyRow
                    key={s.key}
                    summary={s}
                    expanded={expanded.has(s.key)}
                    onToggleExpand={() => toggleKey(s.key)}
                    onDeleteValue={(value, agentIds) =>
                        onDeleteValue(s.key, value, agentIds)
                    }
                />
            ))}
        </div>
    );
}

// --- Page ---

export function Tags({ user }: TagsProps) {
    const [agents, setAgents] = useState<OverviewAgent[]>([]);
    const [labelsByAgent, setLabelsByAgent] = useState<Map<string, AgentLabel[]>>(new Map());
    const [knownKeys, setKnownKeys] = useState<LabelKey[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
 
    const isAdmin = user.role === "admin" || user.role === "superadmin";
 
    const loadAll = useCallback(async () => {
        try {
            const agentList = await api.overview();
            setAgents(agentList);
 
            const [labelResults] = await Promise.all([
                Promise.allSettled(
                    agentList.map((a) =>
                        api.agentLabels(a.id).then((labels) => [a.id, labels] as const)
                    )
                ),
                api.labelKeys().then(setKnownKeys).catch(() => {}),
            ]);
 
            const m = new Map<string, AgentLabel[]>();
            for (const r of labelResults) {
                if (r.status === "fulfilled") {
                    m.set(r.value[0], r.value[1]);
                } else {
                    // Agent labels fetch failed; treat as empty so the UI still renders.
                }
            }
            setLabelsByAgent(m);
            setError(null);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to load");
        } finally {
            setLoading(false);
        }
    }, []);
 
    useEffect(() => {
        loadAll();
    }, [loadAll]);
 
    const refreshLabelsForAgents = useCallback(async (ids: string[]) => {
        const results = await Promise.allSettled(
            ids.map((id) =>
                api.agentLabels(id).then((labels) => [id, labels] as const)
            )
        );
        setLabelsByAgent((prev) => {
            const next = new Map(prev);
            for (const r of results) {
                if (r.status === "fulfilled") {
                    next.set(r.value[0], r.value[1]);
                }
            }
            return next;
        });
        // Refresh known keys — a brand-new key may now exist.
        api.labelKeys().then(setKnownKeys).catch(() => {});
    }, []);
 
    // Build summaries from the labels map.
    const summaries = useMemo<KeySummary[]>(() => {
        // key -> value -> agentIds
        const byKey = new Map<string, { source: "auto" | "user"; values: Map<string, string[]> }>();
        const hostnameById = new Map<string, string>(agents.map((a) => [a.id, a.hostname]));
 
        for (const [agentId, labels] of labelsByAgent) {
            for (const l of labels) {
                let entry = byKey.get(l.key);
                if (!entry) {
                    entry = { source: l.source, values: new Map() };
                    byKey.set(l.key, entry);
                }
                let ids = entry.values.get(l.value);
                if (!ids) {
                    ids = [];
                    entry.values.set(l.value, ids);
                }
                ids.push(agentId);
            }
        }
 
        const summaries: KeySummary[] = [];
        for (const [key, entry] of byKey) {
            const values: ValueGroup[] = [];
            const seenAgents = new Set<string>();
            for (const [value, agentIds] of entry.values) {
                for (const id of agentIds) seenAgents.add(id);
                values.push({
                    value,
                    agentIds,
                    agentHostnames: agentIds
                        .map((id) => hostnameById.get(id) ?? id)
                        .sort(),
                });
            }
            values.sort((a, b) => a.value.localeCompare(b.value));
            summaries.push({
                key,
                source: entry.source,
                agentCount: seenAgents.size,
                values,
            });
        }
 
        // User keys first, then auto. Alphabetical within each group.
        summaries.sort((a, b) => {
            if (a.source !== b.source) return a.source === "user" ? -1 : 1;
            return a.key.localeCompare(b.key);
        });
 
        return summaries;
    }, [labelsByAgent, agents]);
 
    const handleApplied = useCallback(
        async (agentIds: string[]) => {
            await refreshLabelsForAgents(agentIds);
        },
        [refreshLabelsForAgents]
    );
 
    const handleDeleteValue = useCallback(
        async (key: string, _value: string, agentIds: string[]) => {
            await Promise.allSettled(
                agentIds.map((id) => api.deleteAgentLabel(id, key))
            );
            await refreshLabelsForAgents(agentIds);
        },
        [refreshLabelsForAgents]
    );
 
    if (loading) return <LoadingSpinner />;
 
    if (error) {
        return (
            <div
                style={{
                    padding: 24,
                    color: themeVars.danger,
                    fontFamily: themeVars.font,
                }}
            >
                {error}
            </div>
        );
    }
 
    return (
        <div style={{ padding: 24 }}>
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 18,
                    fontWeight: 600,
                    color: themeVars.text,
                    marginBottom: 4,
                }}
            >
                Tags
            </div>
            <div
                style={{
                    fontSize: 12,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    marginBottom: 20,
                }}
            >
                Apply tags to agents and manage the tag namespace.
            </div>
 
            {isAdmin && (
                <ApplyTagPanel
                    agents={agents}
                    knownKeys={knownKeys}
                    labelsByAgent={labelsByAgent}
                    onApplied={handleApplied}
                />
            )}
 
            <AllTagsPanel
                summaries={summaries}
                onDeleteValue={isAdmin ? handleDeleteValue : (async () => {})}
            />
        </div>
    );
}