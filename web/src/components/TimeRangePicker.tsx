import React, { useState } from "react";
import { themeVars } from "../theme";
import type { TimeRange, RangeSelection } from "../types";

const QUICK_RANGES: TimeRange[] = ["5m", "15m", "1h", "6h", "24h", "7d", "30d"];

/** Format an ISO string to the datetime-local input format (YYYY-MM-DDTHH:MM). */
function toLocalInput(iso: string): string {
    const d = new Date(iso);
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Convert a datetime-local input value to an ISO string. */
function fromLocalInput(value: string): string {
    return new Date(value).toISOString();
}

/** Get a sensible default start for the custom picker (-1h). */
function defaultStart(): string {
    return new Date(Date.now() - 60 * 60 * 1000).toISOString();
}

/** Get a sensible default end for the custom picker (now). */
function defaultEnd(): string {
    return new Date().toISOString();
}

const btnBase: React.CSSProperties = {
    padding: "5px 10px",
    fontSize: 11,
    fontFamily: themeVars.font,
    cursor: "pointer",
    letterSpacing: "0.02em",
    border: "1px solid",
};

export function TimeRangePicker({
    value,
    onChange,
}: {
    value: RangeSelection;
    onChange: (sel: RangeSelection) => void;
}) {
    const [showCustom, setShowCustom] = useState(value.type === "custom");
    const [customStart, setCustomStart] = useState(value.type === "custom" ? value.start : defaultStart());
    const [customEnd, setCustomEnd] = useState(value.type === "custom" ? value.end : defaultEnd());

    const handleQuick = (range: TimeRange) => {
        setShowCustom(false);
        onChange({ type: "quick", range });
    };

    const handleCustomToggle = () => {
        if (!showCustom) {
            // Opening custom - set defaults if coming from quick
            if (value.type === "quick") {
                const end = defaultEnd();
                const start = defaultStart();
                setCustomStart(start);
                setCustomEnd(end);
            }
            setShowCustom(true);
        } else {
            setShowCustom(false);
        }
    };

    const handleApply = () => {
        if (!customStart || !customEnd) return;
        if (new Date(customStart) >= new Date(customEnd)) return;

        onChange({ type: "custom", start: customStart, end: customEnd });
    };

    const isQuick = value.type === "quick";
    const activeRange = isQuick ? value.range : null;

    const maxDate = toLocalInput(new Date().toISOString());

    return (
        <div>
            {/* Quick range buttons + custom toggle */}
            <div style={{ display: "flex", gap: 4, alignItems: "center", flexWrap: "wrap" }}>
                {QUICK_RANGES.map((r) => (
                    <button
                        key={r}
                        onClick={() => handleQuick(r)}
                        style={{
                            ...btnBase,
                            color: activeRange === r ? themeVars.text : themeVars.textMuted,
                            background: activeRange === r ? themeVars.accentDim : "transparent",
                            borderColor: activeRange === r ? themeVars.accent : themeVars.border,
                        }}
                    >
                        {r}
                    </button>
                ))}

                <div
                    style={{
                        width: 1,
                        height: 20,
                        background: themeVars.border,
                        margin: "0 4px",
                    }}
                />

                <button
                    onClick={handleCustomToggle}
                    style={{
                        ...btnBase,
                        color: showCustom || !isQuick ? themeVars.text : themeVars.textMuted,
                        background: showCustom || !isQuick ? themeVars.accentDim : "transparent",
                        borderColor: showCustom || !isQuick ? themeVars.accent : themeVars.border,
                    }}
                >
                    CUSTOM
                </button>

                {/* Show active custom range inline */}
                {!isQuick && !showCustom && (
                    <span
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.textMuted,
                            marginLeft: 4,
                        }}
                    >
                        {new Date(value.start).toLocaleString()} — {new Date(value.end).toLocaleString()}
                    </span>
                )}
            </div>

            {/* Custom date inputs */}
            {showCustom && (
                <div
                    style={{
                        display: "flex",
                        alignItems: "flex-end",
                        gap: 12,
                        marginTop: 10,
                        padding: 12,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                >
                    <div>
                        <label
                            style={{
                                display: "block",
                                fontSize: 10,
                                fontFamily: themeVars.font,
                                color: themeVars.textDim,
                                letterSpacing: "0.05em",
                                textTransform: "uppercase",
                                marginBottom: 4,
                            }}
                        >
                            Start
                        </label>
                        <input
                            type="datetime-local"
                            value={toLocalInput(customStart)}
                            max={maxDate}
                            onChange={(e) => setCustomStart(fromLocalInput(e.target.value))}
                            style={{
                                padding: "6px 8px",
                                background: themeVars.bg,
                                border: `1px solid ${themeVars.border}`,
                                color: themeVars.text,
                                fontSize: 12,
                                fontFamily: themeVars.font,
                                outline: "none",
                                colorScheme: "dark",
                            }}
                        />
                    </div>

                    <div>
                        <label
                            style={{
                                display: "block",
                                fontSize: 10,
                                fontFamily: themeVars.font,
                                color: themeVars.textDim,
                                letterSpacing: "0.05em",
                                textTransform: "uppercase",
                                marginBottom: 4,
                            }}
                        >
                            End
                        </label>
                        <input
                            type="datetime-local"
                            value={toLocalInput(customEnd)}
                            max={maxDate}
                            onChange={(e) => setCustomEnd(fromLocalInput(e.target.value))}
                            style={{
                                padding: "6px 8px",
                                background: themeVars.bg,
                                border: `1px solid ${themeVars.border}`,
                                color: themeVars.text,
                                fontSize: 12,
                                fontFamily: themeVars.font,
                                outline: "none",
                                colorScheme: "dark",
                            }}
                        />
                    </div>

                    <button
                        onClick={handleApply}
                        disabled={!customStart || !customEnd || new Date(customStart) >= new Date(customEnd)}
                        style={{
                            padding: "6px 16px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            fontWeight: 500,
                            color: "#fff",
                            background:
                                !customStart || !customEnd || new Date(customStart) >= new Date(customEnd)
                                    ? themeVars.accentDim
                                    : themeVars.accent,
                            border: "none",
                            cursor:
                                !customStart || !customEnd || new Date(customStart) >= new Date(customEnd)
                                    ? "not-allowed"
                                    : "pointer",
                            letterSpacing: "0.02em",
                        }}
                    >
                        APPLY
                    </button>

                    {/* Validation hint */}
                    {customStart && customEnd && new Date(customStart) >= new Date(customEnd) && (
                        <span
                            style={{
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: themeVars.danger,
                            }}
                        >
                            Start date must fall before end date
                        </span>
                    )}
                </div>
            )}
        </div>
    );
}