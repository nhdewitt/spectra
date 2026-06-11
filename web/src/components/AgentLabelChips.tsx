import { useEffect, useId, useState } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { LabelChip } from "./LabelChip";
import type { AgentLabel, LabelKey } from "../types";

const btnStyle: React.CSSProperties = {
    padding: "2px 8px",
    fontSize: 11,
    fontFamily: themeVars.font,
    color: themeVars.text,
    background: themeVars.accentDim,
    border: `1px solid ${themeVars.accent}`,
    cursor: "pointer",
};
 
const inputStyle: React.CSSProperties = {
    padding: "2px 6px",
    fontSize: 11,
    fontFamily: themeVars.font,
    color: themeVars.text,
    background: themeVars.surface,
    border: `1px solid ${themeVars.border}`,
    width: 140,
};
 
interface LabelAdderProps {
    agentId: string;
    existingKeys: LabelKey[];
    onAdded: (l: AgentLabel) => void;
    onCancel: () => void;
}
 
function LabelAdder({ agentId, existingKeys, onAdded, onCancel }: LabelAdderProps) {
    const keysListId = useId();
    const valuesListId = useId();
    const [keyInput, setKeyInput] = useState("");
    const [valueInput, setValueInput] = useState("");
    const [values, setValues] = useState<string[]>([]);
    const [error, setError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);
 
    // Fetch values for autocomplete when key matches an existing user key.
    useEffect(() => {
        const k = keyInput.trim();
        if (!k || !existingKeys.some((ek) => ek.key === k && ek.source === "user")) {
            setValues([]);
            return;
        }
        const timer = setTimeout(() => {
            api.labelValues(k).then(setValues).catch(() => setValues([]));
        }, 150);
        return () => clearTimeout(timer);
    }, [keyInput, existingKeys]);
 
    const handleSubmit = async () => {
        const k = keyInput.trim();
        const v = valueInput.trim();
        if (!k || !v) return;
        setSubmitting(true);
        setError(null);
        try {
            const newLabel = await api.setAgentLabel(agentId, k, v);
            onAdded(newLabel);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to add label");
            setSubmitting(false);
        }
    };
 
    const userKeys = existingKeys.filter((k) => k.source === "user");
    const canSubmit = !submitting && keyInput.trim().length > 0 && valueInput.trim().length > 0;
 
    return (
        <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
            <input
                list={keysListId}
                value={keyInput}
                onChange={(e) => {
                    setKeyInput(e.target.value);
                    setError(null);
                }}
                onKeyDown={(e) => {
                    if (e.key === "Escape") onCancel();
                }}
                placeholder="key (e.g. env)"
                style={inputStyle}
                autoFocus
            />
            <datalist id={keysListId}>
                {userKeys.map((k) => (
                    <option key={k.key} value={k.key} />
                ))}
            </datalist>
            <span style={{ color: themeVars.textDim, fontSize: 11 }}>=</span>
            <input
                list={valuesListId}
                value={valueInput}
                onChange={(e) => {
                    setValueInput(e.target.value);
                    setError(null);
                }}
                onKeyDown={(e) => {
                    if (e.key === "Enter" && canSubmit) handleSubmit();
                    if (e.key === "Escape") onCancel();
                }}
                placeholder="value (e.g. prod)"
                style={inputStyle}
            />
            <datalist id={valuesListId}>
                {values.map((v) => (
                    <option key={v} value={v} />
                ))}
            </datalist>
            <button
                onClick={handleSubmit}
                disabled={!canSubmit}
                style={{
                    ...btnStyle,
                    opacity: canSubmit ? 1 : 0.5,
                    cursor: canSubmit ? "pointer" : "default",
                }}
            >
                Add
            </button>
            <button
                onClick={onCancel}
                style={{
                    ...btnStyle,
                    color: themeVars.textMuted,
                    background: "transparent",
                    borderColor: themeVars.border,
                }}
            >
                Cancel
            </button>
            {error && (
                <span style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.danger }}>
                    {error}
                </span>
            )}
        </div>
    );
}
 
interface AgentLabelChipsProps {
    agentId: string;
    isAdmin: boolean;
}
 
export function AgentLabelChips({ agentId, isAdmin }: AgentLabelChipsProps) {
    const [labels, setLabels] = useState<AgentLabel[]>([]);
    const [keys, setKeys] = useState<LabelKey[]>([]);
    const [loading, setLoading] = useState(true);
    const [adding, setAdding] = useState(false);
 
    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        api.agentLabels(agentId)
            .then((data) => {
                if (!cancelled) setLabels(data);
            })
            .catch(() => {})
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [agentId]);
 
    // Keys for autocomplete; only admins need them.
    useEffect(() => {
        if (!isAdmin) return;
        let cancelled = false;
        api.labelKeys()
            .then((data) => {
                if (!cancelled) setKeys(data);
            })
            .catch(() => {});
        return () => {
            cancelled = true;
        };
    }, [isAdmin, labels]); // refresh after labels change (new key may exist)
 
    const sortLabels = (items: AgentLabel[]) =>
        [...items].sort((a, b) => {
            if (a.source !== b.source) return a.source === "auto" ? -1 : 1;
            return a.key.localeCompare(b.key);
        });
 
    const handleAdded = (label: AgentLabel) => {
        setLabels((prev) => {
            const idx = prev.findIndex((l) => l.key === label.key);
            const next = idx >= 0
                ? [...prev.slice(0, idx), label, ...prev.slice(idx + 1)]
                : [...prev, label];
            return sortLabels(next);
        });
        setAdding(false);
    };
 
    const handleRemove = async (key: string) => {
        try {
            await api.deleteAgentLabel(agentId, key);
            setLabels((prev) => prev.filter((l) => l.key !== key));
        } catch {
            // Best-effort: a failed delete leaves the UI as-is.
        }
    };
 
    if (loading) return null;
    if (!isAdmin && labels.length === 0) return null;
 
    return (
        <div
            style={{
                display: "flex",
                flexDirection: "column",
                gap: 4,
                flex: "1 1 auto",
                minWidth: 0,
            }}
        >
            <div
                style={{
                    fontSize: 10,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                }}
            >
                Labels
            </div>
            <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
                {labels.map((l) => (
                    <LabelChip
                        key={l.key}
                        label={l}
                        onDelete={
                            isAdmin && l.source === "user"
                                ? () => handleRemove(l.key)
                                : null
                        }
                    />
                ))}
                {isAdmin && !adding && (
                    <button
                        onClick={() => setAdding(true)}
                        title="Add label"
                        style={{
                            ...btnStyle,
                            background: "transparent",
                            color: themeVars.textMuted,
                            borderColor: themeVars.border,
                            borderStyle: "dashed",
                        }}
                    >
                        + Add
                    </button>
                )}
            </div>
            {isAdmin && adding && (
                <div style={{ marginTop: 6 }}>
                    <LabelAdder
                        agentId={agentId}
                        existingKeys={keys}
                        onAdded={handleAdded}
                        onCancel={() => setAdding(false)}
                    />
                </div>
            )}
        </div>
    );
}
