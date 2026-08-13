import { useEffect, useState } from "react";
import { api } from "../api";
import type { OverviewAgent } from "../types";

const DEBOUNCE_MS = 250;
const RESULT_SIZE = 20;

/**
 * Debounced, server-side agent search backed by /overview/page's existing
 * `search` param. Always requests count:false and a small page size - 
 * this is a picker, not a full listing, so there's no need for pagination
 * or totals. Empty query returns the first RESULT_SIZE agents alphabetically,
 * giving pickers something to show before the user types.
 */
export function useAgentSearch(query: string) {
	const [agents, setAgents] = useState<OverviewAgent[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		let cancelled = false;
		setLoading(true);

		const t = setTimeout(() => {
			api.overviewPage({
				search: query || undefined,
				size: RESULT_SIZE,
				sort: "hostname",
				order: "asc",
				count: false,
			})
				.then((res) => {
					if (cancelled) return;
					setAgents(res.agents);
					setError(null);
				})
				.catch((err) => {
					if (cancelled) return;
					setError(err instanceof Error ? err.message : "Failed to search agents");
				})
				.finally(() => {
					if (!cancelled) setLoading(false);
				})
		}, DEBOUNCE_MS);

		return () => {
			cancelled = true;
			clearTimeout(t);
		};
	}, [query]);

	return { agents, loading, error };
}