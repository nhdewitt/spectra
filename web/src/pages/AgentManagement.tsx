import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { api } from "../api";
import { statusColor, formatUptime, copyToClipboard } from "../utils";
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

type UpdateStatus = "queued" | "updating" | "restarting" | "updated" | "failed";

const UPDATE_TIMEOUT_MS = 60_000;
const UPDATE_POLL_MS = 3_000;

const UPDATE_STATUS_LABELS: Record<UpdateStatus, string> = {
    queued: "QUEUED",
    updating: "UPDATING",
    restarting: "RESTARTING",
    updated: "UPDATED",
    failed: "FAILED",
};

const UPDATE_STATUS_COLORS: Record<UpdateStatus, string> = {
    queued: themeVars.accent,
    updating: themeVars.accent,
    restarting: themeVars.warn,
    updated: themeVars.ok,
    failed: themeVars.danger,
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
                    copyToClipboard(value);
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

                {result && (
                    <div>
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
                                    value={`curl -fSL${result.config.ca_cert ? ` --cacert ${result.config.ca_cert}` : ''} -o ${result.platform} "${result.config.server}/api/v1/admin/releases/${result.platform}?token=${result.token}"`}
                                />
                                {result.config.ca_cert && (
                                    <div
                                        style={{
                                            fontSize: 10,
                                            fontFamily: themeVars.font,
                                            color: themeVars.warn,
                                            marginTop: 4,
                                        }}
                                    >
                                        Requires the CA certificate - follow the install steps below first.
                                    </div>
                                )}
                            </div>
                        )}

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
                                                if (!trimmed) return true; // preserve blank lines
                                                return trimmed && !/^\d+\.\s/.test(trimmed);
                                            })
                                            .join("\n");
                                        copyToClipboard(commands);
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

    const handleUpgrade = async () => {
        try {
            const res = await api.upgradeInstructions(agent.id);
            if (res.steps) {
                setUpgradeSteps(res.steps);
            }
        } catch {
            setUpgradeSteps("Failed to load upgrade instructions.");
        }
    };

    const handleShowUninstall = async () => {
        try {
            const res = await api.uninstallInstructions(agent.id);
            if (res.steps) {
                setUninstallSteps(res.steps);
            } else {
                // Agent platform unknown or offline. Skip straight to delete confirmation.
                setConfirmDelete(true);
            }
        } catch {
            setUninstallSteps("Failed to load uninstall instructions.");
        }
    };

    if (loading) return <LoadingSpinner />;

    return (
        <div>
            <div style={{ display: "flex", gap: 24, marginBottom: 20, flexWrap: "wrap" }}>
                <StatBlock label="Agent ID" value={agent.id} copyable small />
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
 
    const [updateSelected, setUpdateSelected] = useState<Set<string>>(new Set());
    const [updating, setUpdating] = useState(false);
    const [updateResult, setUpdateResult] = useState<{ queued: number; skipped: number; failed: number } | null>(null);
    const [updateStatuses, setUpdateStatuses] = useState<Map<string, UpdateStatus>>(new Map());
    const updateStartedAt = useRef<number>(0);
 
    const isAdmin = user.role === "admin" || user.role === "superadmin";

    const hasPendingUpdates = useMemo(() => {
        for (const status of updateStatuses.values()) {
            if (status === "queued" || status === "updating" || status === "restarting") return true;
        }
        return false;
    }, [updateStatuses]);

    const loadAgents = useCallback(() => {
        return api.overview()
            .then(setAgents)
            .catch((err) =>
                setError(err instanceof Error ? err.message : "Failed to load")
            )
            .finally(() => setLoading(false));
    }, []);

    // Normal polling at 30s, fast polling at 3s when updates are pending
    useEffect(() => {
        setLoading(true);
        loadAgents();
        const interval = hasPendingUpdates ? UPDATE_POLL_MS : 30_000;
        const id = setInterval(loadAgents, interval);
        return () => clearInterval(id);
    }, [loadAgents, hasPendingUpdates]);

    // Track update progress by watching update_available changes
    useEffect(() => {
        if (updateStatuses.size === 0) return;

        const elapsed = Date.now() - updateStartedAt.current;

        setUpdateStatuses((prev) => {
            const next = new Map(prev);
            let changed = false;

            for (const [agentId, status] of prev) {
                if (status === "updated" || status === "failed") continue;

                const agent = agents.find((a) => a.id === agentId);
                if (!agent) continue;

                // Agent's binary now matches the release — update complete
                if (!agent.update_available) {
                    next.set(agentId, "updated");
                    changed = true;
                    continue;
                }

                // Check if agent went offline (restarting)
                const lastSeen = agent.last_seen ? new Date(agent.last_seen).getTime() : 0;
                const isOffline = (Date.now() - lastSeen) > 15_000;

                if (status === "queued" && !isOffline) {
                    next.set(agentId, "updating");
                    changed = true;
                } else if ((status === "queued" || status === "updating") && isOffline) {
                    next.set(agentId, "restarting");
                    changed = true;
                }

                // Timeout
                if (elapsed > UPDATE_TIMEOUT_MS) {
                    next.set(agentId, "failed");
                    changed = true;
                }
            }

            return changed ? next : prev;
        });
    }, [agents, updateStatuses]);

    // Clear terminal statuses after 10s
    useEffect(() => {
        if (updateStatuses.size === 0) return;

        const hasTerminal = Array.from(updateStatuses.values()).some(
            (s) => s === "updated" || s === "failed"
        );
        if (!hasTerminal) return;

        const timeout = setTimeout(() => {
            setUpdateStatuses((prev) => {
                const next = new Map<string, UpdateStatus>();
                for (const [id, status] of prev) {
                    if (status !== "updated" && status !== "failed") {
                        next.set(id, status);
                    }
                }
                return next;
            });
        }, 10_000);

        return () => clearTimeout(timeout);
    }, [updateStatuses]);

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

    const outdatedIds = useMemo(() => {
        return new Set(
            agents
                .filter((a) => a.update_available)
                .map((a) => a.id)
        );
    }, [agents]);

    // Clean up selections when outdated set changes
    useEffect(() => {
        setUpdateSelected((prev) => {
            const next = new Set<string>();
            for (const id of prev) {
                if (outdatedIds.has(id)) next.add(id);
            }
            return next.size === prev.size ? prev : next;
        });
    }, [outdatedIds]);

    const toggleUpdateSelect = useCallback((agentId: string) => {
        setUpdateSelected((prev) => {
            const next = new Set(prev);
            if (next.has(agentId)) {
                next.delete(agentId);
            } else {
                next.add(agentId);
            }
            return next;
        });
    }, []);

    const selectAllOutdated = useCallback(() => {
        setUpdateSelected((prev) => {
            if (prev.size === outdatedIds.size) return new Set();
            return new Set(outdatedIds);
        });
    }, [outdatedIds]);

    const handlePushUpdate = useCallback(async () => {
        if (updateSelected.size === 0) return;
        setUpdating(true);
        setUpdateResult(null);
        try {
            const ids = Array.from(updateSelected);
            const res = await api.pushUpdate(ids);
            setUpdateResult(res);

            const statuses = new Map<string, UpdateStatus>();
            for (const id of ids) {
                statuses.set(id, "queued");
            }
            updateStartedAt.current = Date.now();
            setUpdateStatuses(statuses);
            setUpdateSelected(new Set());
        } catch {
            setUpdateResult({ queued: 0, skipped: 0, failed: updateSelected.size });
        } finally {
            setUpdating(false);
        }
    }, [updateSelected]);

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

    const hasOutdated = outdatedIds.size > 0;

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
 
            <div style={{ display: "flex", gap: 12, marginBottom: 16, alignItems: "center", flexWrap: "wrap" }}>
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
 
                {isAdmin && hasOutdated && (
                    <>
                        <button
                            onClick={selectAllOutdated}
                            disabled={hasPendingUpdates}
                            style={{
                                ...btnStyle,
                                color: themeVars.textMuted,
                                background: "transparent",
                                borderColor: themeVars.border,
                                opacity: hasPendingUpdates ? 0.4 : 1,
                                cursor: hasPendingUpdates ? "default" : "pointer",
                            }}
                        >
                            {updateSelected.size === outdatedIds.size ? "Deselect All" : "Select All Outdated"}
                        </button>
                        <button
                            onClick={handlePushUpdate}
                            disabled={updateSelected.size === 0 || updating || hasPendingUpdates}
                            style={{
                                ...btnStyle,
                                opacity: updateSelected.size === 0 || updating || hasPendingUpdates ? 0.4 : 1,
                                cursor: updateSelected.size === 0 || updating || hasPendingUpdates ? "default" : "pointer",
                            }}
                        >
                            {updating
                                ? "Pushing..."
                                : hasPendingUpdates
                                    ? "Update in progress..."
                                    : `Update ${updateSelected.size} Agent${updateSelected.size !== 1 ? "s" : ""}`}
                        </button>
                    </>
                )}
 
                {updateResult && !hasPendingUpdates && (
                    <span
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: updateResult.failed > 0 ? themeVars.warn : themeVars.ok,
                        }}
                    >
                        {updateResult.queued} queued
                        {updateResult.skipped > 0 && `, ${updateResult.skipped} skipped`}
                        {updateResult.failed > 0 && `, ${updateResult.failed} failed`}
                    </span>
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
                            {isAdmin && hasOutdated && <th style={{ ...tableHeaderStyle, width: 32 }} />}
                            <th style={tableHeaderStyle}>Status</th>
                            <th style={tableHeaderStyle}>Hostname</th>
                            <th style={tableHeaderStyle}>OS</th>
                            <th style={tableHeaderStyle}>Platform</th>
                            <th style={tableHeaderStyle}>Arch</th>
                            <th style={tableHeaderStyle}>Version</th>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Cores</th>
                            <th style={tableHeaderStyle}>Last Seen</th>
                        </tr>
                    </thead>
                    <tbody>
                        {paged.map((a, i) => {
                            const isOutdated = outdatedIds.has(a.id);
                            const isChecked = updateSelected.has(a.id);
                            const agentUpdateStatus = updateStatuses.get(a.id);
 
                            return (
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
                                    {isAdmin && hasOutdated && (
                                        <td style={tableCellStyle}>
                                            {isOutdated && !agentUpdateStatus && (
                                                <input
                                                    type="checkbox"
                                                    checked={isChecked}
                                                    onClick={(e) => e.stopPropagation()}
                                                    onChange={() => toggleUpdateSelect(a.id)}
                                                    style={{ cursor: "pointer" }}
                                                />
                                            )}
                                        </td>
                                    )}
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
                                    <td
                                        style={{
                                            ...tableMutedCellStyle,
                                            color: agentUpdateStatus
                                                ? UPDATE_STATUS_COLORS[agentUpdateStatus]
                                                : isOutdated
                                                    ? themeVars.warn
                                                    : themeVars.ok,
                                        }}
                                    >
                                        {a.version || "—"}
                                        {agentUpdateStatus ? (
                                            <span
                                                style={{
                                                    fontSize: 9,
                                                    marginLeft: 6,
                                                    color: UPDATE_STATUS_COLORS[agentUpdateStatus],
                                                    textTransform: "uppercase",
                                                    letterSpacing: "0.03em",
                                                }}
                                            >
                                                {UPDATE_STATUS_LABELS[agentUpdateStatus]}
                                            </span>
                                        ) : isOutdated ? (
                                            <span
                                                style={{
                                                    fontSize: 9,
                                                    marginLeft: 6,
                                                    color: themeVars.warn,
                                                    textTransform: "uppercase",
                                                    letterSpacing: "0.03em",
                                                }}
                                            >
                                                outdated
                                            </span>
                                        ) : null}
                                    </td>
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
                            );
                        })}
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