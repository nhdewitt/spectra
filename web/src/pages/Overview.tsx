import React, { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { api } from "../api";
import type { OverviewStats, OverviewLabelFilter } from "../api";
import { themeVars } from "../theme";
import { OSIcon } from "../icons";
import { Sparkline } from "../Sparkline";
import { useSparkHistory } from "../hooks";
import { Pagination } from "../hooks/usePagination";
import { useThresholds } from "../ThresholdsContext";
import type { SparkData } from "../hooks";
import { StatBlock, LoadingSpinner } from "../components";
import { LabelChip } from "../components/LabelChip";
import type { OverviewAgent, AgentLabel, LabelKey, OverviewSortKey, OverviewSortDir } from "../types";
import {
	formatBytes,
	formatUptime,
	severityColor,
	agentStatus,
	agentStatusColor,
} from "../utils";
import type { AgentStatus } from "../utils";

type SortOption = OverviewSortKey;
type SortDir = OverviewSortDir;

const PAGE_SIZE_OPTIONS = [25, 50, 100, 250];
const DEFAULT_PAGE_SIZE = 25;
const SEARCH_DEBOUNCE_MS = 300;
const POLL_INTERVAL_MS = 10_000;

// Direction each sort column is displayed in - "worst/highest first" for
// everything except hostname. Matches the server's own severity/status
// ordering (offline=0, crit=1, warn=2, stale=3, online=4 - ascending
// on "status" surfaces the worst case first).
const DEFAULT_SORT_ORDER: Record<SortOption, SortDir> = {
	platform: "asc",
	arch: "asc",
	severity: "desc",
	status: "asc",
	hostname: "asc",
	os: "asc",
	cpu: "desc",
	memory: "desc",
	disk: "desc",
	temp: "desc",
	uptime: "desc",
	procs: "desc",
	net: "desc",
	last_seen: "desc",
};

// Severity is the resting sort: it has no column of its own (it sums cpu, disk
// and memory), so a third click on any header returns here rather than leaving
// the default unreachable once the user has sorted by something else.
const DEFAULT_SORT: SortOption = "severity";

// Cards are always alphabetical by hostname. Sorting a grid by a metric makes
// tiles jump between positions on every poll, so the view pins its own order
// regardless of what the table is sorted by.
const CARD_SORT: SortOption = "hostname";
const CARD_SORT_DIR: SortDir = "asc";

interface OverviewProps {
	stats: OverviewStats | null;
	onSelectAgent: (agent: OverviewAgent) => void;
	starredIds: string[];
	onToggleStar: (agentId: string) => void;
}

interface LabelFilter {
	key: string;
	value: string;
}

// --- Stat Bar ---

function StatBar({ stats }: { stats: OverviewStats | null }) {
	const c = stats ?? { total: 0, online: 0, stale: 0, offline: 0, warn: 0, crit: 0, reboot: 0 };

	return (
		<div
			style={{
				display: "flex",
				gap: 0,
				marginBottom: 20,
				border: `1px solid ${themeVars.border}`,
			}}
		>
			<StatBarItem label="Total Agents" value={c.total} color={themeVars.text} />
			<StatBarItem label="Online" value={c.online} color={themeVars.ok} />
			<StatBarItem label="Stale" value={c.stale} color={themeVars.warn} />
			<StatBarItem label="Offline" value={c.offline} color={themeVars.textDim} />
			<StatBarItem label="Warning" value={c.warn} color={themeVars.warn} />
			<StatBarItem label="Critical" value={c.crit} color={themeVars.danger} />
			<StatBarItem label="Reboot Req." value={c.reboot} color={themeVars.warn} />
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
	const thresholds = useThresholds();
	const { status, reasons } = agentStatus(agent, thresholds);
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
	onAdd,
	onRemove,
	onClear,
}: {
	filters: LabelFilter[];
	knownKeys: LabelKey[];
	onAdd: (key: string, value: string) => void;
	onRemove: (f: LabelFilter) => void;
	onClear: () => void;
}) {
	const [pickerKey, setPickerKey] = useState("");
	const [pickerValue, setPickerValue] = useState("");
	const [pickerValues, setPickerValues] = useState<string[]>([]);

	const userKeys = useMemo(() => knownKeys.filter((k) => k.source === "user"), [knownKeys]);

	// Fetch distinct values for the chosen key on demand rather than deriving
	// them from a bulk per-agent label dump.
	useEffect(() => {
		if (!pickerKey) { setPickerValues([]); return; }
		let cancelled = false;
		api.labelValues(pickerKey)
			.then((vals) => { if (!cancelled) setPickerValues(vals); })
			.catch(() => { if (!cancelled) setPickerValues([]); });
		return () => { cancelled = true };
	}, [pickerKey]);

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

/**
 * One clickable column header. The button fills the cell so the whole header is
 * a hit target, and carries the padding headerStyle would otherwise put on the
 * th. The arrow renders even when inactive so the column does not change width
 * on the first click. */
function SortableTh({
	label,
	columnKey,
	sort,
	sortDir,
	onSort,
	align = "right",
	width,
}: {
	label: string;
	columnKey: SortOption;
	sort: SortOption;
	sortDir: SortDir;
	onSort: (key: SortOption) => void;
	align?: "left" | "right";
	width?: number;
}) {
	const active = sort === columnKey;

	return (
		<th
			aria-sort={active ? (sortDir === "asc" ? "ascending" : "descending") : "none"}
			style={{ ...headerStyle, textAlign: align, width, padding: 0 }}
		>
			<button
				type="button"
				onClick={() => onSort(columnKey)}
				title={`Sort by ${label}`}
				style={{
					display: "flex",
					alignItems: "center",
					justifyContent: align === "right" ? "flex-end" : "flex-start",
					gap: 4,
					width: "100%",
					padding: "8px 10px",
					margin: 0,
					border: "none",
					background: "transparent",
					fontSize: 10,
					fontFamily: themeVars.font,
					color: active ? themeVars.text : themeVars.textDim,
					letterSpacing: "0.05em",
					textTransform: "uppercase",
					cursor: "pointer",
					userSelect: "none",
					whiteSpace: "nowrap",
				}}
			>
				{label}
				<span
					aria-hidden="true"
					style={{ width: 8, flexShrink: 0, color: active ? themeVars.accent : "transparent" }}
				>
					{active && sortDir === "desc" ? "\u25bc" : "\u25b2"}
				</span>
			</button>
		</th>
	);	
}

function TableHeader({
	sort,
	sortDir,
	onSort,
}: {
	sort: SortOption;
	sortDir: SortDir;
	onSort: (key: SortOption) => void;
}) {
	const common = { sort, sortDir, onSort };

	return (
		<tr style={{ borderBottom: `1px solid ${themeVars.border}` }}>
			{/* Star toggle - nothing to sort. */}
			<th style={{ ...headerStyle, textAlign: "left", width: 28 }} />
			<SortableTh label="Hostname" columnKey="hostname" align="left" {...common} />
			<SortableTh label="Status" columnKey="status" align="left" width={80} {...common} />
			<SortableTh label="OS / Platform" columnKey="os" align="left" width={100} {...common} />
			<SortableTh label="CPU" columnKey="cpu" width={60} {...common} />
			{/* Bar graphic for the column to its left. */}
			<th style={{ ...headerStyle, width: 60 }} />
			<SortableTh label="Memory" columnKey="memory" width={60} {...common} />
			<th style={{ ...headerStyle, width: 60 }} />
			<SortableTh label="Disk" columnKey="disk" width={60} {...common} />
			<th style={{ ...headerStyle, width: 60 }} />
			<SortableTh label="Temp" columnKey="temp" width={50} {...common} />
			{/* CPU Trend is a sparkline of history - there is no single value to
			    order by, and sorting it by current CPU would duplicate the CPU
			    column while implying the trend itself was ranked. */}
			<th style={{ ...headerStyle, width: 70 }}>CPU Trend</th>
			<SortableTh label="Uptime" columnKey="uptime" width={70} {...common} />
			<SortableTh label="Last Seen" columnKey="last_seen" width={70} {...common} />
			<SortableTh label="Procs" columnKey="procs" width={50} {...common} />
			<SortableTh label="Net RX/TX" columnKey="net" width={100} {...common} />
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
	const thresholds = useThresholds();

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
						background: agentStatusColor(agentStatus(agent, thresholds).status),
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
	const thresholds = useThresholds();
	const { status, reasons } = agentStatus(agent, thresholds);

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

// --- Overview Page ---

export function Overview({ stats, onSelectAgent, starredIds, onToggleStar }: OverviewProps) {
	// Query state
	const [searchInput, setSearchInput] = useState("");
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState<AgentStatus | "all">("all");
	const [osFilter, setOsFilter] = useState("all");
	const [archFilter, setArchFilter] = useState("all");
	const [hardwareFilter, setHardwareFilter] = useState("all");
	// One object rather than two useStates: a column switch changes key and
	// direction together, and splitting them makes that two renders with a
	// transiently mismatched pair in between.
	const [sortState, setSortState] = useState<{ key: SortOption; dir: SortDir }>({
		key: DEFAULT_SORT,
		dir: DEFAULT_SORT_ORDER[DEFAULT_SORT],
	});
	const { key: sort, dir: sortDir } = sortState;
	const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
	const [activeFilters, setActiveFilters] = useState<LabelFilter[]>([]);
	const [page, setPage] = useState(0);

	const [viewMode, setViewMode] = useState<"table" | "cards">("table");

	// Facet options - fetched once via labelValues
	const [osOptions, setOsOptions] = useState<string[]>([]);
	const [archOptions, setArchOptions] = useState<string[]>([]);
	const [hardwareOptions, setHardwareOptions] = useState<string[]>([]);
	const [knownKeys, setKnownKeys] = useState<LabelKey[]>([]);

	// Server-driven page data
	const [agentsPage, setAgentsPage] = useState<OverviewAgent[]>([]);
	const [total, setTotal] = useState(0);
	const [totalPages, setTotalPages] = useState(1);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	const sparkHistory = useSparkHistory(agentsPage);

	useEffect(() => {
		api.labelValues("os").then(setOsOptions).catch(() => {});
		api.labelValues("arch").then(setArchOptions).catch(() => {});
		api.labelValues("hardware").then(setHardwareOptions).catch(() => {});
		api.labelKeys().then(setKnownKeys).catch(() => {});
	}, []);

	// Debounce search input into the actual query value, resetting to page 1 once it settles
	useEffect(() => {
		const t = setTimeout(() => {
			setSearch(searchInput);
			setPage(0);
		}, SEARCH_DEBOUNCE_MS);
		return () => clearTimeout(t);
	}, [searchInput]);

	const changeStatusFilter = useCallback((v: AgentStatus | "all") => { setStatusFilter(v); setPage(0); }, []);
	const changeOsFilter = useCallback((v: string) => { setOsFilter(v); setPage(0); }, []);
	const changeArchFilter = useCallback((v: string) => { setArchFilter(v); setPage(0); }, []);
	const changeHardwareFilter = useCallback((v: string) => { setHardwareFilter(v); setPage(0); }, []);
	// Header clicks cycle: default direction -> reversed -> back to severity.
	// The third step matters because severity has no column of its own; without
	// it the resting sort would be unreachable once any header was clicked.
	const changeSort = useCallback((v: SortOption) => {
		setSortState((prev) => {
			if (prev.key !== v) return { key: v, dir: DEFAULT_SORT_ORDER[v] };
			if (prev.dir === DEFAULT_SORT_ORDER[v]) {
				return { key: v, dir: prev.dir === "asc" ? "desc" : "asc" };
			}
			return { key: DEFAULT_SORT, dir: DEFAULT_SORT_ORDER[DEFAULT_SORT] };
		});
		setPage(0);
	}, []);
	const changePageSize = useCallback((n: number) => { setPageSize(n); setPage(0); }, []);

	const addFilter = useCallback((key: string, value: string) => {
		setActiveFilters((prev) => {
			if (prev.some((f) => f.key === key && f.value === value)) return prev;
			return [...prev, { key, value }];
		});
		setPage(0);
	}, []);

	const removeFilter = useCallback((f: LabelFilter) => {
		setActiveFilters((prev) => prev.filter((x) => !(x.key === f.key && x.value === f.value)));
		setPage(0);
	}, []);

	const clearFilters = useCallback(() => { setActiveFilters([]); setPage(0); }, []);

	// Cards ignore the table's sort entirely, so the effective sort, not the
	// sort state, is what the query and the cache key are built from.
	const isCards = viewMode === "cards";
	const effectiveSort = isCards ? CARD_SORT : sort;
	const effectiveSortDir = isCards ? CARD_SORT_DIR : sortDir;

	const filtersKey = useMemo(() => {
		const labelsPart = [
			...activeFilters,
			...(hardwareFilter !== "all" ? [{ key: "hardware", value: hardwareFilter }] : []),
		]
			.map((f) => `${f.key}:${f.value}`)
			.sort()
			.join(",");
		return [search, statusFilter, osFilter, archFilter, effectiveSort, effectiveSortDir, pageSize, labelsPart].join("|");
	}, [search, statusFilter, osFilter, archFilter, hardwareFilter, effectiveSort, effectiveSortDir, pageSize, activeFilters]);

	const buildParams = useCallback((withCount: boolean): Parameters<typeof api.overviewPage>[0] => {
		const labels: OverviewLabelFilter[] = [...activeFilters];
		if (hardwareFilter !== "all") labels.push({ key: "hardware", value: hardwareFilter });
		return {
			page: page + 1,
			size: pageSize,
			sort: effectiveSort,
			order: effectiveSortDir,
			status: statusFilter,
			os: osFilter,
			arch: archFilter,
			search: search || undefined,
			labels,
			count: withCount,
		};
	}, [page, pageSize, effectiveSort, effectiveSortDir, statusFilter, osFilter, archFilter, search, activeFilters, hardwareFilter]);

	const filtersKeyRef = useRef<string | null>(null);

	// Foreground fetch
	useEffect(() => {
		let cancelled = false;
		const withCount = filtersKeyRef.current !== filtersKey;
		filtersKeyRef.current = filtersKey;

		setLoading(true);
		api.overviewPage(buildParams(withCount))
			.then((res) => {
				if (cancelled) return;
				setAgentsPage(res.agents);
				if (res.total != null) setTotal(res.total);
				if (res.total_pages != null) setTotalPages(res.total_pages);
				setError(null);
			})
			.catch((err) => {
				if (cancelled) return;
				setError(err instanceof Error ? err.message : "Failed to load agents");
			})
			.finally(() => {
				if (!cancelled) setLoading(false);
			});

		return () => { cancelled = true; };
	}, [filtersKey, buildParams]);

	// Background refresh
	useEffect(() => {
		const id = setInterval(() => {
			api.overviewPage(buildParams(false))
				.then((res) => setAgentsPage(res.agents))
				.catch(() => {});
		}, POLL_INTERVAL_MS);
		return () => clearInterval(id);
	}, [buildParams]);

	const starredSet = useMemo(() => new Set(starredIds), [starredIds]);

	if (loading && agentsPage.length === 0) return <LoadingSpinner />;

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

	const fleetIsEmpty = stats != null && stats.total === 0;

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
			<StatBar stats={stats} />
 
			{/* Filters + view toggle */}
			<div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
				<FilterToolbar
					search={searchInput}
					onSearchChange={setSearchInput}
					statusFilter={statusFilter}
					onStatusFilterChange={changeStatusFilter}
					osFilter={osFilter}
					onOsFilterChange={changeOsFilter}
					archFilter={archFilter}
					onArchFilterChange={changeArchFilter}
					hardwareFilter={hardwareFilter}
					onHardwareFilterChange={changeHardwareFilter}
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
				onAdd={addFilter}
				onRemove={removeFilter}
				onClear={clearFilters}
			/>
 
			{/* Table view */}
			{viewMode === "table" && (
				<div style={{ overflowX: "auto", border: `1px solid ${themeVars.border}` }}>
					<table style={{ width: "100%", borderCollapse: "collapse" }}>
						<thead>
							<TableHeader sort={sort} sortDir={sortDir} onSort={changeSort} />
						</thead>
						<tbody>
							{agentsPage.map((agent) => (
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
					{agentsPage.map((agent) => (
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
			{total > 0 && (
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
							onChange={(e) => changePageSize(Number(e.target.value))}
							style={selectStyle}
						>
							{PAGE_SIZE_OPTIONS.map((n) => (
								<option key={n} value={n}>{n}</option>
							))}
						</select>
					</label>
				</div>
			)}
 
			{/* Empty states */}
			{total === 0 && !fleetIsEmpty && (
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
 
			{fleetIsEmpty && (
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