import { useState } from "react";
import { themeVars } from "../theme";
import type { User } from "../types";
import { SMTPSettings } from "./SMTPSettings";
import { ThresholdsSettings } from "./ThresholdsSettings";

const TABS = ["thresholds", "email"] as const;

// ServerSettings holds configuration that affects every user and every agent,
// unlike Settings which is per-user. Superadmin only.
export function ServerSettings({ user }: { user: User }) {
	const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>("thresholds");

	if (user.role !== "superadmin") {
		return (
			<div style={{ padding: 24, fontFamily: themeVars.font, fontSize: 12, color: themeVars.textMuted }}>
				Server settings require the superadmin role.
			</div>
		);
	}

	return (
		<div style={{ padding: 24, maxWidth: 700 }}>
			<div
				style={{
					fontFamily: themeVars.font,
					fontSize: 18,
					fontWeight: 600,
					color: themeVars.text,
					marginBottom: 4,
				}}
			>
				Server Settings
			</div>
			<div
				style={{
					fontFamily: themeVars.font,
					fontSize: 11,
					color: themeVars.textDim,
					marginBottom: 20,
				}}
			>
				Applies to all users and every agent in the fleet.
			</div>

			<div
				style={{
					display: "flex",
					gap: 4,
					marginBottom: 20,
					borderBottom: `1px solid ${themeVars.border}`,
					paddingBottom: 8,
				}}
			>
				{TABS.map((tab) => (
					<button
						key={tab}
						onClick={() => setActiveTab(tab)}
						style={{
							padding: "6px 14px",
							fontSize: 12,
							fontFamily: themeVars.font,
							color: activeTab === tab ? themeVars.text : themeVars.textMuted,
							background: activeTab === tab ? themeVars.accentDim : "transparent",
							border: "none",
							cursor: "pointer",
							textTransform: "uppercase",
							letterSpacing: "0.03em",
						}}
					>
						{tab}
					</button>
				))}
			</div>

			{activeTab === "thresholds" && <ThresholdsSettings />}
			{activeTab === "email" && <SMTPSettings />}
		</div>
	);
}