import { useState, useCallback } from "react";
import { api } from "../api";
import { theme } from "../theme";
import { OSIcon } from "../icons";
import { Sparkline } from "../Sparkline";
import { usePolling, useSparkHistory } from "../hooks";
import type { SparkData } from "../hooks";
import {
  StatBlock,
  DetailRow,
  ViewToggle,
  LoadingText,
} from "../components";
import type { ViewMode } from "../components";
import type { OverviewAgent } from "../types";
import {
  formatBytes,
  formatUptime,
  statusColor,
  severityColor,
  sortAgentsBySeverity,
  sortAgentsByStatus,
} from "../utils";

// --- Tile view ---

function AgentCard({
  agent,
  onClick,
}: {
  agent: OverviewAgent;
  onClick: (agent: OverviewAgent) => void;
}) {
  const [hovered, setHovered] = useState(false);

  const cpu = agent.cpu_usage ?? 0;
  const mem = agent.ram_percent ?? 0;
  const disk = agent.disk_max_percent ?? 0;
  const temp = agent.max_temp ?? null;

  return (
    <div
      onClick={() => onClick(agent)}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        background: hovered ? theme.surfaceHover : theme.surface,
        border: `1px solid ${hovered ? theme.borderLight : theme.border}`,
        padding: "16px 20px",
        cursor: "pointer",
        transition: "all 0.15s ease",
      }}
    >
      {/* Header row: status dot, OS icon, hostname, platform */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          marginBottom: 14,
        }}
      >
        <div
          style={{
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: statusColor(agent),
            flexShrink: 0,
          }}
        />
        <OSIcon os={agent.os} platform={agent.platform} size={16} />
        <div
          style={{
            fontFamily: theme.font,
            fontSize: 14,
            fontWeight: 500,
            color: theme.text,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {agent.hostname}
        </div>
        <div
          style={{
            marginLeft: "auto",
            fontSize: 11,
            fontFamily: theme.font,
            color: theme.textDim,
          }}
        >
          {agent.platform || agent.os}
        </div>
      </div>

      {/* Stats row */}
      <div style={{ display: "flex", gap: 20, flexWrap: "wrap" }}>
        <StatBlock
          label="CPU"
          value={cpu.toFixed(1)}
          unit="%"
          color={severityColor(cpu, [50, 80, 95])}
        />
        <StatBlock
          label="MEM"
          value={mem.toFixed(1)}
          unit="%"
          color={severityColor(mem, [50, 80, 95])}
        />
        <StatBlock
          label="DISK"
          value={disk.toFixed(1)}
          unit="%"
          color={severityColor(disk, [60, 80, 90])}
        />
        {temp != null && temp > 0 && (
          <StatBlock
            label="TEMP"
            value={temp.toFixed(0)}
            unit="°C"
            color={severityColor(temp, [50, 70, 85])}
          />
        )}
      </div>

      {/* Expanded detail on hover */}
      {hovered && (
        <div
          style={{
            marginTop: 14,
            paddingTop: 12,
            borderTop: `1px solid ${theme.border}`,
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: "6px 16px",
            fontSize: 12,
            fontFamily: theme.font,
          }}
        >
          <DetailRow label="Platform" value={agent.platform} />
          <DetailRow label="Arch" value={agent.arch} />
          <DetailRow label="Cores" value={agent.cpu_cores} />
          <DetailRow label="Uptime" value={formatUptime(agent.uptime)} />
          <DetailRow label="Processes" value={agent.process_count} />
          <DetailRow
            label="Network"
            value={
              agent.net_rx_bytes != null
                ? `↓${formatBytes(agent.net_rx_bytes)} ↑${formatBytes(agent.net_tx_bytes)}`
                : null
            }
          />
          <DetailRow
            label="Last seen"
            value={
              agent.last_seen
                ? new Date(agent.last_seen).toLocaleTimeString()
                : null
            }
          />
          <DetailRow
            label="Reboot"
            value={
              agent.reboot_required === true
                ? "Required"
                : agent.reboot_required === false
                  ? "No"
                  : null
            }
          />
        </div>
      )}
    </div>
  );
}

// --- List view ---

const listHeaderStyle: React.CSSProperties = {
  fontSize: 10,
  fontFamily: theme.font,
  color: theme.textDim,
  letterSpacing: "0.05em",
  textTransform: "uppercase",
  padding: "8px 12px",
  textAlign: "right",
};

function AgentListHeader() {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "28px 20px 1fr 80px 80px 80px 80px 80px 80px 80px 80px",
        alignItems: "center",
        borderBottom: `1px solid ${theme.border}`,
        background: theme.surface,
      }}
    >
      <div />
      <div />
      <div style={{ ...listHeaderStyle, textAlign: "left" }}>Host</div>
      <div style={listHeaderStyle}>CPU</div>
      <div style={{ ...listHeaderStyle, textAlign: "center" }}>CPU</div>
      <div style={listHeaderStyle}>MEM</div>
      <div style={{ ...listHeaderStyle, textAlign: "center" }}>MEM</div>
      <div style={listHeaderStyle}>DISK</div>
      <div style={{ ...listHeaderStyle, textAlign: "center" }}>DISK</div>
      <div style={listHeaderStyle}>TEMP</div>
      <div style={listHeaderStyle}>UPTIME</div>
    </div>
  );
}

function AgentListRow({
  agent,
  sparkData,
  onClick,
}: {
  agent: OverviewAgent;
  sparkData: SparkData | undefined;
  onClick: (agent: OverviewAgent) => void;
}) {
  const [hovered, setHovered] = useState(false);

  const cpu = agent.cpu_usage ?? 0;
  const mem = agent.ram_percent ?? 0;
  const disk = agent.disk_max_percent ?? 0;
  const temp = agent.max_temp ?? 0;

  return (
    <div
      onClick={() => onClick(agent)}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        display: "grid",
        gridTemplateColumns: "28px 20px 1fr 80px 80px 80px 80px 80px 80px 80px 80px",
        alignItems: "center",
        padding: "6px 0",
        background: hovered ? theme.surfaceHover : "transparent",
        borderBottom: `1px solid ${theme.border}`,
        cursor: "pointer",
        transition: "background 0.1s ease",
        fontFamily: theme.font,
        fontSize: 12,
      }}
    >
      {/* Status dot */}
      <div style={{ display: "flex", justifyContent: "center" }}>
        <div
          style={{
            width: 7,
            height: 7,
            borderRadius: "50%",
            background: statusColor(agent),
          }}
        />
      </div>

      {/* OS icon */}
      <div style={{ display: "flex", justifyContent: "center" }}>
        <OSIcon os={agent.os} platform={agent.platform} size={14} />
      </div>

      {/* Hostname + platform */}
      <div style={{ padding: "0 12px", overflow: "hidden" }}>
        <div
          style={{
            color: theme.text,
            fontWeight: 500,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {agent.hostname}
        </div>
        <div style={{ fontSize: 10, color: theme.textDim, marginTop: 1 }}>
          {agent.platform} · {agent.arch}
        </div>
      </div>

      {/* CPU % */}
      <div
        style={{
          textAlign: "right",
          padding: "0 12px",
          color: severityColor(cpu, [50, 80, 95]),
        }}
      >
        {cpu.toFixed(1)}%
      </div>

      {/* CPU sparkline */}
      <div style={{ display: "flex", justifyContent: "center" }}>
        <Sparkline
          data={sparkData?.cpu ?? []}
          width={64}
          height={20}
          thresholds={[50, 80, 95]}
        />
      </div>

      {/* MEM % */}
      <div
        style={{
          textAlign: "right",
          padding: "0 12px",
          color: severityColor(mem, [50, 80, 95]),
        }}
      >
        {mem.toFixed(1)}%
      </div>

      {/* MEM sparkline */}
      <div style={{ display: "flex", justifyContent: "center" }}>
        <Sparkline
          data={sparkData?.mem ?? []}
          width={64}
          height={20}
          thresholds={[50, 80, 95]}
        />
      </div>

      {/* DISK % */}
      <div
        style={{
          textAlign: "right",
          padding: "0 12px",
          color: severityColor(disk, [60, 80, 90]),
        }}
      >
        {disk.toFixed(1)}%
      </div>

      {/* DISK sparkline */}
      <div style={{ display: "flex", justifyContent: "center" }}>
        <Sparkline
            data={sparkData?.disk ?? []}
            width={64}
            height={20}
            thresholds={[60, 80, 90]}
        />
      </div>

      {/* TEMP */}
      <div
        style={{
          textAlign: "right",
          padding: "0 12px",
          color:
            temp && temp > 0
              ? severityColor(temp, [50, 70, 85])
              : theme.textDim,
        }}
      >
        {temp && temp > 0 ? `${temp.toFixed(0)}°` : "—"}
      </div>

      {/* UPTIME */}
      <div
        style={{ textAlign: "right", padding: "0 12px", color: theme.textMuted }}
      >
        {formatUptime(agent.uptime)}
      </div>
    </div>
  );
}

// --- Overview page ---

export function Overview({
  onSelectAgent,
  viewMode,
  onViewModeChange,
}: {
  onSelectAgent: (agent: OverviewAgent) => void;
  viewMode: ViewMode;
  onViewModeChange: (mode: ViewMode) => void;
}) {
  const fetcher = useCallback(() => api.overview(), []);
  const { data, loading, error } = usePolling(fetcher, 10_000);
  const agents = data ?? [];
  const sparkHistory = useSparkHistory(agents);

  const sorted =
    viewMode === "tiles"
      ? sortAgentsBySeverity(agents)
      : sortAgentsByStatus(agents);

  if (loading) return <LoadingText />;

  if (error) {
    return (
      <div
        style={{ padding: 24, color: theme.danger, fontFamily: theme.font }}
      >
        {error}
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      {/* Toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 20,
        }}
      >
        <div
          style={{
            fontSize: 13,
            fontFamily: theme.font,
            color: theme.textMuted,
          }}
        >
          {agents.length} agent{agents.length !== 1 ? "s" : ""} registered
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <div
            style={{
              display: "flex",
              gap: 12,
              fontSize: 11,
              fontFamily: theme.font,
            }}
          >
            <span style={{ color: theme.ok }}>● online</span>
            <span style={{ color: theme.warn }}>● stale</span>
            <span style={{ color: theme.danger }}>● offline</span>
          </div>
          <ViewToggle mode={viewMode} onChange={onViewModeChange} />
        </div>
      </div>

      {/* Tile grid */}
      {viewMode === "tiles" && (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))",
            gap: 12,
          }}
        >
          {sorted.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              onClick={onSelectAgent}
            />
          ))}
        </div>
      )}

      {/* List table */}
      {viewMode === "list" && (
        <div
          style={{
            border: `1px solid ${theme.border}`,
            overflow: "visible",
          }}
        >
          <AgentListHeader />
          {sorted.map((agent) => (
            <AgentListRow
              key={agent.id}
              agent={agent}
              sparkData={sparkHistory.get(agent.id)}
              onClick={onSelectAgent}
            />
          ))}
        </div>
      )}

      {/* Empty state */}
      {agents.length === 0 && (
        <div
          style={{
            textAlign: "center",
            padding: "60px 0",
            fontFamily: theme.font,
            color: theme.textDim,
            fontSize: 14,
          }}
        >
          No agents registered. Run spectra-setup on a host to get started.
        </div>
      )}
    </div>
  );
}