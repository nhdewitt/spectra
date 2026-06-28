import React, { useEffect, useState, useCallback, useMemo } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { OSIcon } from "../icons";
import { Sparkline } from "../Sparkline";
import { useSparkHistory } from "../hooks";
import { usePagination, Pagination } from "../hooks/usePagination";
import type { SparkData } from "../hooks";
import { StatBlock, LoadingSpinner } from "../components";
import { LabelChip } from "../components/LabelChip";
import type { OverviewAgent, AgentLabel, LabelKey } from "../types";
import {
	formatBytes,
	formatUptime,
	severityColor,
	agentStatus,
	agentStatusColor,
} from "../utils";
import type { AgentStatus } from "../utils";

type SortOption = "severity" | "status" | "hostname" | "cpu" | "memory" | "disk" | "temp";

const PAGE_SIZE_OPTIONS = [25, 50, 100, 250];
const DEFAULT_PAGE_SIZE = 25;

interface OverviewProps {
	agents: OverviewAgent[];
	loading: boolean;
	error: string | null;
	onSelectAgent: (agent: OverviewAgent) => void;
	starredIds: string[];
	onToggleStar: (agentId: string) => void;
}

interface LabelFilter {
	key: string;
	value: string;
}

// --- Stat Bar ---

function StatBar({ agents }: { agents: OverviewAgent[] }) {
	const counts = useMemo(() => {
		const c = { total: agents.length, online: 0, stale: 0, offline: 0, warn: 0, crit: 0, reboot: 0 };
		for (const a of agents) {
			const { status } = agentStatus(a);
			switch (status) {
				case "online": c.online++; break;
				case "warn": c.warn++; break;
				case "crit": c.crit++; break;
				case "stale": c.stale++; break;
				case "offline": c.offline++; break;
			}
			if (a.reboot_required) c.reboot++;
		}
		return c;
	}, [agents]);

	return (
		<div
			style={{
				display: "flex",
				gap: 0,
				marginBottom: 20,
				border: `1px solid ${themeVars.border}`,
			}}
		>
			<StatBarItem label="Total Agents" value={counts.total} color={themeVars.text} />
			<StatBarItem label="Online" value={counts.online} color={themeVars.ok} />
			<StatBarItem label="Stale" value={counts.stale} color={themeVars.warn} />
			<StatBarItem label="Offline" value={counts.offline} color={themeVars.textDim} />
			<StatBarItem label="Warning" value={counts.warn} color={themeVars.warn} />
			<StatBarItem label="Critical" value={counts.crit} color={themeVars.danger} />
			<StatBarItem label="Reboot Req." value={counts.reboot} color={themeVars.warn} />
		</div>
	);
}

function StatBarItem({ label, value, color }: { label: string; value: number; color: string }) {
	return (
		<div
			style={{
				flex: 1,
				padding: "12px 16px",
				borderRight: `1px solid ${themeVars.border}`,
				background: themeVars.surface,
			}}
		>
			<div
				style={{
					fontSize: 22,
					fontWeight: 700,
					fontFamily: themeVars.font,
					color,
				}}
			>
				{value}
			</div>
			<div
				style={{
					fontSize: 10,
					fontFamily: themeVars.font,
					color: themeVars.textDim,
					letterSpacing: "0.04em",
					textTransform: "uppercase",
					marginTop: 2,
				}}
			>
				{label}
			</div>
		</div>
	);
}

// --- Percentage Bar ---

function PercentBar({ value, thresholds }: { value: number; thresholds: [number, number, number] }) {
	const color = severityColor(value, thresholds);
	return (
		<div
			style={{
				width: 60,
				height: 6,
				background: themeVars.border,
				borderRadius: 1,
				overflow: "hidden",
			}}
		>
			<div
				style={{
					width: `${Math.min(value, 100)}%`,
					height: "100%",
					background: color,
					transition: "width 0.3s ease",
				}}
			/>
		</div>
	);
}

// --- Status Badge ---

function StatusBadge({ agent }: { agent: OverviewAgent }) {
	const { status, reasons } = agentStatus(agent);
	const color = agentStatusColor(status);

	return (
		<span
			title={reasons.length > 0 ? reasons.join("\n") : undefined}
			style={{
				fontSize: 9,
				fontFamily: themeVars.font,
				fontWeight: 600,
				color,
				background: `color-mix(in srgb, ${color} 15%, transparent)`,
				border: `1px solid ${color}`,
				padding: "1px 6px",
				letterSpacing: "0.04em",
				textTransform: "uppercase",
				cursor: reasons.length > 0 ? "help" : "default",
			}}
		>
			{status.toUpperCase()}
		</span>
	);
}

// --- Star Button ---

function StarButton({
	isStarred,
	onToggle,
	size = 18,
}: {
	isStarred: boolean;
	onToggle: () => void;
	size?: number;
}) {
	return (
		<button
			onClick={(e) => { e.stopPropagation(); onToggle(); }}
			title={isStarred ? "Remove from quick access" : "Add to quick access"}
			style={{
				background: "none",
				border: "none",
				cursor: "pointer",
				fontSize: size,
				color: isStarred ? themeVars.warn : themeVars.textDim,
				padding: 0,
				lineHeight: 1,
				flexShrink: 0,
			}}
		>
			{isStarred ? "★" : "☆"}
		</button>
	);
}

// --- Label Filter Bar ---

function LabelFilterBar({
	filters,
	knownKeys,
	labelsByAgent,
	onAdd,
	onRemove,
	onClear,
}: {
	filters: LabelFilter[];
	knownKeys: LabelKey[];
	labelsByAgent: Map<string, AgentLabel[]>;
	onAdd: (key: string, value: string) => void;
	onRemove: (f: LabelFilter) => void;
	onClear: () => void;
}) {
	const [pickerKey, setPickerKey] = useState("");
	const [pickerValue, setPickerValue] = useState("");

	const userKeys = useMemo(() => knownKeys.filter((k) => k.source === "user"), [knownKeys]);

	const pickerValues = useMemo(() => {
		if (!pickerKey) return [];
		const s = new Set<string>();
		for (const labels of labelsByAgent.values()) {
			for (const l of labels) {
				if (l.key === pickerKey) s.add(l.value);
			}
		}
		return Array.from(s).sort();
	}, [pickerKey, labelsByAgent]);

	const handleAdd = () => {
		const k = pickerKey.trim();
		const v = pickerValue.trim();
		if (!k || !v) return;
		onAdd(k, v);
		setPickerKey("");
		setPickerValue("");
	};

	const selectStyle: React.CSSProperties = {
		padding: "4px 8px",
		fontSize: 11,
		fontFamily: themeVars.font,
		color: themeVars.text,
		background: themeVars.surface,
		border: `1px solid ${themeVars.border}`,
		cursor: "pointer",
	};

	return (
		<div
			style={{
				display: "flex",
				alignItems: "center",
				gap: 8,
				flexWrap: "wrap",
				marginBottom: 12,
				padding: "8px 12px",
				background: themeVars.surface,
				border: `1px solid ${themeVars.border}`,
			}}
		>
			<span
				style={{
					fontSize: 10,
					fontFamily: themeVars.font,
					color: themeVars.textMuted,
					textTransform: "uppercase",
					letterSpacing: "0.04em",
				}}
			>
				Label filters
				{filters.length > 0 ? ` (AND)` : ""}
			</span>
 
			{filters.map((f) => {
				const synthetic: AgentLabel = {
					key: f.key,
					value: f.value,
					source: "user",
					updated_at: "",
				};
				return (
					<LabelChip
						key={`${f.key}=${f.value}`}
						label={synthetic}
						onDelete={() => onRemove(f)}
					/>
				);
			})}
 
			{/* Inline picker */}
			<select
				value={pickerKey}
				onChange={(e) => {
					setPickerKey(e.target.value);
					setPickerValue("");
				}}
				style={selectStyle}
			>
				<option value="">+ Add filter</option>
				{userKeys.map((k) => (
					<option key={k.key} value={k.key}>{k.key}</option>
				))}
			</select>
 
			{pickerKey && (
				<>
					<span style={{ color: themeVars.textDim, fontSize: 11 }}>=</span>
					<select
						value={pickerValue}
						onChange={(e) => setPickerValue(e.target.value)}
						style={selectStyle}
						autoFocus
					>
						<option value="">choose value</option>
						{pickerValues.map((v) => (
							<option key={v} value={v}>{v}</option>
						))}
					</select>
					<button
						onClick={handleAdd}
						disabled={!pickerValue}
						style={{
							padding: "4px 10px",
							fontSize: 11,
							fontFamily: themeVars.font,
							color: themeVars.text,
							background: pickerValue ? themeVars.accentDim : "transparent",
							border: `1px solid ${pickerValue ? themeVars.accent : themeVars.border}`,
							cursor: pickerValue ? "pointer" : "default",
							opacity: pickerValue ? 1 : 0.5,
							textTransform: "uppercase",
							letterSpacing: "0.03em",
						}}
					>
						Add
					</button>
				</>
			)}
 
			{filters.length > 0 && (
				<button
					onClick={onClear}
					style={{
						marginLeft: "auto",
						padding: "2px 8px",
						fontSize: 10,
						fontFamily: themeVars.font,
						color: themeVars.textMuted,
						background: "transparent",
						border: `1px solid ${themeVars.border}`,
						cursor: "pointer",
						textTransform: "uppercase",
						letterSpacing: "0.03em",
					}}
				>
					Clear all
				</button>
			)}
		</div>
	);
}

// --- Filter Toolbar ---

const selectStyle: React.CSSProperties = {
	padding: "5px 8px",
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.surface,
	border: `1px solid ${themeVars.border}`,
	cursor: "pointer",
};

function FilterToolbar({
	search,
	onSearchChange,
	statusFilter,
	onStatusFilterChange,
	osFilter,
	onOsFilterChange,
	archFilter,
	onArchFilterChange,
	hardwareFilter,
	onHardwareFilterChange,
	sort,
	onSortChange,
	osOptions,
	archOptions,
	hardwareOptions,
}: {
	search: string;
	onSearchChange: (v: string) => void;
	statusFilter: AgentStatus | "all";
	onStatusFilterChange: (v: AgentStatus | "all") => void;
	osFilter: string;
	onOsFilterChange: (v: string) => void;
	archFilter: string;
	onArchFilterChange: (v: string) => void;
	hardwareFilter: string;
	onHardwareFilterChange: (v: string) => void;
	sort: SortOption;
	onSortChange: (v: SortOption) => void;
	osOptions: string[];
	archOptions: string[];
	hardwareOptions: string[];
}) {
	return (
		<div
			style={{
				display: "flex",
				alignItems: "center",
				gap: 8,
				marginBottom: 16,
				flexWrap: "wrap",
			}}
		>
			<input
				type="text"
				value={search}
				onChange={(e) => onSearchChange(e.target.value)}
				placeholder="Filter by hostname..."
				style={{
					padding: "5px 10px",
					fontSize: 12,
					fontFamily: themeVars.font,
					color: themeVars.text,
					background: themeVars.surface,
					border: `1px solid ${themeVars.border}`,
					flex: "0 1 220px",
				}}
			/>
 
			<select
				value={statusFilter}
				onChange={(e) => onStatusFilterChange(e.target.value as AgentStatus | "all")}
				style={selectStyle}
			>
				<option value="all">All Status</option>
				<option value="online">Online</option>
				<option value="warn">Warning</option>
				<option value="crit">Critical</option>
				<option value="stale">Stale</option>
				<option value="offline">Offline</option>
			</select>
 
			<select
				value={osFilter}
				onChange={(e) => onOsFilterChange(e.target.value)}
				style={selectStyle}
			>
				<option value="all">All OS</option>
				{osOptions.map((os) => (
					<option key={os} value={os}>{os}</option>
				))}
			</select>
 
			<select
				value={archFilter}
				onChange={(e) => onArchFilterChange(e.target.value)}
				style={selectStyle}
			>
				<option value="all">All Arch</option>
				{archOptions.map((arch) => (
					<option key={arch} value={arch}>{arch}</option>
				))}
			</select>
 
			{hardwareOptions.length > 0 && (
				<select
					value={hardwareFilter}
					onChange={(e) => onHardwareFilterChange(e.target.value)}
					style={selectStyle}
				>
					<option value="all">All Hardware</option>
					{hardwareOptions.map((hw) => (
						<option key={hw} value={hw}>{hw}</option>
					))}
				</select>
			)}
 
			<select
				value={sort}
				onChange={(e) => onSortChange(e.target.value as SortOption)}
				style={selectStyle}
			>
				<option value="severity">Sort: Severity</option>
				<option value="status">Sort: Status</option>
				<option value="hostname">Sort: Hostname</option>
				<option value="cpu">Sort: CPU</option>
				<option value="memory">Sort: Memory</option>
				<option value="disk">Sort: Disk</option>
				<option value="temp">Sort: Temperature</option>
			</select>
		</div>
	);
}

// --- Table ---

const headerStyle: React.CSSProperties = {
	fontSize: 10,
	fontFamily: themeVars.font,
	color: themeVars.textDim,
	letterSpacing: "0.05em",
	textTransform: "uppercase",
	padding: "8px 10px",
	textAlign: "right",
	whiteSpace: "nowrap",
};

function TableHeader() {
	return (
		<tr style={{ borderBottom: `1px solid ${themeVars.border}` }}>
			<th style={{ ...headerStyle, textAlign: "left", width: 28 }} />
			<th style={{ ...headerStyle, textAlign: "left" }}>Hostname</th>
			<th style={{ ...headerStyle, textAlign: "left", width: 80 }}>Status</th>
			<th style={{ ...headerStyle, textAlign: "left", width: 100 }}>OS / Platform</th>
			<th style={{ ...headerStyle, width: 60 }}>CPU</th>
			<th style={{ ...headerStyle, width: 60 }} />
			<th style={{ ...headerStyle, width: 60 }}>Memory</th>
			<th style={{ ...headerStyle, width: 60 }} />
			<th style={{ ...headerStyle, width: 60 }}>Disk</th>
			<th style={{ ...headerStyle, width: 60 }} />
			<th style={{ ...headerStyle, width: 50 }}>Temp</th>
			<th style={{ ...headerStyle, width: 70 }}>CPU Trend</th>
			<th style={{ ...headerStyle, width: 70 }}>Uptime</th>
			<th style={{ ...headerStyle, width: 70 }}>Last Seen</th>
			<th style={{ ...headerStyle, width: 50 }}>Procs</th>
			<th style={{ ...headerStyle, width: 100 }}>Net RX/TX</th>
		</tr>
	);
}

function formatLastSeen(lastSeen: string | null): string {
	if (!lastSeen) return "—";
	const ago = (Date.now() - new Date(lastSeen).getTime()) / 1000;
	if (ago < 60) return `${Math.floor(ago)}s ago`;
	if (ago < 3600) return `${Math.floor(ago / 60)}m ago`;
	if (ago < 86400) return `${Math.floor(ago / 3600)}h ago`;
	return `${Math.floor(ago / 86400)}d ago`;
}

function AgentRow({
	agent,
	sparkData,
	onClick,
	isStarred,
	onToggleStar,
}: {
	agent: OverviewAgent;
	sparkData: SparkData | undefined;
	onClick: (agent: OverviewAgent) => void;
	isStarred: boolean;
	onToggleStar: (agentId: string) => void;
}) {
	const [hovered, setHovered] = useState(false);

	const cpu = agent.cpu_usage ?? 0;
	const mem = agent.ram_percent ?? 0;
	const disk = agent.disk_max_percent ?? 0;
	const temp = agent.max_temp ?? 0;

	const cellStyle: React.CSSProperties = {
		padding: "8px 10px",
		fontSize: 12,
		fontFamily: themeVars.font,
		borderBottom: `1px solid ${themeVars.border}`,
	};

	return (
		<tr
			onClick={() => onClick(agent)}
			onMouseEnter={() => setHovered(true)}
			onMouseLeave={() => setHovered(false)}
			style={{
				background: hovered ? themeVars.surfaceHover : "transparent",
				cursor: "pointer",
				transition: "background 0.1s ease",
			}}
		>
			{/* Status dot */}
			<td style={cellStyle}>
				<div
					style={{
						width: 7,
						height: 7,
						borderRadius: "50%",
						background: agentStatusColor(agentStatus(agent).status),
					}}
				/>
			</td>
 
			{/* Hostname + reboot badge + star */}
			<td style={{ ...cellStyle, fontWeight: 500, color: themeVars.text }}>
				<div style={{ display: "flex", alignItems: "center", gap: 6 }}>
					<OSIcon os={agent.os} platform={agent.platform} size={14} />
					<span>{agent.hostname}</span>
					{agent.reboot_required && (
						<span
							style={{
								fontSize: 9,
								fontFamily: themeVars.font,
								color: themeVars.warn,
								background: `color-mix(in srgb, ${themeVars.warn} 15%, transparent)`,
								border: `1px solid ${themeVars.warn}`,
								padding: "1px 5px",
								letterSpacing: "0.04em",
								fontWeight: 600,
							}}
						>
							REBOOT
						</span>
					)}
					<span style={{ marginLeft: "auto" }}>
						<StarButton isStarred={isStarred} onToggle={() => onToggleStar(agent.id)} />
					</span>
				</div>
				<div style={{ fontSize: 10, color: themeVars.textDim, marginTop: 1 }}>
					{agent.platform} · {agent.arch}
				</div>
			</td>
 
			{/* Status badge */}
			<td style={cellStyle}>
				<StatusBadge agent={agent} />
			</td>
 
			{/* OS / Platform */}
			<td style={{ ...cellStyle, color: themeVars.textMuted, fontSize: 11 }}>
				{agent.os} · {agent.arch}
			</td>
 
			{/* CPU % */}
			<td style={{ ...cellStyle, textAlign: "right", color: severityColor(cpu, [50, 80, 95]) }}>
				{cpu.toFixed(1)}%
			</td>
			<td style={cellStyle}>
				<PercentBar value={cpu} thresholds={[50, 80, 95]} />
			</td>
 
			{/* Memory % */}
			<td style={{ ...cellStyle, textAlign: "right", color: severityColor(mem, [50, 80, 95]) }}>
				{mem.toFixed(1)}%
			</td>
			<td style={cellStyle}>
				<PercentBar value={mem} thresholds={[50, 80, 95]} />
			</td>
 
			{/* Disk % */}
			<td style={{ ...cellStyle, textAlign: "right", color: severityColor(disk, [80, 98, 99]) }}>
				{disk.toFixed(1)}%
			</td>
			<td style={cellStyle}>
				<PercentBar value={disk} thresholds={[80, 98, 99]} />
			</td>
 
			{/* Temp */}
			<td
				style={{
					...cellStyle,
					textAlign: "right",
					color: temp > 0 ? severityColor(temp, [50, 70, 85]) : themeVars.textDim,
				}}
			>
				{temp > 0 ? `${temp.toFixed(0)}°` : "—"}
			</td>
 
			{/* CPU trend */}
			<td style={{ ...cellStyle, textAlign: "center" }}>
				<Sparkline data={sparkData?.cpu ?? []} width={60} height={20} thresholds={[50, 80, 95]} />
			</td>
 
			{/* Uptime */}
			<td style={{ ...cellStyle, textAlign: "right", color: themeVars.textMuted }}>
				{formatUptime(agent.uptime)}
			</td>
 
			{/* Last Seen */}
			<td style={{ ...cellStyle, textAlign: "right", color: themeVars.textMuted }}>
				{formatLastSeen(agent.last_seen)}
			</td>
 
			{/* Process count */}
			<td style={{ ...cellStyle, textAlign: "right", color: themeVars.textMuted }}>
				{agent.process_count ?? "—"}
			</td>
 
			{/* Net RX/TX */}
			<td style={{ ...cellStyle, textAlign: "right", color: themeVars.textMuted, fontSize: 11 }}>
				{agent.net_rx_bytes != null ? (
					<>
						<span>↓ {formatBytes(agent.net_rx_bytes)}</span>
						<br />
						<span>↑ {formatBytes(agent.net_tx_bytes)}</span>
					</>
				) : "—"}
			</td>
		</tr>
	);
}

// --- Card View ---

function AgentCard({
	agent,
	onClick,
	isStarred,
	onToggleStar,
}: {
	agent: OverviewAgent;
	onClick: (agent: OverviewAgent) => void;
	isStarred: boolean;
	onToggleStar: (agentId: string) => void;
}) {
	const [hovered, setHovered] = useState(false);
	const { status, reasons } = agentStatus(agent);

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
				background: hovered ? themeVars.surfaceHover : themeVars.surface,
				border: `1px solid ${hovered ? themeVars.borderLight : themeVars.border}`,
				padding: "16px 20px",
				cursor: "pointer",
				transition: "all 0.15s ease",
			}}
		>
			{/* Header */}
			<div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 14 }}>
				<div
					style={{
						width: 8,
						height: 8,
						borderRadius: "50%",
						background: agentStatusColor(status),
						flexShrink: 0,
					}}
				/>
				<OSIcon os={agent.os} platform={agent.platform} size={16} />
				<div
					style={{
						fontFamily: themeVars.font,
						fontSize: 14,
						fontWeight: 500,
						color: themeVars.text,
						overflow: "hidden",
						textOverflow: "ellipsis",
						whiteSpace: "nowrap",
						display: "flex",
						alignItems: "center",
						gap: 6,
					}}
				>
					{agent.hostname}
					{agent.reboot_required && (
						<span
							style={{
								fontSize: 9,
								fontFamily: themeVars.font,
								color: themeVars.warn,
								background: `color-mix(in srgb, ${themeVars.warn} 15%, transparent)`,
								border: `1px solid ${themeVars.warn}`,
								padding: "1px 5px",
								letterSpacing: "0.04em",
								fontWeight: 600,
								flexShrink: 0,
							}}
						>
							REBOOT
						</span>
					)}
				</div>
				<div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 8 }}>
					<StarButton isStarred={isStarred} onToggle={() => onToggleStar(agent.id)} size={18} />
					<StatusBadge agent={agent} />
				</div>
			</div>
 
			{/* Stats */}
			<div style={{ display: "flex", gap: 20, flexWrap: "wrap" }}>
				<StatBlock label="CPU" value={cpu.toFixed(1)} unit="%" color={severityColor(cpu, [50, 80, 95])} />
				<StatBlock label="MEM" value={mem.toFixed(1)} unit="%" color={severityColor(mem, [50, 80, 95])} />
				<StatBlock label="DISK" value={disk.toFixed(1)} unit="%" color={severityColor(disk, [80, 98, 99])} />
				{temp != null && temp > 0 && (
					<StatBlock label="TEMP" value={temp.toFixed(0)} unit="°C" color={severityColor(temp, [50, 70, 85])} />
				)}
			</div>
 
			{/* Hover details */}
			{hovered && (
				<div
					style={{
						marginTop: 14,
						paddingTop: 12,
						borderTop: `1px solid ${themeVars.border}`,
						fontSize: 12,
						fontFamily: themeVars.font,
						color: themeVars.textMuted,
					}}
				>
					<div style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
						<span>{agent.platform} · {agent.arch}</span>
						<span>{formatUptime(agent.uptime)}</span>
						<span>{formatLastSeen(agent.last_seen)}</span>
						{agent.process_count != null && <span>{agent.process_count} procs</span>}
					</div>
					{reasons.length > 0 && (
						<div style={{ marginTop: 6, color: agentStatusColor(status), fontSize: 11 }}>
							{reasons.join(" · ")}
						</div>
					)}
				</div>
			)}
		</div>
	);
}

// --- Sorting ---

function sortAgents(agents: OverviewAgent[], sort: SortOption): OverviewAgent[] {
	return [...agents].sort((a, b) => {
		switch (sort) {
			case "severity": {
				const scoreA = (a.cpu_usage ?? 0) + (a.disk_max_percent ?? 0) + (a.ram_percent ?? 0);
				const scoreB = (b.cpu_usage ?? 0) + (b.disk_max_percent ?? 0) + (b.ram_percent ?? 0);
				return scoreB - scoreA;
			}
			case "status": {
				const order: Record<AgentStatus, number> = { crit: 0, warn: 1, stale: 2, offline: 3, online: 4 };
				const diff = order[agentStatus(a).status] - order[agentStatus(b).status];
				if (diff !== 0) return diff;
				return a.hostname.localeCompare(b.hostname);
			}
			case "hostname":
				return a.hostname.localeCompare(b.hostname, undefined, { sensitivity: "base" });
			case "cpu":
				return (b.cpu_usage ?? 0) - (a.cpu_usage ?? 0);
			case "memory":
				return (b.ram_percent ?? 0) - (a.ram_percent ?? 0);
			case "disk":
				return (b.disk_max_percent ?? 0) - (a.disk_max_percent ?? 0);
			case "temp":
				return (b.max_temp ?? 0) - (a.max_temp ?? 0);
			default:
				return 0;
		}
	});
}

// --- Overview Page ---

export function Overview({ agents, loading, error, onSelectAgent, starredIds, onToggleStar }: OverviewProps) {
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState<AgentStatus | "all">("all");
	const [osFilter, setOsFilter] = useState("all");
	const [archFilter, setArchFilter] = useState("all");
	const [hardwareFilter, setHardwareFilter] = useState("all");
	const [sort, setSort] = useState<SortOption>("severity");
	const [viewMode, setViewMode] = useState<"table" | "cards">("table");
	const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

	const [activeFilters, setActiveFilters] = useState<LabelFilter[]>([]);
	const [labelsByAgent, setLabelsByAgent] = useState<Map<string, AgentLabel[]>>(new Map());
	const [knownKeys, setKnownKeys] = useState<LabelKey[]>([]);

	const sparkHistory = useSparkHistory(agents);

	const agentIdsKey = useMemo(() => agents.map((a) => a.id).sort().join(","), [agents]);

	useEffect(() => {
		if (agents.length === 0) return;
		let cancelled = false;
		api.allAgentLabels()
			.then((byId) => {
				if (cancelled) return;
				const m = new Map<string, AgentLabel[]>();
				for (const [id, labels] of Object.entries(byId)) {
					m.set(id, labels as AgentLabel[]);
				}
				setLabelsByAgent(m);
			})
			.catch(() => {});
		return () => {
			cancelled = true;
		};
	}, [agentIdsKey]);

	useEffect(() => {
		api.labelKeys().then(setKnownKeys).catch(() => {});
	}, []);

	const osOptions = useMemo(() => {
		const set = new Set(agents.map((a) => a.os).filter(Boolean));
		return Array.from(set).sort();
	}, [agents]);

	const archOptions = useMemo(() => {
		const set = new Set(agents.map((a) => a.arch).filter(Boolean));
		return Array.from(set).sort();
	}, [agents]);

	const hardwareOptions = useMemo(() => {
		const set = new Set<string>();
		for (const labels of labelsByAgent.values()) {
			for (const l of labels) {
				if (l.key === "hardware") set.add(l.value);
			}
		}
		return Array.from(set).sort();
	}, [labelsByAgent]);

	const filtered = useMemo(() => {
		let result = agents;

		if (search) {
			const q = search.toLowerCase();
			result = result.filter((a) => a.hostname.toLowerCase().includes(q));
		}

		if (statusFilter !== "all") {
			result = result.filter((a) => agentStatus(a).status === statusFilter);
		}

		if (osFilter !== "all") {
			result = result.filter((a) => a.os === osFilter);
		}

		if (archFilter !== "all") {
			result = result.filter((a) => a.arch === archFilter);
		}

		if (hardwareFilter !== "all") {
			result = result.filter((a) => {
				const labels = labelsByAgent.get(a.id) ?? [];
				return labels.some((l) => l.key === "hardware" && l.value === hardwareFilter);
			});
		}

		if (activeFilters.length > 0) {
			result = result.filter((a) => {
				const labels = labelsByAgent.get(a.id) ?? [];
				return activeFilters.every((f) =>
					labels.some((l) => l.key === f.key && l.value === f.value)
				);
			});
		}

		return sortAgents(result, sort);
	}, [agents, search, statusFilter, osFilter, archFilter, hardwareFilter, sort, activeFilters, labelsByAgent]);

	const { paged, page, setPage, totalPages, total, reset: resetPage } = usePagination(filtered, pageSize);

	// Reset to the first page when the query changes. A polling data refresh does not refresh the page
	useEffect(() => {
		resetPage();
	}, [search, statusFilter, osFilter, archFilter, hardwareFilter, sort, activeFilters, pageSize, resetPage]);

	const addFilter = useCallback((key: string, value: string) => {
		setActiveFilters((prev) => {
			if (prev.some((f) => f.key === key && f.value === value)) return prev;
			return [...prev, { key, value }];
		});
	}, []);

	const removeFilter = useCallback((f: LabelFilter) => {
		setActiveFilters((prev) =>
			prev.filter((x) => !(x.key === f.key && x.value === f.value))
		);
	}, []);

	const clearFilters = useCallback(() => setActiveFilters([]), []);
	const starredSet = useMemo(() => new Set(starredIds), [starredIds]);

	if (loading && agents.length === 0) return <LoadingSpinner />;

	if (error) {
		return (
			<div style={{ padding: 24, color: themeVars.danger, fontFamily: themeVars.font }}>
				{error}
			</div>
		);
	}

	const btnStyle = (active: boolean): React.CSSProperties => ({
		padding: "5px 10px",
		fontSize: 11,
		fontFamily: themeVars.font,
		color: active ? themeVars.text : themeVars.textMuted,
		background: active ? themeVars.accentDim : "transparent",
		border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
		cursor: "pointer",
		letterSpacing: "0.03em",
	});

	return (
		<div style={{ padding: 24 }}>
			{/* Page title */}
			<div
				style={{
					fontFamily: themeVars.font,
					fontSize: 16,
					fontWeight: 600,
					color: themeVars.text,
					marginBottom: 16,
				}}
			>
				Fleet Overview
			</div>
 
			{/* Stat bar */}
			<StatBar agents={agents} />
 
			{/* Filters + view toggle */}
			<div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
				<FilterToolbar
					search={search}
					onSearchChange={setSearch}
					statusFilter={statusFilter}
					onStatusFilterChange={setStatusFilter}
					osFilter={osFilter}
					onOsFilterChange={setOsFilter}
					archFilter={archFilter}
					onArchFilterChange={setArchFilter}
					hardwareFilter={hardwareFilter}
					onHardwareFilterChange={setHardwareFilter}
					sort={sort}
					onSortChange={setSort}
					osOptions={osOptions}
					archOptions={archOptions}
					hardwareOptions={hardwareOptions}
				/>
 
				<div style={{ display: "flex", gap: 4 }}>
					<button onClick={() => setViewMode("table")} style={btnStyle(viewMode === "table")}>
						☰ Table
					</button>
					<button onClick={() => setViewMode("cards")} style={btnStyle(viewMode === "cards")}>
						⊞ Cards
					</button>
				</div>
			</div>
 
			{/* Label filter bar */}
			<LabelFilterBar
				filters={activeFilters}
				knownKeys={knownKeys}
				labelsByAgent={labelsByAgent}
				onAdd={addFilter}
				onRemove={removeFilter}
				onClear={clearFilters}
			/>
 
			{/* Table view */}
			{viewMode === "table" && (
				<div style={{ overflowX: "auto", border: `1px solid ${themeVars.border}` }}>
					<table style={{ width: "100%", borderCollapse: "collapse" }}>
						<thead>
							<TableHeader />
						</thead>
						<tbody>
							{paged.map((agent) => (
								<AgentRow
									key={agent.id}
									agent={agent}
									sparkData={sparkHistory.get(agent.id)}
									onClick={onSelectAgent}
									isStarred={starredSet.has(agent.id)}
									onToggleStar={onToggleStar}
								/>
							))}
						</tbody>
					</table>
				</div>
			)}
 
			{/* Cards view */}
			{viewMode === "cards" && (
				<div
					style={{
						display: "grid",
						gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))",
						gap: 12,
					}}
				>
					{paged.map((agent) => (
						<AgentCard
							key={agent.id}
							agent={agent}
							onClick={onSelectAgent}
							isStarred={starredSet.has(agent.id)}
							onToggleStar={onToggleStar}
						/>
					))}
				</div>
			)}
 
			{/* Pagination + page size */}
			{filtered.length > 0 && (
				<div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
					<Pagination
						page={page}
						totalPages={totalPages}
						total={total}
						pageSize={pageSize}
						onPageChange={setPage}
					/>
					<label
						style={{
							display: "flex",
							alignItems: "center",
							gap: 6,
							fontSize: 11,
							fontFamily: themeVars.font,
							color: themeVars.textDim,
							marginLeft: "auto",
						}}
					>
						Per page
						<select
							value={pageSize}
							onChange={(e) => setPageSize(Number(e.target.value))}
							style={selectStyle}
						>
							{PAGE_SIZE_OPTIONS.map((n) => (
								<option key={n} value={n}>{n}</option>
							))}
						</select>
					</label>
				</div>
			)}
 
			{/* Empty state */}
			{filtered.length === 0 && agents.length > 0 && (
				<div
					style={{
						textAlign: "center",
						padding: "40px 0",
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						fontSize: 13,
					}}
				>
					No agents match the current filters.
				</div>
			)}
 
			{agents.length === 0 && (
				<div
					style={{
						textAlign: "center",
						padding: "60px 0",
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						fontSize: 14,
					}}
				>
					No agents registered. Go to Agent Management to provision an agent.
				</div>
			)}
		</div>
	);
}