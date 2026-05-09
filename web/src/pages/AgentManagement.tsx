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
import type { DiskMetric, NetworkMetric, OverviewAgent, PlatformInfo, ProvisionResponse, User } from "../types";

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

function CopyBlock({ value }: { value: string }) {
    const [copied, setCopied] = useState(false);

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                background: themeVars.surface,
                border: `1px solid ${themeVars.border}`,
                padding: "6px 10px",
            }}
        >
            <code
                style={{
                    flex: 1,
                    fontFamily: "monospace",
                    fontSize: 11,
                    color: themeVars.text,
                    wordBreak: "break-all",
                }}
            >
                {value}
            </code>
            <button
                onClick={() => {
                    navigator.clipboard.writeText(value);
                    setCopied(true);
                    setTimeout(() => setCopied(false), 2000);
                }}
                style={{
                    padding: "2px 8px",
                    fontSize: 10,
                    fontFamily: themeVars.font,
                    color: copied ? themeVars.ok : themeVars.textMuted,
                    background: "transparent",
                    border: `1px solid ${copied ? themeVars.ok : themeVars.border}`,
                    cursor: "pointer",
                    flexShrink: 0,
                }}
            >
                {copied ? "Copied!" : "Copy"}
            </button>
        </div>
    )
}

function ProvisionModal({ onClose }: { onClose: () => void }) {
    const [platforms, setPlatforms] = useState<PlatformInfo[]>([]);
    const [selected, setSelected] = useState("");
    const [loading, setLoading] = useState(true);
    const [provisioning, setProvisioning] = useState(false);
    const [result, setResult] = useState<ProvisionResponse | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [copied, setCopied] = useState(false);

    useEffect(() => {
        api.platforms()
            .then((p) => {
                setPlatforms(p);
                if (p.length > 0) setSelected(p[0]!.filename);
            })
            .catch(() => setError("Failed to load platform."))
            .finally(() => setLoading(false));
    }, []);

    const handleProvision = async () => {
        setProvisioning(true);
        setError(null);
        try {
            const res = await api.provision(selected);
            setResult(res);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Provisioning failed.");
        } finally {
            setProvisioning(false);
        }
    };

    return (
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
                if (e.target === e.currentTarget) onClose();
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
                {/* Header */}
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
                        Provision Agent
                    </div>
                    <button
                        onClick={onClose}
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

                {loading && <LoadingSpinner />}

                {error && (
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            color: themeVars.danger,
                            marginBottom: 12,
                        }}
                    >
                        {error}
                    </div>
                )}

                {/* Platform selection */}
                {!loading && !result && (
                    <div>
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
                            Target Platform
                        </div>
                        <select
                            value={selected}
                            onChange={(e) => setSelected(e.target.value)}
                            style={{
                                padding: "6px 10px",
                                fontSize: 12,
                                fontFamily: themeVars.font,
                                color: themeVars.text,
                                background: themeVars.surface,
                                border: `1px solid ${themeVars.border}`,
                                cursor: "pointer",
                                marginBottom: 16,
                                width: "100%",
                            }}
                        >
                            {platforms.map((p) => (
                                <option key={p.filename} value={p.filename}>
                                    {p.label}
                                </option>
                            ))}
                        </select>

                        <div
                            style={{
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: themeVars.textDim,
                                marginBottom: 16,
                            }}
                        >
                            A one-time registration token will be generated. The token expires after first use or after the TTL, whichever comes first.
                        </div>

                        <button
                            onClick={handleProvision}
                            disabled={provisioning || !selected}
                            style={{
                                ...btnStyle,
                                opacity: provisioning ? 0.6 : 1,
                            }}
                        >
                            {provisioning ? "Provisioning..." : "Generate Token & Instructions"}
                        </button>
                    </div>
                )}

                {/* Result */}
                {result && (
                    <div>
                        {/* Token */}
                        <div style={{ marginBottom: 16 }}>
                            <div
                                style={{
                                    fontSize: 11,
                                    fontFamily: themeVars.font,
                                    color: themeVars.textDim,
                                    textTransform: "uppercase",
                                    letterSpacing: "0.04em",
                                    marginBottom: 4,
                                }}
                            >
                                Registration Token
                            </div>
                            <CopyBlock value={result.token} />
                            <div
                                style={{
                                    fontSize: 10,
                                    fontFamily: themeVars.font,
                                    color: themeVars.textDim,
                                    marginTop: 4,
                                }}
                            >
                                Expires: {new Date(result.expires_at).toLocaleString()}
                            </div>
                        </div>

                        {/* Download */}
                        {result.download_url && (
                            <div style={{ marginBottom: 16 }}>
                                <div
                                    style={{
                                        fontSize: 11,
                                        fontFamily: themeVars.font,
                                        color: themeVars.textDim,
                                        textTransform: "uppercase",
                                        letterSpacing: "0.04em",
                                        marginBottom: 4,
                                    }}
                                >
                                    Binary Download
                                </div>
                                <a
                                    href={result.download_url}
                                    download
                                    style={{
                                        display: "inline-block",
                                        padding: "6px 14px",
                                        fontSize: 11,
                                        fontFamily: themeVars.font,
                                        color: themeVars.text,
                                        background: themeVars.accentDim,
                                        border: `1px solid ${themeVars.accent}`,
                                        textDecoration: "none",
                                        letterSpacing: "0.03em",
                                        textTransform: "uppercase",
                                        marginBottom: 8,
                                    }}
                                >
                                    Download {result.platform}
                                </a>
                                <div
                                    style={{
                                        fontSize: 10,
                                        fontFamily: themeVars.font,
                                        color: themeVars.textDim,
                                        marginBottom: 4,
                                    }}
                                >
                                    Or download directly on the target host:
                                </div>
                                <CopyBlock
                                    value={`curl -fSL -o ${result.platform} "${window.location.origin}${result.download_url}"`}
                                />
                            </div>
                        )}

                        {/* Install instructions */}
                        <div style={{ marginBottom: 16 }}>
                            <div
                                style={{
                                    display: "flex",
                                    justifyContent: "space-between",
                                    alignItems: "center",
                                    marginBottom: 6,
                                }}
                            >
                                <div
                                    style={{
                                        fontSize: 11,
                                        fontFamily: themeVars.font,
                                        color: themeVars.textDim,
                                        textTransform: "uppercase",
                                        letterSpacing: "0.04em",
                                    }}
                                >
                                    Install Instructions ({result.install.type})
                                </div>
                                <button
                                    onClick={() => {
                                        const commands = result.install.steps
                                            .split("\n")
                                            .filter((line) => {
                                                const trimmed = line.trim();
                                                return trimmed && !/^\d+\.\s/.test(trimmed);
                                            })
                                            .join("\n");
                                        navigator.clipboard.writeText(commands);
                                        setCopied(true);
                                        setTimeout(() => setCopied(false), 2000);
                                    }}
                                    style={{
                                        padding: "3px 8px",
                                        fontSize: 10,
                                        fontFamily: themeVars.font,
                                        color: copied ? themeVars.ok : themeVars.textMuted,
                                        background: "transparent",
                                        border: `1px solid ${copied ? themeVars.ok : themeVars.border}`,
                                        cursor: "pointer",
                                    }}
                                >
                                    {copied ? "Copied!" : "Copy commands"}
                                </button>
                            </div>
                            <pre
                                style={{
                                    margin: 0,
                                    padding: 12,
                                    fontFamily: themeVars.font,
                                    fontSize: 11,
                                    lineHeight: 1.6,
                                    color: themeVars.text,
                                    background: themeVars.surface,
                                    border: `1px solid ${themeVars.border}`,
                                    whiteSpace: "pre-wrap",
                                    wordBreak: "break-word",
                                    maxHeight: 300,
                                    overflowY: "auto",
                                }}
                            >
                                {result.install.steps.split("\n").map((line, i) => {
                                    const trimmed = line.trim();
                                    const isStepHeader = /^\d+\.\s/.test(trimmed);
                                    const isComment = trimmed.startsWith("#");
                                    return (
                                        <span
                                            key={i}
                                            style={{
                                                color: isStepHeader
                                                    ? themeVars.accent
                                                    : isComment
                                                        ? themeVars.textDim
                                                        : themeVars.text,
                                                fontWeight: isStepHeader ? 600 : 400,
                                            }}
                                        >
                                            {line}
                                            {"\n"}
                                        </span>
                                    );
                                })}
                            </pre>
                        </div>

                        {/* Actions */}
                        <div
                            style={{
                                display: "flex",
                                gap: 8,
                                marginTop: 16,
                            }}
                        >
                            <button
                                onClick={() => setResult(null)}
                                style={btnStyle}
                            >
                                Provision Another
                            </button>
                            <button
                                onClick={onClose}
                                style={{
                                    ...btnStyle,
                                    color: themeVars.textMuted,
                                    background: "transparent",
                                    borderColor: themeVars.border,
                                }}
                            >
                                Done
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

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
    isAdmin,
}: {
    agent: OverviewAgent;
    onDelete: () => void;
    isAdmin: boolean;
}) {
    const [config, setConfig] = useState<AgentConfig>({});
    const [loading, setLoading] = useState(true);
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [availableFs, setAvailableFs] = useState<string[]>([]);
    const [availableIfaces, setAvailableIfaces] = useState<string[]>([]);
    const [upgradeSteps, setUpgradeSteps] = useState<string | null>(null);
    const [uninstallSteps, setUninstallSteps] = useState<string | null>(null);
    const [fullAgent, setFullAgent] = useState<{ ip_address?: string }>({});

    useEffect(() => {
        setLoading(true);
        const fetches: Promise<any>[] = [api.agent(agent.id)];
        if (isAdmin) {
            fetches.push(
                api.agentConfig(agent.id),
                api.agentDisk(agent.id, { type: "quick", range: "1h" }),
                api.agentNetwork(agent.id, { type: "quick", range: "1h" }),
            );
        }
        Promise.all(fetches)
            .then((results) => {
                setFullAgent(results[0]);
                if (isAdmin) {
                    setConfig(results[1] as AgentConfig);
                    const disks = results[2] as DiskMetric[];
                    const nets = results[3] as NetworkMetric[];
                    setAvailableFs([...new Set(disks.map((d: any) => d.filesystem))].filter(Boolean).sort());
                    setAvailableIfaces([...new Set(nets.map((n: any) => n.interface))].filter(Boolean).sort());
                }
            })
            .finally(() => setLoading(false));
    }, [agent.id, isAdmin]);


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

    if (loading) return <LoadingSpinner />;

    return (
        <div>
            <div style={{ display: "flex", gap: 24, marginBottom: 20, flexWrap: "wrap" }}>
                <StatBlock label="OS" value={agent.os ?? null} />
                <StatBlock label="Platform" value={agent.platform ?? null} />
                <StatBlock label="Arch" value={agent.arch ?? null} />
                <StatBlock label="Cores" value={agent.cpu_cores ? String(agent.cpu_cores) : null} />
                <StatBlock label="Uptime" value={formatUptime(agent.uptime)} />
                <StatBlock label="IP" value={fullAgent.ip_address ?? "—"} />
                <StatBlock label="Version" value={agent.version || "—"} />
            </div>

            {isAdmin && (
                <>
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

                    {upgradeSteps && (
                        <InstructionBlock
                            title={`Upgrade ${agent.hostname}`}
                            steps={upgradeSteps}
                            onClose={() => setUpgradeSteps(null)}
                        />
                    )}

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
                </>
            )}
        </div>
    );
}

function DangerAction({
    title,
    description,
    buttonLabel,
    confirmLabel,
    onConfirm,
}: {
    title: string;
    description: string;
    buttonLabel: string;
    confirmLabel: string;
    onConfirm: () => Promise<string>;
}) {
    const [confirming, setConfirming] = useState(false);
    const [result, setResult] = useState<string | null>(null);

    const handleConfirm = async () => {
        try {
            const msg = await onConfirm();
            setResult(msg);
            setConfirming(false);
            setTimeout(() => setResult(null), 3000);
        } catch {
            setResult("Action failed.");
            setTimeout(() => setResult(null), 3000);
        }
    };

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 16,
            }}
        >
            <div>
                <div
                    style={{
                        fontSize: 13,
                        fontFamily: themeVars.font,
                        fontWeight: 500,
                        color: themeVars.text,
                    }}
                >
                    {title}
                </div>
                <div
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                        marginTop: 2,
                    }}
                >
                    {description}
                </div>
            </div>

            <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0 }}>
                {result && (
                    <span
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.ok,
                        }}
                    >
                        {result}
                    </span>
                )}

                {!confirming ? (
                    <button
                        onClick={() => setConfirming(true)}
                        style={{
                            ...btnStyle,
                            color: themeVars.danger,
                            background: "transparent",
                            borderColor: themeVars.danger,
                        }}
                    >
                        {buttonLabel}
                    </button>
                ) : (
                    <>
                        <span
                            style={{
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: themeVars.danger,
                            }}
                        >
                            {confirmLabel}
                        </span>
                        <button
                            onClick={handleConfirm}
                            style={{
                                ...btnStyle,
                                color: "#fff",
                                background: themeVars.danger,
                                borderColor: themeVars.danger,
                            }}
                        >
                            Confirm
                        </button>
                        <button
                            onClick={() => setConfirming(false)}
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
        </div>
    );
}

interface AgentManagementProps {
    user: User;
}

export function AgentManagement({ user }: AgentManagementProps) {
    const [agents, setAgents] = useState<OverviewAgent[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [search, setSearch] = useState("");
    const [showProvision, setShowProvision] = useState(false);

    const isAdmin = user.role === "admin" || user.role === "superadmin";

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

                {isAdmin && (
                    <button onClick={() => setShowProvision(true)} style={btnStyle}>
                        + Provision Agent
                    </button>
                )}

                <span
                    style={{
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                        marginLeft: "auto",
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

            {/* Danger Zone */}
            {isAdmin && (
                <div
                    style={{
                        marginTop: 32,
                        border: `1px solid ${themeVars.danger}`,
                        padding: 20,
                    }}
                >
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: themeVars.font,
                            fontWeight: 600,
                            color: themeVars.danger,
                            textTransform: "uppercase",
                            letterSpacing: "0.04em",
                            marginBottom: 16,
                        }}
                    >
                        Danger Zone
                    </div>

                    <DangerAction
                        title="Purge Offline Agents"
                        description="Remove all agents that haven't been seen in more than 7 days. Cascades metric data."
                        buttonLabel="Purge Offline"
                        confirmLabel="This will permanently delete offline agents and all their data. Continue?"
                        onConfirm={async () => {
                            const res = await api.purgeOfflineAgents();
                            loadAgents();
                            return `${res.purged} agent${res.purged !== 1 ? "s" : ""} purged.`;
                        }}
                    />

                    <div style={{ borderTop: `1px solid ${themeVars.border}`, margin: "12px 0" }} />

                    <DangerAction
                        title="Revoke All Tokens"
                        description="Invalidate all pending registration tokens immediately."
                        buttonLabel="Revoke Tokens"
                        confirmLabel="This will revoke all pending registration tokens. Continue?"
                        onConfirm={async () => {
                            await api.revokeAllTokens();
                            return "All tokens revoked.";
                        }}
                    />
                </div>
            )}

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
                            isAdmin={isAdmin}
                        />
                    </div>
                </div>
            )}
            
            {showProvision && (
                <ProvisionModal onClose={() => setShowProvision(false)} />
            )}

        </div>
    );
}
