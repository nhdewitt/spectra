import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "./api";
import { initTheme, themeVars } from "./theme";
import { Login } from "./components";
import { Sidebar } from "./components/Sidebar";
import { Overview } from "./pages/Overview";
import { AgentDetail } from "./pages/AgentDetail";
import { AgentManagement } from "./pages/AgentManagement";
import { Diagnostics } from "./pages/Diagnostics";
import { UserManagement } from "./pages/UserManagement";
import { Settings } from "./pages/Settings";
import { Tags } from "./pages/Tags";
import type { User, Page, OverviewAgent } from "./types";
import { usePolling } from "./hooks";
import { statusColor } from "./utils";
import { Alerts } from "./pages/Alerts";

export default function App() {
	const [user, setUser] = useState<User | null>(null);
	const [checking, setChecking] = useState(true);
	const [page, setPage] = useState<Page>("overview");
	const [selectedAgent, setSelectedAgent] = useState<OverviewAgent | null>(null);
	const [starredIds, setStarredIds] = useState<string[]>([]);
	const [starredLoaded, setStarredLoaded] = useState(false);
	const [logoutReason, setLogoutReason] = useState<string | null>(null);
	const [version, setVersion] = useState<string>("");

	const starredRef = useRef(starredIds);
	starredRef.current = starredIds;
	const hasUserEdited = useRef(false);

	const toggleStar = useCallback((agentId: string) => {
		setStarredIds((prev) =>
			prev.includes(agentId)
				? prev.filter((id) => id !== agentId)
				: [...prev, agentId]
		);
	}, []);

	useEffect(() => {
		if (!user) return;
		setStarredIds([]);
		setStarredLoaded(false);
		api.userConfig()
			.then((cfg) => {
				const starred = cfg.starred_agents as string[] | undefined;
				if (starred) setStarredIds(starred);
			})
			.catch(() => {})
			.finally(() => setStarredLoaded(true));
	}, [user]);

	useEffect(() => {
		if (!user || !starredLoaded) return;
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
		}, 500);
		return () => clearTimeout(timeout);
	}, [starredIds, user, starredLoaded]);

	// Fetch agent list for sidebar quick access and online count
	const agentFetcher = useCallback(() => user ? api.overview() : Promise.resolve([]), [user]);
	const { data: agents } = usePolling(agentFetcher, 10_000);
	const agentList = agents ?? [];

	const onlineCount = agentList.filter((a) => {
		if (!a.last_seen) return false;
		return (Date.now() - new Date(a.last_seen).getTime()) / 1000 < 120;
	}).length;

	const handleLogout = useCallback(async (reason?: string) => {
		try {
			await api.logout();
		} catch {}
		setUser(null);
		setPage("overview");
		setSelectedAgent(null);
		if (reason) setLogoutReason(reason);
	}, []);

	// Expose logout for 401 interceptor in api.ts
	useEffect(() => {
		window.__spectraLogout = () => handleLogout("Your session has expired.");
		return () => {
			delete window.__spectraLogout;
		};
	}, [handleLogout]);

	useEffect(() => {
		api.version()
			.then((v) => setVersion(v.version))
			.catch(() => {});
	}, []);

	// Check existing session on mount
	useEffect(() => {
		api
			.me()
			.then(setUser)
			.catch(() => {})
			.finally(() => setChecking(false));
	}, []);

	useEffect(() => {
		initTheme();
	}, []);

	const handleSelectAgent = useCallback((agent: OverviewAgent) => {
		setSelectedAgent(agent);
		setPage((prev) => prev === "diagnostics" ? "diagnostics" : "detail");
	}, []);

	const handleNavigate = useCallback((p: Page) => {
		setPage(p);
		if (p !== "detail" && p !== "diagnostics") {
			setSelectedAgent(null);
		}
	}, []);

	// Loading splash
	if (checking) {
		return (
			<div
				style={{
					minHeight: "100vh",
					background: themeVars.bg,
					display: "flex",
					alignItems: "center",
					justifyContent: "center",
					fontFamily: themeVars.font,
					color: themeVars.textMuted,
				}}
			>
				...
			</div>
		);
	}

	// Not authenticated
	if (!user) {
		return <Login onLogin={(u) => { setUser(u); setLogoutReason(null); }} message={logoutReason} />;
	}

	// Authenticated shell
	return (
		<div style={{ display: "flex", minHeight: "100vh", background: themeVars.bg }}>
			<Sidebar
				user={user}
				currentPage={page}
				onNavigate={handleNavigate}
				selectedAgent={selectedAgent}
				onSelectAgent={handleSelectAgent}
				agents={agentList}
				starredIds={starredIds}
				version={version}
			/>

			<div style={{ flex: 1, minWidth: 0 }}>
				{/* Content header */}
				<div
					style={{
						display: "flex",
						alignItems: "center",
						justifyContent: "flex-end",
						padding: "8px 24px",
						fontSize: 12,
						fontFamily: themeVars.font,
						color: themeVars.textMuted,
						borderBottom: `1px solid ${themeVars.border}`,
					}}
				>
					<span>
						{new Date().toLocaleDateString(undefined, {
							month: "short",
							day: "numeric",
							year: "numeric",
						})}
					</span>
					<span style={{ margin: "0 12px", color: themeVars.border }}>|</span>
					<span style={{ color: themeVars.ok }}>●</span>
					<span style={{ marginLeft: 4 }}>
						{onlineCount}/{agentList.length} online
					</span>
				</div>

				{/* Page content */}
				{page === "overview" && (
					<Overview
						onSelectAgent={handleSelectAgent}
						starredIds={starredIds}
						onToggleStar={toggleStar}
					/>
				)}

				{page === "detail" && selectedAgent && (
					<AgentDetail
						agent={selectedAgent}
						agents={agentList}
						user={user}
						onSelectAgent={handleSelectAgent}
						onBack={() => { setSelectedAgent(null); setPage("overview"); }}
						starredIds={starredIds}
						onToggleStar={toggleStar}
					/>
				)}

				{page === "detail" && !selectedAgent && (
					<div style={{ padding: 24}}>
						<div
							style={{
								fontFamily: themeVars.font,
								fontSize: 16,
								fontWeight: 600,
								color: themeVars.text,
								marginBottom: 16,
							}}
						>
							Agent Detail
						</div>
						<div
							style={{
								fontSize: 12,
								fontFamily: themeVars.font,
								color: themeVars.textDim,
								marginBottom: 16,
							}}
						>
							Select an agent to view details.
						</div>
						<div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
							{agentList.map((a) => (
								<button
									key={a.id}
									onClick={() => handleSelectAgent(a)}
									style={{
										display: "flex",
										alignItems: "center",
										gap: 10,
										padding: "8px 12px",
										fontSize: 12,
										fontFamily: themeVars.font,
										color: themeVars.text,
										background: themeVars.surface,
										border: `1px solid ${themeVars.border}`,
										cursor: "pointer",
										textAlign: "left",
									}}
								>
									<span
										style={{
											width: 7,
											height: 7,
											borderRadius: "50%",
											background: statusColor(a),
											flexShrink: 0,
										}}
									/>
									<span style={{ fontWeight: 500 }}>{a.hostname}</span>
									<span style={{ color: themeVars.textDim, marginLeft: "auto", fontSize: 11 }}>
										{a.os} · {a.platform} · {a.arch}
									</span>
								</button>
							))}
						</div>
					</div>
				)}

				{page === "diagnostics" && (
					<Diagnostics
						agents={agentList}
						selectedAgent={selectedAgent}
						onSelectAgent={handleSelectAgent}
					/>
				)}

				{page === "agents" && <AgentManagement user={user}/>}

				{page === "tags" && <Tags user={user} />}

				{page === "users" && <UserManagement user={user} />}

				{page === "settings" && (
					<Settings user={user} onLogout={handleLogout} />
				)}

				{page === "alerts" && <Alerts user={user} />}
			</div>
		</div>
	);
}