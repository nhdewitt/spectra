import { useState, useEffect, useCallback } from "react";
import { api } from "./api";
import { theme } from "./theme";
import { Login, Header } from "./components";
import type { ViewMode } from "./components";
import { Overview, AgentDetail } from "./pages";
import type { User, Page, OverviewAgent } from "./types";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [checking, setChecking] = useState(true);
  const [page, setPage] = useState<Page>("overview");
  const [selectedAgent, setSelectedAgent] = useState<OverviewAgent | null>(
    null
  );
  const [viewMode, setViewMode] = useState<ViewMode>("tiles");

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

  // Loading splash
  if (checking) {
    return (
      <div
        style={{
          minHeight: "100vh",
          background: theme.bg,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily: theme.font,
          color: theme.textMuted,
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
    <div style={{ minHeight: "100vh", background: theme.bg }}>
      <Header
        user={user}
        onLogout={handleLogout}
        onNavigate={(p) => {
          setPage(p);
          setSelectedAgent(null);
        }}
        currentPage={page}
      />

      {page === "overview" && !selectedAgent && (
        <Overview
            onSelectAgent={setSelectedAgent}
            viewMode={viewMode}
            onViewModeChange={setViewMode}
        />
      )}

      {selectedAgent && (
        <AgentDetail
          agent={selectedAgent}
          onBack={() => setSelectedAgent(null)}
        />
      )}

      {page === "agents" && !selectedAgent && (
        <div
          style={{
            padding: 24,
            fontFamily: theme.font,
            color: theme.textDim,
          }}
        >
          Agent management view — coming soon.
        </div>
      )}

      {page === "admin" && (
        <div
          style={{
            padding: 24,
            fontFamily: theme.font,
            color: theme.textDim,
          }}
        >
          User administration — coming soon.
        </div>
      )}
    </div>
  );
}