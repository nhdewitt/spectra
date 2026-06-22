import { themeVars } from "../theme";
import type { User } from "../types";

interface AlertRulesProps {
	user: User;
}

// Placeholder — full rules management lands in a following commit.
export function AlertRules({ user: _user }: AlertRulesProps) {
	return (
		<div
			style={{
				textAlign: "center",
				padding: "40px 0",
				fontFamily: themeVars.font,
				color: themeVars.textDim,
				fontSize: 13,
			}}
		>
			Rules management coming soon.
		</div>
	);
}