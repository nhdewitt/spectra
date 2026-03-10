import { useState, useEffect } from "react";
import { themeVars } from "../theme";

// StatBlock

interface StatBlockProps {
    label: string;
    value: string | null;
    unit?: string;
    color?: string;
}

export function StatBlock({ label, value, unit, color }: StatBlockProps) {
    return (
        <div style={{ minWidth: 64 }}>
            <div
                style={{
                    fontSize: 11,
                    color: themeVars.textMuted,
                    fontFamily: themeVars.font,
                    letterSpacing: "0.04em",
                    textTransform: "uppercase",
                    marginBottom: 4,
                }}
            >
                {label}
            </div>
            <div style={{ display: "flex", alignItems: "baseline", gap: 3 }}>
                <span
                    style={{
                        fontSize: 18,
                        fontWeight: 600,
                        fontFamily: themeVars.font,
                        color: color ?? themeVars.text,
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