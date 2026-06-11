import { themeVars } from "../theme";
import type { AgentLabel } from "../types";

function hexToRgba(hex: string, alpha: number): string {
	const r = parseInt(hex.slice(1, 3), 16);
	const g = parseInt(hex.slice(3, 5), 16);
	const b = parseInt(hex.slice(5, 7), 16);
	return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

const PALETTE = [
    "#4ecdc4",
    "#ff6b9d",
    "#feca57",
    "#a29bfe",
    "#ff9f43",
    "#48dbfb",
    "#1dd1a1",
    "#ee5a6f",
    "#5f27cd",
    "#00d2d3",
    "#54a0ff",
    "#10ac84",
    "#f368e0",
    "#ff9ff3",
    "#ffa502",
    "#7bed9f",
];

function colorForLabel(key: string, value: string): string {
	const input = `${key}=${value}`
	let hash = 0;
	for (let i = 0; i < key.length; i++) {
		hash = ((hash << 5) - hash) + input.charCodeAt(i);
		hash |= 0;
	}
	return PALETTE[Math.abs(hash) % PALETTE.length]!;
}

interface LabelChipProps {
	label: AgentLabel;
	onDelete?: (() => void) | null;
	onPickColor?: (() => void) | null;
}

export function LabelChip({ label, onDelete }: LabelChipProps) {
	const isAuto = label.source === "auto";
	const accent = isAuto ? themeVars.textDim : colorForLabel(label.key, label.value);
	const bg = isAuto ? themeVars.surface : hexToRgba(accent, 0.12);
	const border = isAuto ? themeVars.border : hexToRgba(accent, 0.45);

    return (
        <span
            title={`${label.source} label`}
            style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 4,
                padding: "2px 8px",
                fontSize: 11,
                fontFamily: themeVars.font,
                color: themeVars.text,
                background: bg,
                border: `1px solid ${border}`,
                opacity: isAuto ? 0.85 : 1,
            }}
        >
            <span style={{ color: accent, fontWeight: isAuto ? 400 : 500 }}>
                {label.key}
            </span>
            <span style={{ color: themeVars.textDim }}>=</span>
            {label.value}
            {onDelete && (
                <button
                    onClick={onDelete}
                    title={`Remove ${label.key}`}
                    style={{
                        background: "none",
                        border: "none",
                        color: themeVars.danger,
                        cursor: "pointer",
                        fontSize: 12,
                        padding: 0,
                        lineHeight: 1,
                        marginLeft: 2,
                    }}
                >
                    ×
                </button>
            )}
        </span>
    );
}