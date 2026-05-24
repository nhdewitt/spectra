import React, { useState, useCallback, useEffect, useRef, useMemo } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { formatBytes, statusColor } from "../utils";
import { tableHeaderStyle, tableCellStyle, tableMutedCellStyle, LoadingSpinner } from "../components/ui";
import type { OverviewAgent, CommandResponse, CommandEntry } from "../types";
import { Pagination, usePagination } from "../hooks/usePagination";

interface DiagnosticsProps {
	agents: OverviewAgent[];
	selectedAgent: OverviewAgent | null;
	onSelectAgent: (agent: OverviewAgent) => void;
}

type DiagTool = "ping" | "traceroute" | "netstat" | "disk" | "logs";
type CardStatus = "idle" | "running" | "complete" | "error";

interface CardDef {
	id: DiagTool;
	label: string;
	icon: string;
	desc: string;
	needsTarget: boolean;
	needsOptions: boolean;
}

const CARDS: CardDef[] = [
	{ id: "ping", label: "Ping", icon: "◆", desc: "ICMP connectivity test", needsTarget: true, needsOptions: false },
	{ id: "traceroute", label: "Traceroute", icon: "⟶", desc: "Network path tracing", needsTarget: true, needsOptions: false },
	{ id: "netstat" , label: "Netstat", icon: "▤", desc: "Active connections list", needsTarget: false, needsOptions: false },
	{ id: "disk", label: "Disk Scan", icon: "◉", desc: "Top files/directories", needsTarget: false, needsOptions: true },
	{ id: "logs", label: "Fetch Logs", icon: "≡", desc: "System logs by severity", needsTarget: false, needsOptions: true },
];

function useCommandPoller(cmdId: string | null) {
	const [entry, setEntry] = useState<CommandEntry | null>(null);
	const [error, setError] = useState<string | null>(null);
	const intervalRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

	useEffect(() => {
		if (!cmdId) {
			setEntry(null);
			setError(null);
			return;
		}

		const poll = async () => {
			try {
				const result = await api.commandResult(cmdId);
				setEntry(result);
				if (result.done) {
					clearInterval(intervalRef.current);
				}
			} catch (err) {
				setError(err instanceof Error ? err.message : "Failed to poll");
				clearInterval(intervalRef.current);
			}
		};

		poll();
		intervalRef.current = setInterval(poll, 1000);
		return () => clearInterval(intervalRef.current);
	}, [cmdId]);

	return { entry, error };
}

interface LogEntry {
	timestamp: number;
	source: string;
	level: string;
	message: string;
	pid?: number;
	process_name?: string;
}

function levelColor(level: string): string {
	switch (level) {
		case "EMERGENCY":
		case "ALERT":
		case "CRITICAL":
		case "ERROR":
			return themeVars.danger;
		case "WARNING":
			return themeVars.warn;
		case "NOTICE":
			return themeVars.accent;
		default:
			return themeVars.textMuted;
	}
}

function severityOrder(level: string): number {
	const order: Record<string, number> = {
		EMERGENCY: 0, ALERT: 1, CRITICAL: 2, ERROR: 3,
		WARNING: 4, NOTICE: 5, INFO: 6, DEBUG: 7,
	};
	return order[level] ?? 99;
}

interface DiskReport {
	root: string;
	top_dirs: Array<{ path: string; size: number; count?: number }>;
	top_files: Array<{ path: string; size: number }>;
	scanned_dirs: number;
	scanned_files: number;
	error_count: number;
	partial: boolean;
	duration_ms: number;
}

interface NetworkReport {
	action: string;
	target?: string;
	raw_output?: string;
	netstat?: Array<{
		proto: string;
		local_addr: string;
		local_port: number;
		remote_addr: string;
		remote_port: number;
		state: string;
	}>;
	ping_results?: Array<{
		seq: number;
		success: boolean;
		rtt: number;
		response: string;
		peer: string;
	}>;
}

const btnStyle: React.CSSProperties = {
	padding: "6px 14px",
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.accentDim,
	border: `1px solid ${themeVars.accent}`,
	cursor: "pointer",
	textTransform: "uppercase",
	letterSpacing: "0.03em",
};

const inputStyle: React.CSSProperties = {
	padding: "6px 10px",
	fontSize: 12,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.surface,
	border: `1px solid ${themeVars.border}`,
};

const LOG_LEVELS = ["DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY"];

function LogResultsInline({ entries }: { entries: LogEntry[] }) {
	const [activeLevel, setActiveLevel] = useState<string | null>(null);

	const levels = useMemo(() => {
		const counts: Record<string, number> = {};
		for (const e of entries) {
			counts[e.level] = (counts[e.level] || 0) + 1;
		}
		return counts;
	}, [entries]);

	const filtered = activeLevel
		? entries.filter((e) => severityOrder(e.level) <= severityOrder(activeLevel))
		: entries;

	const { paged, page, setPage, totalPages, total, reset } = usePagination(filtered, 50);

	useEffect(() => { reset(); }, [activeLevel, reset]);

	if (entries.length === 0) {
		return (
			<div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12, padding: 16 }}>
				No log entries found at or below this severity level.
			</div>
		);
	}

	const filterBtn = (label: string, active: boolean, onClick: () => void, color?: string): React.ReactNode => (
		<button
			onClick={onClick}
			style={{
				padding: "3px 10px",
				fontSize: 10,
				fontFamily: themeVars.font,
				color: active ? themeVars.text : (color ?? themeVars.textMuted),
				background: active ? themeVars.accentDim : "transparent",
				border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
				cursor: "pointer",
				textTransform: "uppercase",
				letterSpacing: "0.03em",
			}}
		>
			{label}
		</button>
	);

    return (
        <div>
            {/* Filters */}
            <div style={{ display: "flex", gap: 4, marginBottom: 12, flexWrap: "wrap", alignItems: "center" }}>
                {filterBtn(`All (${entries.length})`, !activeLevel, () => setActiveLevel(null))}
                {Object.entries(levels)
                    .sort(([a], [b]) => severityOrder(a) - severityOrder(b))
                    .map(([level, count]) =>
                        filterBtn(
                            `${level} (${count})`,
                            activeLevel === level,
                            () => setActiveLevel(activeLevel === level ? null : level),
                            levelColor(level)
                        )
                    )}
                <span style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginLeft: 8 }}>
                    {filtered.length} entries
                </span>
            </div>

            {/* Entries */}
            <div
                style={{
                    border: `1px solid ${themeVars.border}`,
                    maxHeight: 400,
                    overflowY: "auto",
                }}
            >
                {paged.map((e, i) => (
                    <div
                        key={i}
                        style={{
                            padding: "4px 8px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            borderBottom: `1px solid ${themeVars.border}`,
                            display: "grid",
                            gridTemplateColumns: "130px 70px 100px 1fr",
                            gap: 8,
                            background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
                        }}
                    >
                        <span style={{ color: themeVars.textDim, whiteSpace: "nowrap" }}>
                            {new Date(e.timestamp * 1000).toLocaleString(undefined, {
                                month: "short",
                                day: "numeric",
                                hour: "2-digit",
                                minute: "2-digit",
                                second: "2-digit",
                            })}
                        </span>
                        <span style={{ color: levelColor(e.level), fontWeight: 600, whiteSpace: "nowrap" }}>
                            {e.level}
                        </span>
                        <span
                            style={{
                                color: themeVars.accent,
                                whiteSpace: "nowrap",
                                overflow: "hidden",
                                textOverflow: "ellipsis",
                            }}
                        >
                            {e.source}
                        </span>
                        <span
                            style={{
                                color: themeVars.text,
                                whiteSpace: "nowrap",
                                overflow: "hidden",
                                textOverflow: "ellipsis",
                            }}
                            title={e.message}
                        >
                            {e.message}
                        </span>
                    </div>
                ))}
            </div>

            <Pagination
                page={page}
                totalPages={totalPages}
                total={total}
                pageSize={50}
                onPageChange={setPage}
            />
        </div>
    );	
}

function DiskResultsInline({ report }: { report: DiskReport }) {
	return (
		<div>
			<div
				style={{
					display: "flex",
					gap: 16,
					marginBottom: 12,
					fontSize: 11,
					fontFamily: themeVars.font,
					color: themeVars.textDim,
				}}
			>
				<span>Root: {report.root}</span>
				<span>Scanned: {report.scanned_dirs} dirs, {report.scanned_files} files</span>
				<span>Duration: {report.duration_ms}ms</span>
				{report.error_count > 0 && (
					<span style={{ color: themeVars.warn }}>
						{report.error_count} error{report.error_count === 1 ? "" : "s"}
					</span>
				)}
				{report.partial && <span style={{ color: themeVars.warn }}>Partial scan</span>}
			</div>

			{report.top_dirs.length > 0 && (
				<div style={{ marginBottom: 16 }}>
					<div 
						style={{
							fontSize: 11,
							fontFamily: themeVars.font,
							color: themeVars.textDim,
							textTransform: "uppercase",
							letterSpacing: "0.04em",
							marginBottom: 6,
						}}
					>
						Largest Directories
					</div>
					<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
						<thead>
							<tr>
								<th style={tableHeaderStyle}>Path</th>
								<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Size</th>
								<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Files</th>
							</tr>
						</thead>
						<tbody>
							{report.top_dirs.map((d, i) => (
								<tr key={d.path} style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}>
									<td style={tableCellStyle}>{d.path}</td>
									<td style={{ ...tableMutedCellStyle, textAlign: "right" }}>{formatBytes(d.size)}</td>
									<td style={{ ...tableMutedCellStyle, textAlign: "right" }}>{d.count ?? "—"}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}


			{report.top_files.length > 0 && (
				<div>
					<div
						style={{
							fontSize: 11,
							fontFamily: themeVars.font,
							color: themeVars.textDim,
							textTransform: "uppercase",
							letterSpacing: "0.04em",
							marginBottom: 6,
						}}
					>
						Largest Files
					</div>
					<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
						<thead>
							<tr>
								<th style={tableHeaderStyle}>Path</th>
								<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Size</th>
							</tr>
						</thead>
						<tbody>
							{report.top_files.map((f, i) => (
								<tr key={f.path} style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}>
									<td style={tableCellStyle}>{f.path}</td>
									<td style={{ ...tableMutedCellStyle, textAlign: "right" }}>{formatBytes(f.size)}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</div>
	);
}

function PingResultsInline({ report }: { report: NetworkReport }) {
	if (report.ping_results) {
		return (
			<div>
				<div
					style={{
						fontSize: 11,
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						marginBottom: 8
					}}
				>
					Ping {report.target}
				</div>
				{report.ping_results.map((p, i) => (
					<div
						key={i}
						style={{
							padding: "3px 8px",
							fontSize: 11,
							fontFamily: themeVars.font,
							color: p.success ? themeVars.ok : themeVars.danger,
							background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
						}}
					>
						seq={p.seq} {p.response} from {p.peer}{" "}{p.success && `rtt=${(p.rtt / 1_000_000).toFixed(2)}ms`}
					</div>
				))}
			</div>
		);
	}
	return null;
}

function TracerouteResultsInline({ report }: { report: NetworkReport }) {
	if (report.raw_output) {
		return (
			<pre
				style={{
					fontFamily: themeVars.font,
					fontSize: 11,
					color: themeVars.text,
					whiteSpace: "pre-wrap",
					wordBreak: "break-all",
					margin: 0,
					padding: 8,
					background: themeVars.surfaceHover,
					border: `1px solid ${themeVars.border}`,
					maxHeight: 400,
					overflowY: "auto",
				}}
			>
				{report.raw_output}
			</pre>
		);
	}
	return null;
}

function NetstatResultsInline({ entries }: {
	entries: Array<{
		proto: string;
		local_addr: string;
		local_port: number;
		remote_addr: string;
		remote_port: number;
		state: string;
	}>;
}) {
	const [protoFilter, setProtoFilter] = useState<string | null>(null);
	const [stateFilter, setStateFilter] = useState<string | null>(null);
	const [search, setSearch] = useState("");

	const protos = useMemo(() => {
		const counts: Record<string, number> = {};
		for (const e of entries) counts[e.proto] = (counts[e.proto] || 0) + 1;
		return counts;
	}, [entries]);

	const states = useMemo(() => {
		const counts: Record<string, number> = {};
		for (const e of entries) counts[e.state] = (counts[e.state] || 0) + 1;
		return counts;
	}, [entries]);

	const filtered = useMemo(() => {
		return entries.filter((e) => {
			if (protoFilter && e.proto !== protoFilter) return false;
			if (stateFilter && e.state !== stateFilter) return false;
			if (search) {
				const q = search.toLowerCase();
				const line = `${e.local_addr}:${e.local_port} ${e.remote_addr}:${e.remote_port}`.toLowerCase();
				if (!line.includes(q)) return false;
			}
			return true;
		});
	}, [entries, protoFilter, stateFilter, search]);

	const stateColor = (state: string): string => {
		switch (state) {
			case "ESTABLISHED":
				return themeVars.ok;
			case "LISTEN":
				return themeVars.accent;
			case "TIME_WAIT":
			case "CLOSE_WAIT":
				return themeVars.warn;
			case "SYN_SENT":
			case "SYN_RECV":
				return themeVars.warn;
			default:
				return themeVars.textMuted;
		}
	};

	const filterBtn = (label: string, active: boolean, onClick: () => void): React.ReactNode => (
		<button
			onClick={onClick}
			style={{
				padding: "3px 10px",
				fontSize: 10,
				fontFamily: themeVars.font,
				color: active ? themeVars.text : themeVars.textMuted,
				background: active ? themeVars.accentDim : "transparent",
				border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
				cursor: "pointer",
				textTransform: "uppercase",
				letterSpacing: "0.03em",
			}}
		>
			{label}
		</button>
	);

	return (
		<div>
			<div style={{ display: "flex", gap: 8, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
				<div style={{ display: "flex", gap: 4 }}>
					{filterBtn("All", !protoFilter, () => setProtoFilter(null))}
					{Object.entries(protos).map(([proto, count]) =>
						filterBtn(`${proto} (${count})`, protoFilter === proto, () => setProtoFilter(protoFilter === proto ? null : proto))
					)}
				</div>
				<div style={{ display: "flex", gap: 4 }}>
					{filterBtn("All States", !stateFilter, () => setStateFilter(null))}
					{Object.entries(states)
						.sort(([, a], [, b]) => b - a)
						.map(([state, count]) =>
							filterBtn(`${state} (${count})`, stateFilter === state, () => setStateFilter(stateFilter === state ? null : state))
						)}
				</div>
				<input
					type="text"
					value={search}
					onChange={(e) => setSearch(e.target.value)}
					placeholder="Filter by address..."
					style={{ ...inputStyle, flex: "0 1 180px", fontSize: 11, padding: "3px 8px" }}
				/>
			</div>

			<div style={{ maxHeight: 400, overflowY: "auto", border: `1px solid ${themeVars.border}` }}>
				<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
					<thead>
						<tr>
							<th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Proto</th>
							<th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Local</th>
							<th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>Remote</th>
							<th style={{ ...tableHeaderStyle, position: "sticky", top: 0, background: themeVars.bg }}>State</th>
						</tr>
					</thead>
					<tbody>
						{filtered.map((n, i) => (
							<tr key={i} style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}>
								<td style={tableMutedCellStyle}>{n.proto}</td>
								<td style={tableCellStyle}>{n.local_addr}:{n.local_port}</td>
								<td style={tableCellStyle}>{n.remote_addr}:{n.remote_port}</td>
								<td style={{ ...tableCellStyle, color: stateColor(n.state) }}>{n.state}</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>

			<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
				{filtered.length} of {entries.length} connections
			</div>
		</div>
	);
}

export function Diagnostics({ agents, selectedAgent, onSelectAgent }: DiagnosticsProps) {
	const [remoteTarget, setRemoteTarget] = useState("");
	const [targetFlash, setTargetFlash] = useState(false);
	const targetRef = useRef<HTMLInputElement>(null);

	const [cardStatus, setCardStatus] = useState<Record<DiagTool, CardStatus>>({
		ping: "idle", traceroute: "idle", netstat: "idle", disk: "idle", logs: "idle",
	})

	const [activeCmd, setActiveCmd] = useState<string | null>(null);
	const [activeTool, setActiveTool] = useState<DiagTool | null>(null);
	const { entry, error: pollError } = useCommandPoller(activeCmd);

	const [showDiskOptions, setShowDiskOptions] = useState(false);
	const [showLogOptions, setShowLogOptions] = useState(false);
	const [diskPath, setDiskPath] = useState("");
	const [diskTopN, setDiskTopN] = useState(20);
	const [logLevel, setLogLevel] = useState("WARNING");

	const [results, setResults] = useState<Record<string, CommandEntry>>({});

	const [sendError, setSendError] = useState<string | null>(null);

	const confirmedTarget = useRef(false);
	const pendingTool = useRef<DiagTool | null>(null);

	useEffect(() => {
		if (!entry || !activeTool) return;
		if (entry.done) {
			setResults((prev) => ({ ...prev, [activeTool]: entry }));
			setCardStatus((prev) => ({
				...prev,
				[activeTool]: entry.result?.error ? "error" : "complete",
			}));
			setActiveCmd(null);
		}
	}, [entry, activeTool]);

	useEffect(() => {
		if (pollError && activeTool) {
			setCardStatus((prev) => ({ ...prev, [activeTool]: "error" }));
			setActiveCmd(null);
		}
	}, [pollError, activeTool]);

	useEffect(() => {
		confirmedTarget.current = false;
		pendingTool.current = null;
	}, [remoteTarget]);

	const flashTarget = useCallback(() => {
		targetRef.current?.focus();
		setTargetFlash(true);
		setTimeout(() => setTargetFlash(false), 1500);
	}, []);

	const runCommand = useCallback(async (tool: DiagTool, fn: () => Promise<CommandResponse>) => {
		setSendError(null);
		setActiveTool(tool);
		setCardStatus((prev) => ({ ...prev, [tool]: "running" }));
		setShowDiskOptions(false);
		setShowLogOptions(false);
		try {
			const res = await fn();
			setActiveCmd(res.command_id);
		} catch (err) {
			setSendError(err instanceof Error ? err.message : "Failed to send");
			setCardStatus((prev) => ({ ...prev, [tool]: "error" }));
		}
	}, []);

	const handleCardClick = useCallback((card: CardDef) => {
		if (!selectedAgent) return;

		const agentId = selectedAgent.id;
		const isRunning = Object.values(cardStatus).some((s) => s === "running");
		if (isRunning) return;

		if (card.needsTarget && !remoteTarget.trim()) {
			flashTarget();
			return;
		}
		if (card.id === "disk") {
			setShowLogOptions(false);
			setShowDiskOptions((prev) => !prev);
			return;
		}
		if (card.id === "logs") {
			setShowDiskOptions(false);
			setShowLogOptions((prev) => !prev);
			return;
		}

		switch (card.id) {
			case "ping":
			case "traceroute": {
				// If target field has content, flash it to confirm, then run on second click
				if (!confirmedTarget.current) {
					flashTarget();
					confirmedTarget.current = true;
					pendingTool.current = card.id;
					return;
				}
				confirmedTarget.current = false;
				pendingTool.current = null;
				const action = card.id === "ping" ? "ping" : "traceroute";
				runCommand(card.id, () => api.triggerNetwork(agentId, action, remoteTarget.trim()));
				break;
			}
			case "netstat":
				runCommand("netstat", () => api.triggerNetwork(agentId, "netstat"));
				break;
		}
	}, [selectedAgent, remoteTarget, cardStatus, flashTarget, runCommand]);

	const renderResult = (tool: DiagTool) => {
		const result = results[tool];
		if (!result) return null;

		if (result.result?.error) {
			return (
				<div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
					Error: {result.result.error}
				</div>
			);
		}

		const payload = result.result?.payload;
		if (!payload) {
			return (
				<div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
					No data returned.
				</div>
			);
		}

		switch (tool) {
			case "logs":
				return <LogResultsInline entries={(payload as LogEntry[]) ?? []} />;
			case "disk":
				return <DiskResultsInline report={payload as DiskReport} />;
			case "ping":
				return <PingResultsInline report={payload as NetworkReport} />;
			case "traceroute":
				return <TracerouteResultsInline report={payload as NetworkReport} />;
			case "netstat":
				return (payload as NetworkReport).netstat
					? <NetstatResultsInline entries={(payload as NetworkReport).netstat!} />
					: null;
			default:
				return (
					<pre style={{ fontFamily: themeVars.font, fontSize: 11, color: themeVars.text, whiteSpace: "pre-wrap", margin: 0 }}>
						{JSON.stringify(payload, null, 2)}
					</pre>
				);
		}
	};

	const isRunning = Object.values(cardStatus).some((s) => s === "running");

	const displayTool = activeTool ?? null;

    return (
        <div style={{ padding: 24 }}>
            {/* Title */}
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 18,
                    fontWeight: 600,
                    color: themeVars.text,
                    marginBottom: 20,
                }}
            >
                Diagnostics
                {selectedAgent && (
                    <span style={{ color: themeVars.textDim, fontWeight: 400 }}>
                        {" "}— {selectedAgent.hostname}
                    </span>
                )}
            </div>
 
            {/* Target section */}
            <div
                style={{
                    border: `1px solid ${themeVars.border}`,
                    padding: 16,
                    marginBottom: 20,
                }}
            >
                <div
                    style={{
                        fontSize: 10,
                        fontFamily: themeVars.font,
                        color: themeVars.accent,
                        textTransform: "uppercase",
                        letterSpacing: "0.06em",
                        marginBottom: 12,
                    }}
                >
                    Target
                </div>
 
                <div style={{ display: "flex", gap: 16, alignItems: "flex-end", flexWrap: "wrap" }}>
                    {/* Agent */}
                    <div>
                        <div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginBottom: 4 }}>
                            Agent
                        </div>
                        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                            <select
                                value={selectedAgent?.id ?? ""}
                                onChange={(e) => {
                                    const agent = agents.find((a) => a.id === e.target.value);
                                    if (agent) onSelectAgent(agent);
                                }}
                                style={{ ...inputStyle, minWidth: 280, cursor: "pointer" }}
                            >
                                <option value="" disabled>Select an agent...</option>
                                {agents.map((a) => (
                                    <option key={a.id} value={a.id}>
                                        {a.hostname} — {a.os} {a.platform} {a.arch}
                                    </option>
                                ))}
                            </select>
                            {selectedAgent && (
                                <span style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, fontFamily: themeVars.font, color: themeVars.text }}>
                                    <span style={{ width: 7, height: 7, borderRadius: "50%", background: statusColor(selectedAgent) }} />
                                    {selectedAgent.hostname}
                                </span>
                            )}
                        </div>
                    </div>
 
                    {/* Remote target */}
                    <div>
                        <div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginBottom: 4 }}>
                            Remote Target (ping/traceroute)
                        </div>
                        <input
                            ref={targetRef}
                            type="text"
                            value={remoteTarget}
                            onChange={(e) => setRemoteTarget(e.target.value)}
							onKeyDown={(e) => {
								if (e.key === "Enter" && remoteTarget.trim() && selectedAgent && !isRunning) {
									runCommand("ping", () => api.triggerNetwork(selectedAgent.id, "ping", remoteTarget.trim()));
								}
							}}
                            placeholder="IP or hostname"
                            style={{
                                ...inputStyle,
                                width: 220,
                                borderColor: targetFlash ? themeVars.warn : themeVars.border,
                                transition: "border-color 0.3s ease",
                            }}
                        />
                    </div>
                </div>
            </div>
 
            {/* No agent selected */}
            {!selectedAgent && (
                <div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.textDim, padding: "40px 0", textAlign: "center" }}>
                    Select an agent to run diagnostics.
                </div>
            )}
 
            {/* Cards + options + results */}
            {selectedAgent && (
                <>
                    {/* Card grid */}
                    <div
                        style={{
                            fontSize: 10,
                            fontFamily: themeVars.font,
                            color: themeVars.textDim,
                            textTransform: "uppercase",
                            letterSpacing: "0.06em",
                            marginBottom: 10,
                        }}
                    >
                        Available Diagnostics
                    </div>
 
                    <div
                        style={{
                            display: "grid",
                            gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
                            gap: 10,
                            marginBottom: 20,
                        }}
                    >
                        {CARDS.map((card) => {
                            const status = cardStatus[card.id];
                            const isActive = activeTool === card.id && isRunning;
                            const isCardRunning = status === "running";
 
                            return (
                                <button
                                    key={card.id}
                                    onClick={() => handleCardClick(card)}
                                    disabled={isRunning && !isCardRunning}
                                    style={{
                                        padding: "16px 14px",
                                        background: themeVars.surface,
                                        border: `1px solid ${isActive ? themeVars.accent : status === "complete" ? themeVars.ok : status === "error" ? themeVars.danger : themeVars.border}`,
                                        cursor: isRunning && !isCardRunning ? "default" : "pointer",
                                        opacity: isRunning && !isCardRunning ? 0.4 : 1,
                                        textAlign: "left",
                                        fontFamily: themeVars.font,
                                        transition: "all 0.15s ease",
                                        display: "flex",
                                        flexDirection: "column",
                                        gap: 6,
                                    }}
                                >
                                    <div style={{ fontSize: 18, color: themeVars.textDim }}>{card.icon}</div>
                                    <div style={{ fontSize: 13, fontWeight: 600, color: themeVars.text }}>{card.label}</div>
                                    <div style={{ fontSize: 10, color: themeVars.textDim }}>{card.desc}</div>
                                    {status === "running" && (
                                        <div style={{ fontSize: 10, color: themeVars.accent, marginTop: 2 }}>
                                            ⟳ RUNNING
                                        </div>
                                    )}
                                    {status === "complete" && (
                                        <div style={{ fontSize: 10, color: themeVars.ok, marginTop: 2 }}>
                                            ✓ COMPLETE
                                        </div>
                                    )}
                                    {status === "error" && (
                                        <div style={{ fontSize: 10, color: themeVars.danger, marginTop: 2 }}>
                                            ✗ ERROR
                                        </div>
                                    )}
                                </button>
                            );
                        })}
                    </div>
 
                    {/* Disk options */}
                    {showDiskOptions && (
                        <div
                            style={{
                                border: `1px solid ${themeVars.border}`,
                                padding: 14,
                                marginBottom: 16,
                                display: "flex",
                                gap: 10,
                                alignItems: "flex-end",
                                flexWrap: "wrap",
                            }}
                        >
                            <div>
                                <div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginBottom: 4 }}>
                                    Path to scan
                                </div>
                                <input
                                    type="text"
                                    value={diskPath}
                                    onChange={(e) => setDiskPath(e.target.value)}
                                    placeholder="/"
                                    style={{ ...inputStyle, width: 200 }}
                                />
                            </div>
                            <div>
                                <div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginBottom: 4 }}>
                                    Top N
                                </div>
                                <input
                                    type="number"
                                    value={diskTopN}
                                    onChange={(e) => setDiskTopN(Number(e.target.value) || 20)}
                                    min={5}
                                    max={100}
                                    style={{ ...inputStyle, width: 60 }}
                                />
                            </div>
                            <button
                                onClick={() =>
                                    runCommand("disk", () =>
                                        api.triggerDisk(selectedAgent.id, diskPath.trim() || "/", diskTopN)
                                    )
                                }
                                disabled={isRunning}
                                style={{ ...btnStyle, opacity: isRunning ? 0.5 : 1 }}
                            >
                                Run Disk Scan
                            </button>
                            <button
                                onClick={() => setShowDiskOptions(false)}
                                style={{ ...btnStyle, color: themeVars.textMuted, background: "transparent", borderColor: themeVars.border }}
                            >
                                Cancel
                            </button>
                        </div>
                    )}
 
                    {/* Log options */}
                    {showLogOptions && (
                        <div
                            style={{
                                border: `1px solid ${themeVars.border}`,
                                padding: 14,
                                marginBottom: 16,
                                display: "flex",
                                gap: 10,
                                alignItems: "flex-end",
                                flexWrap: "wrap",
                            }}
                        >
                            <div>
                                <div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginBottom: 4 }}>
                                    Minimum Severity
                                </div>
                                <select
                                    value={logLevel}
                                    onChange={(e) => setLogLevel(e.target.value)}
                                    style={{ ...inputStyle, width: 200, cursor: "pointer" }}
                                >
                                    {LOG_LEVELS.map((l) => (
                                        <option key={l} value={l}>{l}</option>
                                    ))}
                                </select>
                            </div>
                            <button
                                onClick={() =>
                                    runCommand("logs", () =>
                                        api.triggerLogs(selectedAgent.id, logLevel)
                                    )
                                }
                                disabled={isRunning}
                                style={{ ...btnStyle, opacity: isRunning ? 0.5 : 1 }}
                            >
                                Fetch Logs
                            </button>
                            <button
                                onClick={() => setShowLogOptions(false)}
                                style={{ ...btnStyle, color: themeVars.textMuted, background: "transparent", borderColor: themeVars.border }}
                            >
                                Cancel
                            </button>
                        </div>
                    )}
 
                    {/* Errors */}
                    {(sendError || pollError) && (
                        <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12, marginBottom: 12 }}>
                            {sendError || pollError}
                        </div>
                    )}
 
                    {/* Running indicator */}
                    {isRunning && activeTool && (
                        <div
                            style={{
                                border: `1px solid ${themeVars.border}`,
                                padding: 16,
                                marginBottom: 16,
                                display: "flex",
                                alignItems: "center",
                                gap: 12,
                            }}
                        >
                            <LoadingSpinner />
                            <span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.textDim, textTransform: "uppercase", letterSpacing: "0.04em" }}>
                                {activeTool} running...
                            </span>
                        </div>
                    )}
 
                    {/* Results */}
                    {displayTool && results[displayTool] && !isRunning && (
                        <div
                            style={{
                                border: `1px solid ${themeVars.border}`,
                                padding: 16,
                            }}
                        >
                            <div
                                style={{
                                    display: "flex",
                                    alignItems: "center",
                                    gap: 8,
                                    marginBottom: 12,
                                }}
                            >
                                <div
                                    style={{
                                        fontSize: 11,
                                        fontFamily: themeVars.font,
                                        fontWeight: 600,
                                        color: themeVars.textDim,
                                        textTransform: "uppercase",
                                        letterSpacing: "0.04em",
                                    }}
                                >
                                    {displayTool} output
                                </div>
                                <span
                                    style={{
                                        fontSize: 10,
                                        fontFamily: themeVars.font,
                                        color: results[displayTool]?.result?.error ? themeVars.danger : themeVars.ok,
                                        textTransform: "uppercase",
                                        letterSpacing: "0.04em",
                                    }}
                                >
                                    {results[displayTool]?.result?.error ? "ERROR" : "COMPLETE"}
                                </span>
                            </div>
                            {renderResult(displayTool)}
                        </div>
                    )}
                </>
            )}
        </div>
    );
}