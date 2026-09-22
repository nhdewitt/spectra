import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "../api";
import type { User } from "../types";

/** Delay before persisting an edit, so a burst of toggles is one request. */
const persistDelayMs = 500;

export interface StarredAgents {
	starredIds: string[];
	toggleStar: (agentId: string) => void;
}

/**
 * Owns the starred-agent list for the signed-in account.
 * 
 * Each account gets a generation. A userConfig() response that outlives the
 * account that asked for it would otherwise write one user's stars into the
 * next user's state, and hasUserEdited staying true across the switch would
 * make the second account's initial load look like an edit and persist it.
 */
export function useStarredAgents(user: User | null): StarredAgents {
	const [starredIds, setStarredIds] = useState<string[]>([]);
	const [starredLoaded, setStarredLoaded] = useState(false);

	const starredRef = useRef(starredIds);
	starredRef.current = starredIds;
	const hasUserEdited = useRef(false);
	const userGenRef = useRef(0);

	const toggleStar = useCallback((agentId: string) => {
		setStarredIds((prev) =>
			prev.includes(agentId)
				? prev.filter((id) => id !== agentId)
				: [...prev, agentId]
		);
	}, []);

	useEffect(() => {
		const gen = ++userGenRef.current;
		if (!user) return;

		hasUserEdited.current = false;
		setStarredIds([]);
		setStarredLoaded(false);

		api.userConfig()
			.then((cfg) => {
				if (gen !== userGenRef.current) return;
				const starred = cfg.starred_agents as string[] | undefined;
				if (starred) setStarredIds(starred);
			})
			.catch(() => {})
			.finally(() => {
				if (gen === userGenRef.current) setStarredLoaded(true);
			});
	}, [user]);

	useEffect(() => {
		if (!user || !starredLoaded) return;
		// First run after a load is the load itself, not an edit
		if (!hasUserEdited.current) {
			hasUserEdited.current = true;
			return;
		}
		const timeout = setTimeout(() => {
			const ids = starredRef.current;
			if (ids.length === 0) {
				api.deleteUserConfig("starred_agents").catch(() => {});
			} else {
				api.setUserConfig("starred_agents", ids).catch(() => {});
			}
		}, persistDelayMs);
		return () => clearTimeout(timeout);
	}, [starredIds, user, starredLoaded]);

	return { starredIds, toggleStar };
}