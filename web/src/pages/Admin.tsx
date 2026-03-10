import { useState, useEffect } from "react";
import { api } from "../api";
import { OSIcon } from "../icons";
import { theme } from "../theme";
import type { PlatformInfo, ProvisionResponse } from "../types";

function CopyButton({ text, label }: { text: string; label?: string }) {
    const [copied, setCopied] = useState(false);
    const [failed, setFailed] = useState(false);

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(text);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            setFailed(true);
            setTimeout(() => setFailed(false), 2000);
        }
    };

    return (
        <button
            onClick={handleCopy}
            style={{
                padding: "4px 10px",
                fontSize: 11,
                fontFamily: theme.font,
                color: copied ? theme.ok : failed ? theme.danger : theme.textMuted,
                background: "transparent",
                border: `1px solid ${copied ? theme.ok : failed ? theme.danger : theme.border}`,
                cursor: "pointer",
                letterSpacing: "0.02em",
                transition: "all 0.15s ease",
            }}
        >
            {copied ? "COPIED" : failed ? "FAILED" : label ?? "COPY"}
        </button>
    );
}

function downloadBlob(content: string, filename: string, type: string) {
    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
}

function configPath(platform: string): string {
    if (platform.startsWith("spectra-agent-freebsd")) return "/usr/local/etc/spectra/";
    if (platform.startsWith("spectra-agent-windows")) return "C:\\spectra\\";
    return "/etc/spectra";
}

function PlatformSelector({
    platforms,
    selected,
    onSelect,
}: {
    platforms: PlatformInfo[];
    selected: string | null;
    onSelect: (filename: string) => void;
}) {
    const groups = platforms.reduce<Record<string, PlatformInfo[]>>(
        (acc, p) => {
            if (!acc[p.os]) acc[p.os] = [];
            acc[p.os]!.push(p);
            return acc;
        },
        {} as Record<string, PlatformInfo[]>
    );

    const osOrder = ["linux", "darwin", "freebsd", "windows"];

    return (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {osOrder
                .filter((os) => groups[os])
                .map((os) => (
                    <div key={os}>
                        <div
                            style={{
                                fontSize: 10,
                                fontFamily: theme.font,
                                color: theme.textDim,
                                letterSpacing: "0.05em",
                                textTransform: "uppercase",
                                marginBottom: 6,
                            }}
                        >
                            <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                                <OSIcon os={os} platform="" size={14} />
                                {os === "darwin" ? "macOS" : os.charAt(0).toUpperCase() + os.slice(1)}
                            </span>
                        </div>
                        <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                            {groups[os]!.map((p) => (
                                <button
                                    key={p.filename}
                                    onClick={() => onSelect(p.filename)}
                                    style={{
                                        padding: "8px 14px",
                                        fontSize: 12,
                                        fontFamily: theme.font,
                                        color:
                                            selected === p.filename ? theme.text : theme.textMuted,
                                        background:
                                            selected === p.filename
                                                ? theme.accentDim
                                                : theme.surface,
                                        border: `1px solid ${selected === p.filename ? theme.accent : theme.border}`,
                                        cursor: "pointer",
                                        transition: "all 0.1s ease",
                                    }}
                                >
                                    {p.label}
                                </button>
                            ))}
                        </div>
                    </div>
                ))}
        </div>
    );
}

function ProvisionResult({
    result,
    onBack,
}: {
    result: ProvisionResponse;
    onBack: () => void;
}) {
    const configJSON = JSON.stringify(result.config, null, 2)

    return (
        <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
            {/* Header */}
            <div
                style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                }}
            >
                <div>
                    <div
                        style={{
                            fontSize: 14,
                            fontFamily: theme.font,
                            fontWeight: 500,
                            color: theme.text,
                        }}
                    >
                        Agent Provisioned
                    </div>
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: theme.font,
                            color: theme.textMuted,
                            marginTop: 4,
                        }}
                    >
                        Token expires{" "}
                        {new Date(result.expires_at).toLocaleString()}
                    </div>
                </div>
                <button
                    onClick={onBack}
                    style={{
                        padding: "6px 12px",
                        fontSize: 11,
                        fontFamily: theme.font,
                        color: theme.textMuted,
                        background: "transparent",
                        border: `1px solid ${theme.border}`,
                        cursor: "pointer",
                    }}
                >
                    ← PROVISION ANOTHER
                </button>
            </div>

            {/* Token */}
            <div
                style={{
                    background: theme.surface,
                    border: `1px solid ${theme.border}`,
                    padding: 16,
                }}
            >
                <div
                    style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        marginBottom: 8,
                    }}
                >
                    <div
                        style={{
                            fontSize: 10,
                            fontFamily: theme.font,
                            color: theme.textDim,
                            letterSpacing: "0.05em",
                            textTransform: "uppercase",
                        }}
                    >
                        Registration Token
                    </div>
                    <CopyButton text={result.token} />
                </div>
                <code
                    style={{
                        fontSize: 13,
                        fontFamily: theme.font,
                        color: theme.warn,
                        wordBreak: "break-all",
                    }}
                >
                    {result.token}
                </code>
            </div>

            {/* Downloads */}
            <div
                style={{
                    display: "flex",
                    gap: 12,
                }}
            >
                {/* Config download */}
                <button
                    onClick={() =>
                        downloadBlob(configJSON, "spectra-agent.json", "application/json")
                    }
                    style={{
                        flex: 1,
                        padding: "12px 16px",
                        fontSize: 12,
                        fontFamily: theme.font,
                        color: theme.text,
                        background: theme.surface,
                        border: `1px solid ${theme.border}`,
                        cursor: "pointer",
                        textAlign: "left",
                    }}
                >
                    <div style={{ fontWeight: 500, marginBottom: 4 }}>
                        ↓ spectra-agent.json
                    </div>
                    <div style={{ fontSize: 11, color: theme.textDim }}>
                        Config file - place in {configPath(result.platform)}
                    </div>
                </button>

                {/* Binary download */}
                {result.download_url && (
                    <a
                        href={result.download_url}
                        download
                        style={{
                            flex: 1,
                            padding: "12px 16px",
                            fontSize: 12,
                            fontFamily: theme.font,
                            color: theme.text,
                            background: theme.surface,
                            border: `1px solid ${theme.border}`,
                            cursor: "pointer",
                            textAlign: "left",
                            textDecoration: "none",
                        }}
                    >
                        <div style={{ fontWeight: 500, marginBottom: 4 }}>
                            ↓ {result.platform}
                        </div>
                        <div style={{ fontSize: 11, color: theme.textDim }}>
                            Agent binary - SHA256 verified
                        </div>
                    </a>
                )}
            </div>

            {/* Install instructions */}
            <div
                style={{
                    background: theme.surface,
                    border: `1px solid ${theme.border}`,
                    padding: 16,
                }}
            >
                <div
                    style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        marginBottom: 12,
                    }}
                >
                    <div
                        style={{
                            fontSize: 10,
                            fontFamily: theme.font,
                            color: theme.textDim,
                            letterSpacing: "0.05em",
                            textTransform: "uppercase",
                        }}
                    >
                        Install Steps ({result.install.type})
                    </div>
                    <CopyButton text={result.install.steps} label="COPY ALL" />
                </div>
                <pre
                    style={{
                        margin: 0,
                        padding: 12,
                        background: theme.bg,
                        border: `1px solid ${theme.border}`,
                        fontFamily: theme.font,
                        fontSize: 12,
                        color: theme.textMuted,
                        lineHeight: 1.6,
                        overflowX: "auto",
                        whiteSpace: "pre-wrap",
                        wordBreak: "break-word",
                    }}
                >
                    {result.install.steps}
                </pre>
            </div>

            {/* Service file content */}
            {result.install.content && (
                <div
                    style={{
                        background: theme.surface,
                        border: `1px solid ${theme.border}`,
                        padding: 16,
                    }}
                >
                    <div
                        style={{
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "space-between",
                            marginBottom: 12,
                        }}
                    >
                        <div
                            style={{
                                fontSize: 10,
                                fontFamily: theme.font,
                                color: theme.textDim,
                                letterSpacing: "0.05em",
                                textTransform: "uppercase",
                            }}
                        >
                            Service File
                        </div>
                        <CopyButton text={result.install.content} />
                    </div>
                    <pre
                        style={{
                            margin: 0,
                            padding: 12,
                            background: theme.bg,
                            border: `1px solid ${theme.border}`,
                            fontFamily: theme.font,
                            fontSize: 12,
                            color: theme.textMuted,
                            lineHeight: 1.6,
                            overflowX: "auto",
                            whiteSpace: "pre-wrap",
                        }}
                    >
                        {result.install.content}
                    </pre>
                </div>
            )}
        </div>
    );
}

export function Admin() {
    const [platforms, setPlatforms] = useState<PlatformInfo[]>([]);
    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [provisioning, setProvisioning] = useState(false);
    const [result, setResult] = useState<ProvisionResponse | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        api
            .platforms()
            .then(setPlatforms)
            .catch((err) =>
                setError(err instanceof Error ? err.message : "Failed to load platforms")
            )
            .finally(() => setLoading(false));

    }, []);

    const handleProvision = async () => {
        if (!selectedPlatform) return;

        setProvisioning(true);
        setError(null);
        try {
            const resp = await api.provision(selectedPlatform);
            setResult(resp);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Provisioning failed");
        } finally {
            setProvisioning(false);
        }
    };

    const handleBack = () => {
        setResult(null);
        setSelectedPlatform(null);
        setError(null);
    };

    if (loading) {
        return (
            <div
                style={{
                    padding: 24,
                    fontFamily: theme.font,
                    color: theme.textMuted,
                }}
            >
                Loading platforms...
            </div>
        );
    }

    if (result) {
        return (
            <div style={{ padding: 24, maxWidth: 720 }}>
                <ProvisionResult result={result} onBack={handleBack} />
            </div>
        );
    }

    return (
        <div style={{ padding: 24, maxWidth: 720 }}>
            <div
                style={{
                    fontSize: 15,
                    fontFamily: theme.font,
                    fontWeight: 600,
                    color: theme.text,
                    marginBottom: 4,
                }}
            >
                Register Agent
            </div>
            <div
                style={{
                    fontSize: 12,
                    fontFamily: theme.font,
                    color: theme.textMuted,
                    marginBottom: 24,
                }}
            >
                Select a platform to generate a one-time registration token, config file, and install instructions.
            </div>

            {/* Platform selector */}
            <div
                style={{
                    background: theme.surface,
                    border: `1px solid ${theme.border}`,
                    padding: 20,
                    marginBottom: 16,
                }}
            >
                <div
                    style={{
                        fontSize: 10,
                        fontFamily: theme.font,
                        color: theme.textDim,
                        letterSpacing: "0.05em",
                        textTransform: "uppercase",
                        marginBottom: 12,
                    }}
                >
                    Platform
                </div>

                {platforms.length > 0 ? (
                    <PlatformSelector
                        platforms={platforms}
                        selected={selectedPlatform}
                        onSelect={setSelectedPlatform}
                    />
                ) : (
                    <div
                        style={{
                            fontSize: 12,
                            fontFamily: theme.font,
                            color: theme.textDim,
                            padding: "12px 0",
                        }}
                    >
                        No pre-build binaries available. Run{" "}
                        <code
                            style={{
                                background: theme.bg,
                                padding: "2px 6px",
                                fontSize: 11,
                            }}
                        >
                            make release
                        </code>{" "}
                        to build agent binaries, or provision without a binary download.
                    </div>
                )}

                {/* Show "provision without binary" option when no releases */}
                {platforms.length === 0 && (
                    <ManualPlatformSelector
                        selected={selectedPlatform}
                        onSelect={setSelectedPlatform}
                    />
                )}
            </div>

            {/* Error */}
            {error && (
                <div
                    style={{
                        padding: 12,
                        marginBottom: 16,
                        background: theme.surface,
                        border: `1px solid ${theme.danger}`,
                        fontSize: 12,
                        fontFamily: theme.font,
                        color: theme.danger,
                    }}
                >
                    {error}
                </div>
            )}

            {/* Provision button */}
            <button
                onClick={handleProvision}
                disabled={!selectedPlatform || provisioning}
                style={{
                    padding: "10px 24px",
                    fontSize: 13,
                    fontFamily: theme.font,
                    fontWeight: 500,
                    color: "#fff",
                    background:
                        !selectedPlatform || provisioning ? theme.accentDim : theme.accent,
                    border: "none",
                    cursor:
                        !selectedPlatform || provisioning ? "not-allowed" : "pointer",
                    letterSpacing: "0.02em",
                    opacity: !selectedPlatform ? 0.5 : 1,
                }}
            >
                {provisioning ? "PROVISIONING..." : "PROVISION AGENT" }
            </button>
        </div>
    );
}

// Fallback selector when no pre-built binaries exist
function ManualPlatformSelector({
    selected,
    onSelect,
}: {
    selected: string | null;
    onSelect: (filename: string) => void;
}) {
    const allPlatforms: PlatformInfo[] = [
        { os: "linux", arch: "amd64", label: "Linux (x86_64)", filename: "spectra-agent-linux-amd64" },
        { os: "linux", arch: "arm64", label: "Linux (arm64)", filename: "spectra-agent-linux-arm64" },
        { os: "linux", arch: "arm", variant: "armv6", label: "Raspberry Pi Zero/1 (armv6)", filename: "spectra-agent-linux-armv6" },
        { os: "linux", arch: "arm", variant: "armv7", label: "Raspberry Pi 2/3/4 (armv7)", filename: "spectra-agent-linux-armv7" },
        { os: "freebsd", arch: "amd64", label: "FreeBSD (x86_64)", filename: "spectra-agent-freebsd-amd64" },
        { os: "darwin", arch: "amd64", label: "macOS (Intel)", filename: "spectra-agent-darwin-amd64" },
        { os: "darwin", arch: "arm64", label: "macOS (Apple Silicon)", filename: "spectra-agent-darwin-arm64" },
        { os: "windows", arch: "amd64", label: "Windows (x86_64)", filename: "spectra-agent-windows-amd64.exe" },
    ];

    return (
        <div style={{ marginTop: 12 }}>
            <div
                style={{
                    fontSize: 10,
                    fontFamily: theme.font,
                    color: theme.textDim,
                    letterSpacing: "0.05em",
                    textTransform: "uppercase",
                    marginBottom: 8,
                }}
            >
                Provision without binary
            </div>
            <PlatformSelector
                platforms={allPlatforms}
                selected={selected}
                onSelect={onSelect}
            />
        </div>
    );
}