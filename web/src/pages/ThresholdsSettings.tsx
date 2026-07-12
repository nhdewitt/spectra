import { useState, useEffect } from "react";
import { themeVars } from "../theme";
import { LoadingSpinner } from "../components";
import { api } from "../api";
import type { Thresholds } from "../types";
import { DEFAULT_THRESHOLDS } from "../types";

// ThresholdsSettings is the admin editor for the global status thresholds.
// Loads the current values, lets an admin edit them, and saves via
// api.updateThresholds. Non-admins should not be shown this (gate at
// the Settings page level).
export function ThresholdsSettings() {
	const [t, setT] = useState<Thresholds>(DEFAULT_THRESHOLDS);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [saved, setSaved] = useState(false);

	useEffect(() => {
		api.thresholds()
			.then((v) => setT(v))
			.catch(() => {})
			.finally(() => setLoading(false));
	}, []);

	const setField = (key: keyof Thresholds, value: number) => {
		setT((prev) => ({ ...prev, [key]: value }));
		setSaved(false);
	};

	const save = async () => {
		setSaving(true);
		setError(null);
		try {
			const updated = await api.updateThresholds(t);
			setT(updated);
			setSaved(true);
		} catch (e) {
			setError(e instanceof Error ? e.message : "Failed to save thresholds");
		} finally {
			setSaving(false);
		}
	};

	if (loading) {
		return <LoadingSpinner />;
	}

	const metricRows: { label: string; warn: keyof Thresholds; crit: keyof Thresholds; unit: string }[] = [
		{ label: "CPU", warn: "cpu_warn", crit: "cpu_crit", unit: "%" },
		{ label: "Memory", warn: "mem_warn", crit: "mem_crit", unit: "%" },
		{ label: "Disk", warn: "disk_warn", crit: "disk_crit", unit: "%" },
		{ label: "Temperature", warn: "temp_warn", crit: "temp_crit", unit: "°C" },		
	];

	const labelStyle: React.CSSProperties = {
		fontSize: 11,
		fontFamily: themeVars.font,
		color: themeVars.textMuted,
		textTransform: "uppercase",
		letterSpacing: "0.04em",
	};

	const inputStyle: React.CSSProperties = {
		width: 70,
		padding: "5px 8px",
		fontSize: 12,
		fontFamily: themeVars.font,
		color: themeVars.text,
		background: themeVars.surface,
		border: `1px solid ${themeVars.border}`,
	};

	return (
		<div style={{ maxWidth: 480 }}>
			<div style={{ display: "grid", gridTemplateColumns: "100px 1fr 1fr", gap: "10px 16px", alignItems: "center" }}>
				<div />
				<div style={labelStyle}>Warn</div>
				<div style={labelStyle}>Critical</div>
 
				{metricRows.map((row) => (
					<Row key={row.label} label={row.label}>
						<NumberInput value={t[row.warn]} unit={row.unit} style={inputStyle}
							onChange={(v) => setField(row.warn, v)} />
						<NumberInput value={t[row.crit]} unit={row.unit} style={inputStyle}
							onChange={(v) => setField(row.crit, v)} />
					</Row>
				))}
			</div>
 
			<div style={{ height: 1, background: themeVars.border, margin: "20px 0" }} />
 
			<div style={{ display: "grid", gridTemplateColumns: "160px 1fr", gap: "10px 16px", alignItems: "center" }}>
				<div style={labelStyle}>Stale after</div>
				<NumberInput value={t.stale_seconds} unit="s" style={inputStyle}
					onChange={(v) => setField("stale_seconds", v)} />
				<div style={labelStyle}>Offline after</div>
				<NumberInput value={t.offline_seconds} unit="s" style={inputStyle}
					onChange={(v) => setField("offline_seconds", v)} />
			</div>
 
			<div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 20 }}>
				<button
					onClick={save}
					disabled={saving}
					style={{
						padding: "6px 14px",
						fontSize: 12,
						fontFamily: themeVars.font,
						color: themeVars.text,
						background: themeVars.accentDim,
						border: `1px solid ${themeVars.accent}`,
						cursor: saving ? "default" : "pointer",
						opacity: saving ? 0.6 : 1,
						textTransform: "uppercase",
						letterSpacing: "0.03em",
					}}
				>
					{saving ? "Saving..." : "Save thresholds"}
				</button>
				{saved && <span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.ok }}>Saved</span>}
				{error && <span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.danger }}>{error}</span>}
			</div>
		</div>
	);
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
	return (
		<>
			<div style={{ fontSize: 13, fontFamily: themeVars.font, color: themeVars.text }}>{label}</div>
			{children}
		</>
	);
}

function NumberInput({
	value,
	unit,
	style,
	onChange,
}: {
	value: number;
	unit: string;
	style: React.CSSProperties;
	onChange: (v: number) => void;
}) {
	return (
		<div style={{ display: "flex", alignItems: "center", gap: 6 }}>
			<input
				type="number"
				value={value}
				onChange={(e) => onChange(Number(e.target.value))}
				style={style}
			/>
			<span style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.textDim }}>{unit}</span>
		</div>
	);	
}