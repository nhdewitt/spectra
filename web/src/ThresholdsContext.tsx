import React from "react";
import type { Thresholds } from "./types";
import { DEFAULT_THRESHOLDS } from "./types";

// ThresholdsContext supplies the global status thresholds to any component that
// classifies agent status. Defaults to DEFAULT_THRESHOLDS so consumers rendered
// before the fetch resolves (or in tests) still work.
const ThresholdsContext = React.createContext<Thresholds>(DEFAULT_THRESHOLDS);

export function ThresholdsProvider({
	value,
	children,
}: {
	value: Thresholds;
	children: React.ReactNode;
}) {
	return <ThresholdsContext.Provider value={value}>{children}</ThresholdsContext.Provider>
}

export function useThresholds(): Thresholds {
	return React.useContext(ThresholdsContext);
}