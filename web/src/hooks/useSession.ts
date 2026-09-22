import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "../api";
import type { User } from "../types";

export interface Session {
	user: User | null;
	/** True until the mount-time session check settles. */
	checking: boolean;
	/** Message shown on the login screen, set when a session ends unexpectedly. */
	logoutReason: string | null;
	signIn: (user: User) => void;
	logout: () => Promise<void>;
}

/**
 * Owns authentication state and the two ways a session can end.
 * 
 * onSignOut runs whenever the session is cleared, so the caller can reset anything
 * tied to it. It is held in a ref so the returned callbacks stay referentially stable
 * regardless of what the caller passes.
 */
export function useSession(onSignOut?: () => void): Session {
	const [user, setUser] = useState<User | null>(null);
	const [checking, setChecking] = useState(true);
	const [logoutReason, setLogoutReason] = useState<string | null>(null);

	const loggingOut = useRef(false);
	const onSignOutRef = useRef(onSignOut);
	onSignOutRef.current = onSignOut;

	const clearSession = useCallback((reason?: string) => {
		setUser(null);
		if (reason) setLogoutReason(reason);
		onSignOutRef.current?.();
	}, []);

	const signIn = useCallback((u: User) => {
		setUser(u);
		setLogoutReason(null);
	}, []);

	// Explicit logout. Drop the session server-side, then clear locally.
	const logout = useCallback(async () => {
		loggingOut.current = true;
		try {
			await api.logout();
		} catch {
		} finally {
			loggingOut.current = false;
		}
		clearSession();
	}, [clearSession]);

	// Expired session: don't call api.logout() (invalid cookie, 401). The
	// guard handles the other direction. An explicit logout 401 should not
	// report an expiry the user never experienced.
	const handleSessionExpired = useCallback(() => {
		if (loggingOut.current) return;
		clearSession("Your session has expired.");
	}, [clearSession]);

	// Expose expiry handling for the 401 interceptor in api.ts.
	useEffect(() => {
		window.__spectraLogout = handleSessionExpired;
		return () => {
			delete window.__spectraLogout;
		};
	}, [handleSessionExpired]);

	useEffect(() => {
		api.me()
			.then(setUser)
			.catch(() => {})
			.finally(() => setChecking(false));
	}, []);

	return { user, checking, logoutReason, signIn, logout };
}