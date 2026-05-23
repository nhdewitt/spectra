import React, { useState, useEffect } from "react";
import { themeVars } from "../theme";
import { SpectraLogo } from "./SpectraLogo";
import { copyToClipboard } from "../utils";

// StatBlock

interface StatBlockProps {
    label: string;
    value: string | null;
    unit?: string;
    color?: string;
    copyable?: boolean;
    small?: boolean;
}

export function StatBlock({ label, value, unit, color, copyable, small }: StatBlockProps) {
    const [copied, setCopied] = useState(false);

    const handleClick = () => {
        if (!copyable || !value) return;
        copyToClipboard(value);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <div style={{ minWidth: 64 }}>
            <div
                style={{
                    fontSize: 11,
                    color: copied ? themeVars.ok : themeVars.textMuted,
                    fontFamily: themeVars.font,
                    letterSpacing: "0.04em",
                    textTransform: "uppercase",
                    marginBottom: 4,
                    transition: "color 0.2s ease",
                }}
            >
                {copied ? "Copied!" : (
                    <>
                        {label}
                        {copyable && (
                            <span style={{ textTransform: "none", fontSize: 9, marginLeft: 4 }}>
                                (click to copy)
                            </span>
                        )}
                    </>
                )}
            </div>
            <div style={{ display: "flex", alignItems: "baseline", gap: 3 }}>
                <span
                    onClick={handleClick}
                    style={{
                        fontSize: small ? 11 : 18,
                        fontWeight: small ? 500 : 600,
                        fontFamily: themeVars.font,
                        color: color ?? themeVars.text,
                        cursor: copyable ? "pointer" : "default",
                    }}
                >
                    {value ?? "—"}
                </span>
                {unit && (
                    <span
                        style={{
                            fontSize: 11,
                            color: themeVars.textDim,
                            fontFamily: themeVars.font,
                        }}
                    >
                        {unit}
                    </span>
                )}
            </div>
        </div>
    );
}

// DetailRow

export function DetailRow({
    label,
    value,
}: {
    label: string;
    value: string | number | null | undefined;
}) {
    return (
        <div style={{ display: "flex", justifyContent: "space-between" }}>
            <span style={{ color: themeVars.textDim }}>{label}</span>
            <span style={{ color: themeVars.textMuted }}>{value ?? "—"}</span>
        </div>
    );
}

// ViewToggle

export type ViewMode = "tiles" | "list";

export function ViewToggle({
    mode,
    onChange,
}: {
    mode: ViewMode;
    onChange: (m: ViewMode) => void;
}) {
    return (
        <div style={{ display: "flex", gap: 2 }}>
            {(["tiles", "list"] as const).map((m) => (
                <button
                    key={m}
                    onClick={() => onChange(m)}
                    style={{
                        padding: "4px 10px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: mode === m ? themeVars.text : themeVars.textMuted,
                        background: mode === m ? themeVars.accentDim : "transparent",
                        border: `1px solid ${mode === m ? themeVars.accent : themeVars.border}`,
                        cursor: "pointer",
                        textTransform: "uppercase",
                        letterSpacing: "0.03em",
                    }}
                >
                    {m === "tiles" ? "⊞" : "☰"} {m}
                </button>
            ))}
        </div>
    );
}

// LoadingText

export function LoadingText() {
    const [dots, setDots] = useState(0);

    useEffect(() => {
        const id = setInterval(() => setDots((d) => (d + 1) % 4), 400);
        return () => clearInterval(id);
    }, []);

    return (
        <div
            style={{
                padding: 24,
                color: themeVars.textMuted,
                fontFamily: themeVars.font,
            }}
        >
            Loading{".".repeat(dots)}
        </div>
    );
}

export function LoadingSpinner() {
    return (
        <div style={{ display: "flex", justifyContent: "center", padding: 24 }}>
            <SpectraLogo size={40} animate />
        </div>
    );
}

export function MetricSelector({
    label,
    options,
    value,
    onChange,
}: {
    label: string;
    options: string[];
    value: string;
    onChange: (v: string) => void;
}) {
    if (options.length <= 1) return null;

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                marginBottom: 8,
            }}
        >
            <span
                style={{
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                }}
            >
                {label}:
            </span>
            <select
                value={value}
                onChange={(e) => onChange(e.target.value)}
                style={{
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.text,
                    background: themeVars.surface,
                    border: `1px solid ${themeVars.border}`,
                    padding: "3px 6px",
                    cursor: "pointer",
                }}
            >
                {options.map((o) => (
                    <option key={o} value={o}>
                        {o}
                    </option>
                ))}
            </select>
        </div>
    );
}

export function InstructionBlock({
    title,
    steps,
    onClose,
    footer,
}: {
    title: string;
    steps: string;
    onClose: () => void;
    footer?: React.ReactNode;
}) {
    const [copied, setCopied] = useState(false);

    const handleCopy = () => {
        // Extract the commands and skip the numbered headers
        const commands = steps
            .split("\n")
            .filter((line) => {
                const trimmed = line.trim();
                return trimmed && !/^\d+\.\s/.test(trimmed);
            })
            .join("\n");
        copyToClipboard(commands);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
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
                zIndex: 200,
            }}
            onClick={(e) => {
                if (e.target === e.currentTarget) onClose();
            }}
        >
            <div
                style={{
                    background: themeVars.bg,
                    border: `1px solid ${themeVars.border}`,
                    width: "90%",
                    maxWidth: 700,
                    maxHeight: "85vh",
                    display: "flex",
                    flexDirection: "column",
                }}
            >
                <div
                    style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "center",
                        padding: "12px 16px",
                        borderBottom: `1px solid ${themeVars.border}`,
                    }}
                >
                    <span
                        style={{
                            fontFamily: themeVars.font,
                            fontSize: 13,
                            fontWeight: 600,
                            color: themeVars.text,
                        }}
                    >
                        {title}
                    </span>
                    <div style={{ display: "flex", gap: 8 }}>
                        <button
                            onClick={handleCopy}
                            style={{
                                padding: "4px 10px",
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: copied ? themeVars.ok : themeVars.textMuted,
                                background: "transparent",
                                border: `1px solid ${themeVars.border}`,
                                cursor: "pointer",
                            }}
                        >
                            {copied ? "Copied" : "Copy commands"}
                        </button>
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
                </div>
                <pre
                    style={{
                        flex: 1,
                        overflowY: "auto",
                        margin: 0,
                        padding: 16,
                        fontFamily: themeVars.font,
                        fontSize: 12,
                        lineHeight: 1.6,
                        color: themeVars.text,
                        whiteSpace: "pre-wrap",
                        wordBreak: "break-word",
                    }}
                >
                    {steps.split("\n").map((line, i) => {
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
                                    fontWeight: isStepHeader  ? 600 : 400,
                                }}
                            >
                                {line}
                                {"\n"}
                            </span>
                        );
                    })}
                </pre>
                {footer}
            </div>
        </div>
    );
}

export const tableHeaderStyle: React.CSSProperties = {
    padding: "6px 10px",
    fontSize: 11,
    fontFamily: themeVars.font,
    color: themeVars.textDim,
    textTransform: "uppercase",
    letterSpacing: "0.04em",
    textAlign: "left",
    borderBottom: `1px solid ${themeVars.border}`,
    whiteSpace: "nowrap",
};

export const tableCellStyle: React.CSSProperties = {
    padding: "5px 10px",
    fontSize: 12,
    fontFamily: themeVars.font,
    color: themeVars.text,
    borderBottom: `1px solid ${themeVars.border}`,
};

export const tableMutedCellStyle: React.CSSProperties = {
    ...tableCellStyle,
    color: themeVars.textMuted,
};