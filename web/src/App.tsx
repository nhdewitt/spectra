import { useState, useEffect, useCallback } from "react";
import { api } from "./api";
import { initTheme, themeVars } from "./theme";
import { Login } from "./components";
import { Sidebar } from "./components/Sidebar";
import { Overview } from "./pages/Overview";
import { AgentDetail } from "./pages/AgentDetail";
import { AgentManagement } from "./pages/AgentManagement";
import { Settings } from "./pages/Settings";
import type { User, Page, OverviewAgent } from "./types";
import { usePolling } from "./hooks";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [checking, setChecking] = useState(true);
  const [page, setPage] = useState<Page>("overview");
  const [selectedAgent, setSelectedAgent] = useState<OverviewAgent | null>(null);

  // Fetch agent list for sidebar quick access and online count
  const agentFetcher = useCallback(() => api.overview(), []);
  const { data: agents } = usePolling(agentFetcher, 10_000);
  const agentList = agents ?? [];

  const onlineCount = agentList.filter((a) => {
    if (!a.last_seen) return false;
    return (Date.now() - new Date(a.last_seen).getTime()) / 1000 < 120;
  }).length;

  const handleLogout = useCallback(async () => {
    try {
      await api.logout();
    } catch {}
    setUser(null);
    setPage("overview");
    setSelectedAgent(null);
  }, []);

  // Expose logout for 401 interceptor in api.ts
  useEffect(() => {
    window.__spectraLogout = handleLogout;
    return () => {
      delete window.__spectraLogout;
    };
  }, [handleLogout]);

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
    setPage("detail");
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
    return <Login onLogin={setUser} />;
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
        onlineCount={onlineCount}
        totalCount={agentList.length}
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
            viewMode="list"
            onViewModeChange={() => {}}
          />
        )}

        {page === "detail" && selectedAgent && (
          <AgentDetail
            agent={selectedAgent}
            onBack={() => { setSelectedAgent(null); setPage("overview"); }}
          />
        )}

        {page === "detail" && !selectedAgent && (
          <div
            style={{
              padding: 48,
              textAlign: "center",
              fontFamily: themeVars.font,
              color: themeVars.textDim,
              fontSize: 13,
            }}
          >
            Select an agent from the overview or quick access to view details.
          </div>
        )}

        {page === "diagnostics" && selectedAgent && (
          <div style={{ padding: 24 }}>
            <div
              style={{
                fontFamily: themeVars.font,
                fontSize: 16,
                fontWeight: 600,
                color: themeVars.text,
                marginBottom: 16,
              }}
            >
              Diagnostics — {selectedAgent.hostname}
            </div>
          </div>
        )}

        {page === "agents" && <AgentManagement />}

        {page === "users" && (
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
              User Management
            </div>
          </div>
        )}

        {page === "settings" && (
          <Settings user={user} onLogout={handleLogout} />
        )}
      </div>
    </div>
  );
}