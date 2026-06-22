import { useState } from "react";
import { themeVars } from "../theme";
import type { User } from "../types";
import { AlertChannels } from "./AlertChannels";
import { AlertRules } from "./AlertRules";
import { ActiveAlerts } from "./ActiveAlerts";

type AlertTab = "active" | "rules" | "channels";

const TABS: { key: AlertTab; label: string }[] = [
	{ key: "active", label: "Active" },
	{ key: "rules", label: "Rules" },
	{ key: "channels", label: "Channels" },
];

interface AlertsProps {
	user: User;
}

export function Alerts({ user }: AlertsProps) {
	const [tab, setTab] = useState<AlertTab>("active");

	const tabStyle = (active: boolean): React.CSSProperties => ({
		padding: "8px 16px",
		fontSize: 12,
		fontFamily: themeVars.font,
		color: active ? themeVars.text : themeVars.textMuted,
		background: "transparent",
		border: "none",
		borderBottom: `2px solid ${active ? themeVars.accent : "transparent"}`,
		cursor: "pointer",
		letterSpacing: "0.02em",
	});

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
				Alerts
			</div>
 
			{/* Tab bar */}
			<div
				style={{
					display: "flex",
					gap: 4,
					marginBottom: 20,
					borderBottom: `1px solid ${themeVars.border}`,
				}}
			>
				{TABS.map((t) => (
					<button key={t.key} onClick={() => setTab(t.key)} style={tabStyle(tab === t.key)}>
						{t.label}
					</button>
				))}
			</div>
 
			{/* Panel */}
			{tab === "active" ? (
				<ActiveAlerts />
			) : tab === "rules" ? (
				<AlertRules user={user} />
			) : tab === "channels" ? (
				<AlertChannels />
			) : null}
		</div>
	);
}