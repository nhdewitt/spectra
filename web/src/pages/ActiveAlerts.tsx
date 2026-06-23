import { useState, useEffect, useCallback } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import { usePolling } from "../hooks";
import { usePagination, Pagination } from "../hooks/usePagination";
import { timeAgo } from "../utils";
import { tableHeaderStyle, tableCellStyle, tableMutedCellStyle, LoadingSpinner } from "../components/ui";
import type { AlertEvent, ConditionType, AgentOfflineSnapshot, DiskPredictionSnapshot, ServiceDownSnapshot } from "../types";

const HISTORY_LIMIT = 200;

const CONDITION_LABELS: Record<ConditionType, string> = {
	agent_offline: "Agent Offline",
	disk_prediction: "Disk Prediction",
	service_down: "Service Down",
};

/** Absolute timestamp for hover tooltips. */
function absTime(ts: string | null): string {
	if (!ts) return "—";
	return new Date(ts).toLocaleString(undefined, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	})	;
}

/** Compact duration from seconds (e.g. 300 -> "5m"). */
function humanDuration(seconds: number): string {
	if (seconds < 60) return `${Math.floor(seconds)}s`;
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	return m > 0 ? `${h}h ${m}m` : `${h}h`;
}

/**
 * Renders a summary from an event's condition_snapshot, narrowing
 * the snapshot by the event's condition type. Falls back to a dash
 * if the snapshot is missing or malformed.
 */
function formatSnapshot(event: AlertEvent): string {
	const snap = event.condition_snapshot;
	if (snap == null || typeof snap !== "object") return "—";

	try {
		switch (event.condition_type) {
			case "agent_offline": {
				const s = snap as unknown as AgentOfflineSnapshot;
				const silent = typeof s.seconds_silent === "number" ? humanDuration(s.seconds_silent) : "?";
				return `Silent ${silent} (last seen ${absTime(s.last_seen ?? null)})`;
			}
			case "disk_prediction": {
				const s = snap as unknown as DiskPredictionSnapshot;
				const pct = typeof s.used_pct === "number" ? `${s.used_pct.toFixed(1)}%` : "?";
				const hrs = typeof s.hours_remaining === "number"
					? `full in ~${humanDuration(s.hours_remaining * 3600)}`
					: "projection unavailable";
				return `${s.mount ?? "?"} at ${pct} — ${hrs}`;
			}
			case "service_down": {
				const s = snap as unknown as ServiceDownSnapshot;
				const status = s.last_status ? ` (${s.last_status})` : "";
				return `${s.service_name ?? "service"} not healthy${status}`;
			}
			default: {
				const _exhaustive: never = event.condition_type;
				return String(_exhaustive);
			}
		}
	} catch {
		return "—";
	}
}

function conditionBadge(): React.CSSProperties {
	return {
		fontSize: 9,
		fontFamily: themeVars.font,
		fontWeight: 600,
		color: themeVars.textMuted,
		background: `color-mix(in srgb, ${themeVars.textMuted} 12%, transparent)`,
		border: `1px solid ${themeVars.border}`,
		padding: "1px 6px",
		letterSpacing: "0.04em",
		textTransform: "uppercase",
		whiteSpace: "nowrap",
	};
}

function statusBadge(resolved: boolean): React.CSSProperties {
	const color = resolved ? themeVars.textDim : themeVars.danger;
	return {
		fontSize: 9,
		fontFamily: themeVars.font,
		fontWeight: 600,
		color,
		background: `color-mix(in srgb, ${color} 15%, transparent)`,
		border: `1px solid ${color}`,
		padding: "1px 6px",
		letterSpacing: "0.04em",
		textTransform: "uppercase",
		whiteSpace: "nowrap",
	};
}

function ActiveSection() {
	const fetcher = useCallback(() => api.activeAlerts(), []);
	const { data, loading, error } = usePolling(fetcher, 10_000);
	const events = data ?? [];

	if (loading && events.length === 0) return <LoadingSpinner />;

	if (error) {
		return (
			<div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.danger, marginBottom: 12 }}>
				{error}
			</div>
		);
	}

	if (events.length === 0) {
		return (
			<div
				style={{
					padding: "20px 0",
					fontFamily: themeVars.font,
					color: themeVars.textDim,
					fontSize: 13,
				}}
			>
				No active alerts. All monitored conditions are healthy.
			</div>
		);
	}

	return (
		<div style={{ overflowX: "auto" }}>
			<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
				<thead>
					<tr>
						<th style={tableHeaderStyle}>Rule</th>
						<th style={tableHeaderStyle}>Agent</th>
						<th style={tableHeaderStyle}>Condition</th>
						<th style={tableHeaderStyle}>Detail</th>
						<th style={tableHeaderStyle}>Fired</th>
					</tr>
				</thead>
				<tbody>
					{events.map((e, i) => (
						<tr key={e.id} style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}>
							<td style={{ ...tableCellStyle, fontWeight: 500 }}>{e.rule_name}</td>
							<td style={tableMutedCellStyle}>{e.hostname ?? "—"}</td>
							<td style={tableCellStyle}>
								<span style={conditionBadge()}>
									{CONDITION_LABELS[e.condition_type] ?? e.condition_type}
								</span>
							</td>
							<td style={tableMutedCellStyle}>{formatSnapshot(e)}</td>
							<td style={tableMutedCellStyle} title={absTime(e.fired_at)}>
								{timeAgo(e.fired_at)}
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}

function HistorySection() {
	const [events, setEvents] = useState<AlertEvent[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		setLoading(true);
		api.alertHistory(HISTORY_LIMIT, 0)
			.then((data) => setEvents(data ?? []))
			.catch((err) => setError(err instanceof Error ? err.message : "Failed to load history."))
			.finally(() => setLoading(false));
	}, []);

	const { paged, page, setPage, totalPages, total } = usePagination(events, 20);

	if (loading) return <LoadingSpinner />;

	if (error) {
		return (
			<div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.danger }}>
				{error}
			</div>
		);
	}
 
	if (events.length === 0) {
		return (
			<div
				style={{
					padding: "20px 0",
					fontFamily: themeVars.font,
					color: themeVars.textDim,
					fontSize: 13,
				}}
			>
				No alert history yet.
			</div>
		);
	}
 
	return (
		<>
			<div style={{ overflowX: "auto" }}>
				<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
					<thead>
						<tr>
							<th style={tableHeaderStyle}>Status</th>
							<th style={tableHeaderStyle}>Rule</th>
							<th style={tableHeaderStyle}>Agent</th>
							<th style={tableHeaderStyle}>Condition</th>
							<th style={tableHeaderStyle}>Detail</th>
							<th style={tableHeaderStyle}>Fired</th>
							<th style={tableHeaderStyle}>Resolved</th>
						</tr>
					</thead>
					<tbody>
						{paged.map((e, i) => {
							const resolved = e.resolved_at != null;
							return (
								<tr key={e.id} style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}>
									<td style={tableCellStyle}>
										<span style={statusBadge(resolved)}>{resolved ? "Resolved" : "Firing"}</span>
									</td>
									<td style={{ ...tableCellStyle, fontWeight: 500 }}>{e.rule_name}</td>
									<td style={tableMutedCellStyle}>{e.hostname ?? "—"}</td>
									<td style={tableCellStyle}>
										<span style={conditionBadge()}>
											{CONDITION_LABELS[e.condition_type] ?? e.condition_type}
										</span>
									</td>
									<td style={tableMutedCellStyle}>{formatSnapshot(e)}</td>
									<td style={tableMutedCellStyle} title={absTime(e.fired_at)}>
										{timeAgo(e.fired_at)}
									</td>
									<td style={tableMutedCellStyle} title={absTime(e.resolved_at)}>
										{resolved ? timeAgo(e.resolved_at) : "—"}
									</td>
								</tr>
							);
						})}
					</tbody>
				</table>
			</div>
			<Pagination
				page={page}
				totalPages={totalPages}
				total={total}
				pageSize={20}
				onPageChange={setPage}
			/>
		</>
	);
}

const sectionHeaderStyle: React.CSSProperties = {
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.textDim,
	textTransform: "uppercase",
	letterSpacing: "0.05em",
	marginBottom: 8,
	marginTop: 4,
};

export function ActiveAlerts() {
	return (
		<div>
			<div style={sectionHeaderStyle}>Active</div>
			<ActiveSection />

			<div style={{ ...sectionHeaderStyle, marginTop: 28 }}>History</div>
			<HistorySection />
		</div>
	);
}