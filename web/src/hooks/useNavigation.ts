import { useState, useEffect, useCallback, useRef } from "react";
import type { Page, OverviewAgent } from "../types";

interface NavState {
	page: Page;
	agent: OverviewAgent | null;
}

export interface Navigation {
	page: Page;
	selectedAgent: OverviewAgent | null;
	navigate: (page: Page) => void;
	selectAgent: (agent: OverviewAgent) => void;
	back: () => void;
	reset: () => void;
}

const initial: NavState = { page: "overview", agent: null };

/**
 * In-app navigation backed by browser history, so the back button
 * does what the in-app back does.
 * 
 * The agent rides in the history entry rather than the URL. The URL
 * would only carry an id, and there is no endpoint returning an
 * OverviewAgent for one agent (api.agent gives an Agent, which has
 * no metric rollups). Carrying the object sidesteps that entirely,
 * at the cost of the address bad never changing (no deep links, and
 * a refresh lands on the overview).
 */
export function useNavigation(): Navigation {
	const [state, setState] = useState<NavState>(initial);

	const applyingPop = useRef(false);

	useEffect(() => {
		// seed the current entry so the first push has something to come back to
		if (window.history.state?.spectraNav == null) {
			window.history.replaceState({ spectraNav: initial }, "");
		}

		const onPop = (ev: PopStateEvent) => {
			applyingPop.current = true;
			setState((ev.state?.spectraNav as NavState | undefined) ?? initial);
			applyingPop.current = false;
		};

		window.addEventListener("popstate", onPop);
		return () => window.removeEventListener("popstate", onPop);
	}, []);

	const navigate = useCallback(
		(page: Page) => {
			setState((prev) => {
				const agent = page === "detail" || page === "diagnostics" ? prev.agent : null;
				const next = { page, agent };
				if (!applyingPop.current) {
					window.history.pushState({ spectraNav: next }, "");
				}
				return next;
			});
		},
		[],
	);

	const selectAgent = useCallback((agent: OverviewAgent) => {
		setState((prev) => {
			const page: Page = prev.page === "diagnostics" ? "diagnostics" : "detail";
			const next = { page, agent };
			if (!applyingPop.current) {
				window.history.pushState({ spectraNav: next }, "");
			}
			return next;
		});
	}, []);

	const back = useCallback(() => {
		window.history.back();
	}, []);

	// Signing out - replace rather than push so the back button can't return to the previous
	// account's view.
	const reset = useCallback(() => {
		setState(initial);
		window.history.replaceState({ spectraNav: initial }, "");
	}, []);

	return {
		page: state.page,
		selectedAgent: state.agent,
		navigate,
		selectAgent,
		back,
		reset,
	};
}