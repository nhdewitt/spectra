import { useState, useEffect, useCallback } from "react";
import { api } from "./api";
import type { OverviewStats } from "./api";
import { initTheme, themeVars } from "./theme";
import { Login } from "./components";
import { Sidebar } from "./components/Sidebar";
import { AgentSearchList } from "./components/AgentSearchList";
import { Overview } from "./pages/Overview";
import { AgentDetail } from "./pages/AgentDetail";
import { AgentManagement } from "./pages/AgentManagement";
import { Diagnostics } from "./pages/Diagnostics";
import { UserManagement } from "./pages/UserManagement";
import { Settings } from "./pages/Settings";
import { ServerSettings } from "./pages/ServerSettings";
import { Tags } from "./pages/Tags";
import type { Thresholds } from "./types";
import { useNavigation, usePolling, useSession, useStarredAgents } from "./hooks";
import { ThresholdsProvider } from "./ThresholdsContext";
import { DEFAULT_THRESHOLDS } from "./types";
import { Alerts } from "./pages/Alerts";

export default function App() {
	const [version, setVersion] = useState<string>("");
	const [thresholds, setThresholds] = useState<Thresholds>(DEFAULT_THRESHOLDS);

	const {
		page,
		selectedAgent,
		navigate: handleNavigate,
		selectAgent: handleSelectAgent,
		back,
		reset: resetNavigation,
	} = useNavigation();

	const { user, checking, logoutReason, signIn, logout } = useSession(resetNavigation);
	const { starredIds, toggleStar } = useStarredAgents(user);

	const statsFetcher = useCallback(() => user ? api.overviewStats() : Promise.resolve(null), [user]);
	const { data: stats } = usePolling<OverviewStats | null>(statsFetcher, 10_000);

	const onlineCount = stats?.online ?? 0;
	const totalCount = stats?.total ?? 0;

	useEffect(() => {
		api.version()
			.then((v) => setVersion(v.version))
			.catch(() => {});
	}, []);

	useEffect(() => {
		if (!user) return;
		api.thresholds().then(setThresholds).catch(() => {});
	}, [user]);

	useEffect(() => {
		initTheme();
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
		return <Login onLogin={signIn} message={logoutReason} />;
	}

	// Authenticated shell
	return (
		<ThresholdsProvider value = {thresholds}>
			<div style={{ display: "flex", minHeight: "100vh", background: themeVars.bg }}>
				<Sidebar
					user={user}
					currentPage={page}
					onNavigate={handleNavigate}
					selectedAgent={selectedAgent}
					onSelectAgent={handleSelectAgent}
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
							{onlineCount}/{totalCount} online
						</span>
					</div>
 
					{/* Page content */}
					{page === "overview" && (
						<Overview
							stats={stats}
							onSelectAgent={handleSelectAgent}
							starredIds={starredIds}
							onToggleStar={toggleStar}
						/>
					)}
 
					{page === "detail" && selectedAgent && (
						<AgentDetail
							agent={selectedAgent}
							user={user}
							onSelectAgent={handleSelectAgent}
							onBack={back}
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
							<div style={{ maxWidth: 420 }}>
								<AgentSearchList onSelectAgent={handleSelectAgent} placeholder="Search agents..." />
							</div>
						</div>
					)}
 
					{page === "diagnostics" && (
						<Diagnostics
							selectedAgent={selectedAgent}
							onSelectAgent={handleSelectAgent}
						/>
					)}
 
					{page === "agents" && <AgentManagement user={user}/>}
 
					{page === "tags" && <Tags user={user} />}
 
					{page === "users" && <UserManagement user={user} />}
 
					{page === "settings" && (
						<Settings user={user} onLogout={logout} />
					)}
 
					{page === "server" && <ServerSettings user={user} />}
 
					{page === "alerts" && <Alerts user={user} />}
				</div>
			</div>
		</ThresholdsProvider>
	);
}