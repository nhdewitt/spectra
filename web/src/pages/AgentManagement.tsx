import { useState, useEffect, useCallback, useMemo } from "react";
import { api } from "../api";
import { statusColor, formatUptime } from "../utils";
import {
    StatBlock,
    tableHeaderStyle,
    tableCellStyle,
    tableMutedCellStyle,
    LoadingSpinner,
    InstructionBlock,
} from "../components/ui";
import { usePagination, Pagination } from "../hooks/usePagination";
import { themeVars } from "../theme";
import type { OverviewAgent } from "../types";

interface AgentConfig {
    ignored_filesystems?: string[];
    ignored_interfaces?: string[];
    labels?: Record<string, string>;
    log_level?: string;
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

function IgnoreChecklist({
    label,
    available,
    ignored,
    onToggle,
}: {
    label: string;
    available: string[];
    ignored: string[];
    onToggle: (item: string, hide: boolean) => void;
}) {
    return (
        <div style={{ marginBottom: 16 }}>
            <div
                style={{
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                    marginBottom: 6,
                }}
            >
                {label}
            </div>
            {available.length === 0 && (
                <div style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.textDim }}>
                    No data available.
                </div>
            )}
            {available.map((item) => {
                const isIgnored = ignored.includes(item);
                return (
                    <label
                        key={item}
                        style={{
                            display: "flex",
                            alignItems: "center",
                            gap: 8,
                            padding: "4px 0",
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            color: isIgnored ? themeVars.textDim : themeVars.text,
                            cursor: "pointer",
                        }}
                    >
                        <input
                            type="checkbox"
                            checked={!isIgnored}
                            onChange={() => onToggle(item, !isIgnored)}
                        />
                        <span style={{ textDecoration: isIgnored ? "line-through" : "none" }}>
                            {item}
                        </span>
                    </label>
                );
            })}
        </div>
    );
}

function LabelEditor({
    labels,
    onSet,
    onRemove,
}: {
    labels: Record<string, string>;
    onSet: (key: string, value: string) => void;
    onRemove: (key: string) => void;
}) {
    const [keyInput, setKeyInput] = useState("");
    const [valueInput, setValueInput] = useState("");

    const handleAdd = () => {
        const k = keyInput.trim();
        const v = valueInput.trim();
        if (k && v) {
            onSet(k, v);
            setKeyInput("");
            setValueInput("");
        }
    };

    const entries = Object.entries(labels);

    return (
        <div style={{ marginBottom: 16 }}>
            <div
                style={{
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                    marginBottom: 6,
                }}
            >
                Labels
            </div>

            <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
                <input
                    type="text"
                    value={keyInput}
                    onChange={(e) => setKeyInput(e.target.value)}
                    placeholder="Key"
                    style={{
                        flex: 1,
                        padding: "4px 8px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                />
                <input
                    type="text"
                    value={valueInput}
                    onChange={(e) => setValueInput(e.target.value)}
                    placeholder="Value"
                    style={{
                        flex: 1,
                        padding: "4px 8px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                />
                <button onClick={handleAdd} style={btnStyle}>
                    Add
                </button>
            </div>

            {entries.length === 0 && (
                <div style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.textDim }}>
                    No labels set.
                </div>
            )}
            <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                {entries.map(([k, v]) => (
                    <span
                        key={k}
                        style={{
                            display: "inline-flex",
                            alignItems: "center",
                            gap: 4,
                            padding: "2px 8px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.text,
                            background: themeVars.surfaceHover,
                            border: `1px solid ${themeVars.border}`,
                        }}
                    >
                        <span style={{ color: themeVars.accent }}>{k}</span>
                        <span style={{ color: themeVars.textDim }}>=</span>
                        {v}
                        <button
                            onClick={() => onRemove(k)}
                            style={{
                                background: "none",
                                border: "none",
                                color: themeVars.danger,
                                cursor: "pointer",
                                fontSize: 12,
                                padding: 0,
                                lineHeight: 1,
                            }}
                        >
                            ×
                        </button>
                    </span>
                ))}
            </div>
        </div>
    );
}

function AgentConfigPanel({
    agent,
    onDelete,
}: {
    agent: OverviewAgent;
    onDelete: () => void;
}) {
    const [config, setConfig] = useState<AgentConfig>({});
    const [loading, setLoading] = useState(true);
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [availableFs, setAvailableFs] = useState<string[]>([]);
    const [availableIfaces, setAvailableIfaces] = useState<string[]>([]);
    const [upgradeSteps, setUpgradeSteps] = useState<string | null>(null);
    const [uninstallSteps, setUninstallSteps] = useState<string | null>(null);

    useEffect(() => {
        setLoading(true);
        Promise.all([
            api.agentConfig(agent.id),
            api.agentDisk(agent.id, { type: "quick", range: "1h" }),
            api.agentNetwork(agent.id, { type: "quick", range: "1h" }),
        ])
            .then(([cfg, disks, nets]) => {
                setConfig(cfg as AgentConfig);
                setAvailableFs([...new Set(disks.map((d) => d.filesystem))].filter(Boolean).sort());
                setAvailableIfaces([...new Set(nets.map((n) => n.interface))].filter(Boolean).sort());
            })
            .finally(() => setLoading(false));
    }, [agent.id]);

    const saveList = useCallback(
        async (key: "ignored_filesystems" | "ignored_interfaces", items: string[]) => {
            setConfig((prev) => ({ ...prev, [key]: items }));
            if (items.length === 0) {
                await api.deleteAgentConfig(agent.id, key);
            } else {
                await api.setAgentConfig(agent.id, key, items);
            }
        },
        [agent.id]
    );

    const saveLabels = useCallback(
        async (labels: Record<string, string>) => {
            setConfig((prev) => ({ ...prev, labels }));
            if (Object.keys(labels).length === 0) {
                await api.deleteAgentConfig(agent.id, "labels");
            } else {
                await api.setAgentConfig(agent.id, "labels", labels);
            }
        },
        [agent.id]
    );

    const handleUpgrade = async () => {
        try {
            const res = await api.upgradeInstructions(agent.id);
            setUpgradeSteps(res.steps);
        } catch {
            setUpgradeSteps("Failed to load upgrade instructions.");
        }
    };

    const handleShowUninstall = async () => {
        try {
            const res = await api.uninstallInstructions(agent.id);
            setUninstallSteps(res.steps);
        } catch {
            setUninstallSteps("Failed to load uninstall instructions.");
        }
    };

    if (loading) return <LoadingSpinner />

    return (
        <div>
            <div style={{ display: "flex", gap: 24, marginBottom: 20, flexWrap: "wrap" }}>
                <StatBlock label="OS" value={agent.os ?? null} />
                <StatBlock label="Platform" value={agent.platform ?? null} />
                <StatBlock label="Arch" value={agent.arch ?? null} />
                <StatBlock label="Cores" value={agent.cpu_cores ? String(agent.cpu_cores) : null} />
                <StatBlock label="Uptime" value={formatUptime(agent.uptime)} />
                <StatBlock label="IP" value={agent.ip_address ?? null} />
                <StatBlock label="Version" value={agent.version || "—"} />
            </div>

            <IgnoreChecklist
                label="Filesystems"
                available={availableFs}
                ignored={config.ignored_filesystems ?? []}
                onToggle={(item, hide) => {
                    const current = config.ignored_filesystems ?? [];
                    const next = hide
                        ? [...current, item]
                        : current.filter((i) => i !== item);
                    saveList("ignored_filesystems", next);
                }}
            />

            <IgnoreChecklist
                label="Network Interfaces"
                available={availableIfaces}
                ignored={config.ignored_interfaces ?? []}
                onToggle={(item, hide) => {
                    const current = config.ignored_interfaces ?? [];
                    const next = hide
                        ? [...current, item]
                        : current.filter((i) => i !== item);
                    saveList("ignored_interfaces", next);
                }}
            />

            <div style={{ marginBottom: 16 }}>
                <div
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                        textTransform: "uppercase",
                        letterSpacing: "0.04em",
                        marginBottom: 6,
                    }}
                >
                    Log Level
                </div>
                <select
                    value={config.log_level ?? "info"}
                    onChange={async (e) => {
                        const level = e.target.value;
                        setConfig((prev) => ({ ...prev, log_level: level }));
                        await api.setAgentConfig(agent.id, "log_level", level);
                    }}
                    style={{
                        padding: "4px 8px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        cursor: "pointer",
                    }}
                >
                    <option value="debug">Debug</option>
                    <option value="info">Info</option>
                    <option value="warn">Warn</option>
                    <option value="error">Error</option>
                </select>
            </div>

            {/* Actions */}
            <div
                style={{
                    marginTop: 20,
                    borderTop: `1px solid ${themeVars.border}`,
                    paddingTop: 16,
                    display: "flex",
                    gap: 8,
                    flexWrap: "wrap",
                    alignItems: "center",
                }}
            >
                <button onClick={handleUpgrade} style={btnStyle}>
                    Upgrade instructions
                </button>

                {!confirmDelete ? (
                    <button
                        onClick={handleShowUninstall}
                        style={{
                            ...btnStyle,
                            color: themeVars.danger,
                            background: "transparent",
                            borderColor: themeVars.danger,
                        }}
                    >
                        Delete agent
                    </button>
                ) : (
                    <>
                        <span
                            style={{
                                fontSize: 12,
                                fontFamily: themeVars.font,
                                color: themeVars.danger,
                            }}
                        >
                            Delete {agent.hostname} and all its data?
                        </span>
                        <button
                            onClick={onDelete}
                            style={{
                                ...btnStyle,
                                color: "#fff",
                                background: themeVars.danger,
                                borderColor: themeVars.danger,
                            }}
                        >
                            Confirm delete
                        </button>
                        <button
                            onClick={() => setConfirmDelete(false)}
                            style={{
                                ...btnStyle,
                                color: themeVars.textMuted,
                                background: "transparent",
                                borderColor: themeVars.border,
                            }}
                        >
                            Cancel
                        </button>
                    </>
                )}
            </div>

            {/* Upgrade modal */}
            {upgradeSteps && (
                <InstructionBlock
                    title={`Upgrade ${agent.hostname}`}
                    steps={upgradeSteps}
                    onClose={() => setUpgradeSteps(null)}
                />
            )}

            {/* Uninstall modal - shows before delete confirmation */}
            {uninstallSteps && !confirmDelete && (
                <InstructionBlock
                    title={`Uninstall from ${agent.hostname}`}
                    steps={uninstallSteps}
                    onClose={() => setUninstallSteps(null)}
                    footer={
                        <div
                            style={{
                                padding: "12px 16px",
                                borderTop: `1px solid ${themeVars.border}`,
                                display: "flex",
                                justifyContent: "flex-end",
                                gap: 8,
                            }}
                        >
                            <button
                                onClick={() => setUninstallSteps(null)}
                                style={{
                                    ...btnStyle,
                                    color: themeVars.textMuted,
                                    background: "transparent",
                                    borderColor: themeVars.border,
                                }}
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => {
                                    setUninstallSteps(null);
                                    setConfirmDelete(true);
                                }}
                                style={{
                                    ...btnStyle,
                                    color: "#fff",
                                    background: themeVars.danger,
                                    borderColor: themeVars.danger,
                                }}
                            >
                                I've uninstalled, delete the agent
                            </button>
                        </div>
                    }
                />
            )}
        </div>
    );
}

export function AgentManagement() {
    const [agents, setAgents] = useState<OverviewAgent[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [search, setSearch] = useState("");

    const loadAgents = useCallback(() => {
        api.overview()
            .then(setAgents)
            .catch((err) =>
                setError(err instanceof Error ? err.message : "Failed to load")
            )
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        setLoading(true);
        loadAgents();
        const id = setInterval(loadAgents, 30_000);
        return () => clearInterval(id);
    }, [loadAgents]);

    useEffect(() => {
        if (!selectedId) return;
        const handler = (e: KeyboardEvent) => {
            if (e.key === "Escape") setSelectedId(null);
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [selectedId]);

    const filtered = useMemo(() => {
        if (!search) return agents;
        const q = search.toLowerCase();
        return agents.filter(
            (a) =>
                a.hostname.toLowerCase().includes(q) ||
                (a.os ?? "").toLowerCase().includes(q) ||
                (a.platform ?? "").toLowerCase().includes(q)
        );
    }, [agents, search]);

    const { paged, page, setPage, totalPages, total } = usePagination(filtered, 20);

    const handleDelete = useCallback(
        async (agentId: string) => {
            try {
                await api.deleteAgent(agentId);
                setSelectedId(null);
                loadAgents();
            } catch {
                // TODO: show error toast
            }
        },
        [loadAgents]
    );

    if (loading) return <LoadingSpinner />;

    if (error) {
        return (
            <div style={{ padding: 24, color: themeVars.danger, fontFamily: themeVars.font }}>
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
                    marginBottom: 16,
                }}
            >
                Agent Management
            </div>

            {/* Search */}
            <div style={{ display: "flex", gap: 12, marginBottom: 16, alignItems: "center" }}>
                <input
                    type="text"
                    value={search}
                    onChange={(e) => {
                        setSearch(e.target.value);
                        setPage(0);
                    }}
                    placeholder="Search agents..."
                    style={{
                        padding: "6px 10px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        flex: "0 1 300px",
                    }}
                />
                <span
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                    }}
                >
                    {agents.length} agent{agents.length === 1 ? "" : "s"} registered
                </span>
            </div>

            {/* Agent table */}
            <div style={{ overflowX: "auto" }}>
                <table
                    style={{
                        width: "100%",
                        borderCollapse: "collapse",
                        textAlign: "left",
                    }}
                >
                    <thead>
                        <tr>
                            <th style={tableHeaderStyle}>Status</th>
                            <th style={tableHeaderStyle}>Hostname</th>
                            <th style={tableHeaderStyle}>OS</th>
                            <th style={tableHeaderStyle}>Platform</th>
                            <th style={tableHeaderStyle}>Arch</th>
                            <td style={tableHeaderStyle}>Version</td>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Cores</th>
                            <th style={tableHeaderStyle}>Last Seen</th>
                        </tr>
                    </thead>
                    <tbody>
                        {paged.map((a, i) => (
                            <tr
                                key={a.id}
                                onClick={() =>
                                    setSelectedId(selectedId === a.id ? null : a.id)
                                }
                                style={{
                                    cursor: "pointer",
                                    background:
                                        selectedId === a.id
                                            ? themeVars.accentDim
                                            : i % 2 === 0
                                                ? "transparent"
                                                : themeVars.surfaceHover,
                                }}
                            >
                                <td style={tableCellStyle}>
                                    <span
                                        style={{
                                            width: 8,
                                            height: 8,
                                            borderRadius: "50%",
                                            background: statusColor(a),
                                            display: "inline-block",
                                        }}
                                    />
                                </td>
                                <td style={{ ...tableCellStyle, fontWeight: 500 }}>
                                    {a.hostname}
                                </td>
                                <td style={tableMutedCellStyle}>{a.os}</td>
                                <td style={tableMutedCellStyle}>{a.platform}</td>
                                <td style={tableMutedCellStyle}>{a.arch}</td>
                                <td style={tableMutedCellStyle}>{a.version || "—"}</td>
                                <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                    {a.cpu_cores}
                                </td>
                                <td style={tableMutedCellStyle}>
                                    {a.last_seen
                                        ? new Date(a.last_seen).toLocaleString(undefined, {
                                            month: "short",
                                            day: "numeric",
                                            hour: "2-digit",
                                            minute: "2-digit",
                                        })
                                        : "Never"}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            <Pagination
                page={page}
                totalPages={totalPages}
                total={total}
                pageSize={20}
                onPageChange={setPage}
            />

            {/* Expanded config panel */}
            {selectedId && (
                <div
                    style={{
                        position: "fixed",
                        top: 0,
                        left: 0,
                        right: 0,
                        bottom: 0,
                        background: "rgba(0, 0, 0, 0.6)",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        zIndex: 100,
                    }}
                    onClick={(e) => {
                        if (e.target === e.currentTarget) setSelectedId(null);
                    }}
                >
                    <div
                        style={{
                            background: themeVars.bg,
                            border: `1px solid ${themeVars.border}`,
                            padding: 24,
                            maxWidth: 700,
                            width: "90%",
                            maxHeight: "80vh",
                            overflowY: "auto",
                        }}
                    >
                        {/* Close button */}
                        <div
                            style={{
                                display: "flex",
                                justifyContent: "space-between",
                                alignItems: "center",
                                marginBottom: 16,
                            }}
                        >
                            <div
                                style={{
                                    fontFamily: themeVars.font,
                                    fontSize: 16,
                                    fontWeight: 600,
                                    color: themeVars.text,
                                }}
                            >
                                {agents.find((a) => a.id === selectedId)?.hostname}
                            </div>
                            <button
                                onClick={() => setSelectedId(null)}
                                style={{
                                    background: "none",
                                    border: "none",
                                    color: themeVars.textMuted,
                                    fontSize: 18,
                                    cursor: "pointer",
                                    fontFamily: themeVars.font,
                                }}
                            >
                                ×
                            </button>
                        </div>

                        <AgentConfigPanel
                            agent={agents.find((a) => a.id === selectedId)!}
                            onDelete={() => handleDelete(selectedId)}
                        />
                    </div>
                </div>
            )}
        </div>
    );
}
