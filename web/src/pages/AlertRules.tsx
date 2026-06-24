import { useState, useEffect, useCallback } from "react";
import { api, HttpError } from "../api";
import { themeVars } from "../theme";
import { tableHeaderStyle, tableCellStyle, tableMutedCellStyle, LoadingSpinner } from "../components/ui";
import { usePagination, Pagination } from "../hooks/usePagination";
import type {
	User,
	Agent,
	AlertRule,
	AlertChannel,
	AlertScope,
	ConditionType,
	ConditionParams,
	DiskMetric,
	Service,
} from "../types";

const CONDITION_LABELS: Record<ConditionType, string> = {
	agent_offline: "Agent Offline",
	disk_prediction: "Disk Prediction",
	service_down: "Service Down",
};

const DEFAULT_TIMEOUT_SECONDS = 300;
const DEFAULT_WARN_HOURS = 72;

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
 
const labelStyle: React.CSSProperties = {
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.textDim,
	textTransform: "uppercase",
	letterSpacing: "0.04em",
	marginBottom: 4,
};
 
const inputStyle: React.CSSProperties = {
	width: "100%",
	padding: "6px 10px",
	fontSize: 12,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.surface,
	border: `1px solid ${themeVars.border}`,
	boxSizing: "border-box",
};
 
const disabledInputStyle: React.CSSProperties = {
	...inputStyle,
	color: themeVars.textDim,
	background: themeVars.bg,
	cursor: "not-allowed",
};

/**
 * Reads a typed param field out of a rule's condition_params for edit pre-fill,
 * tolerating shape mismatches. */
function readParams(rule: AlertRule) {
	const p = (rule.condition_params ?? {}) as unknown as Record<string, unknown>;
	return {
		timeoutSeconds: typeof p.timeout_seconds === "number" ? p.timeout_seconds : DEFAULT_TIMEOUT_SECONDS,
		mount: typeof p.mount === "string" ? p.mount : "",
		warnHours: typeof p.warn_hours === "number" ? p.warn_hours : DEFAULT_WARN_HOURS,
		serviceName: typeof p.service_name === "string" ? p.service_name : "",
	};
}

function scopeBadge(scope: string): React.CSSProperties {
	const color = scope === "global" ? themeVars.accent : themeVars.warn;
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
	};
}

function RuleModal({
	existing,
	agents,
	channels,
	onClose,
	onSaved,
}: {
	existing: AlertRule | null;
	agents: Agent[];
	channels: AlertChannel[];
	onClose: () => void;
	onSaved: () => void;
}) {
	const isEdit = existing !== null;
	const seed = existing ? readParams(existing) : null;

	const [name, setName] = useState(existing?.name ?? "");
	const [enabled, setEnabled] = useState(existing?.enabled ?? true);
	const [scope, setScope] = useState<AlertScope>(existing?.scope ?? "global");
	const [agentId, setAgentId] = useState(existing?.agent_id ?? "");
	const [conditionType, setConditionType] = useState<ConditionType>(existing?.condition_type ?? "agent_offline");
	const [cooldown, setCooldown] = useState(existing?.cooldown_seconds ?? 0);

	// Per-field condition param state (seeded from defaults or existing rule)
	const [timeoutSeconds, setTimeoutSeconds] = useState(seed?.timeoutSeconds ?? DEFAULT_TIMEOUT_SECONDS);
	const [mount, setMount] = useState(seed?.mount ?? "");
	const [warnHours, setWarnHours] = useState(seed?.warnHours ?? DEFAULT_WARN_HOURS);
	const [serviceName, setServiceName] = useState(seed?.serviceName ?? "");

	const [mountOptions, setMountOptions] = useState<string[]>([]);
	const [serviceOptions, setServiceOptions] = useState<string[]>([]);
	const [selectedChannels, setSelectedChannels] = useState<Set<string>>(new Set());
	const [channelsLoaded, setChannelsLoaded] = useState(!isEdit);
	const [error, setError] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);

	// Load an existing rule's currently attached channels
	useEffect(() => {
		if (!existing) return;
		api.getAlertRule(existing.id)
			.then((res) => {
				setSelectedChannels(new Set(res.channels.map((c) => c.id)));
			})
			.catch(() => {})
			.finally(() => setChannelsLoaded(true));
	}, [existing]);

	// Offer agent's tracked mounts for disk_prediction
	useEffect(() => {
		if (scope !== "agent" || conditionType !== "disk_prediction" || !agentId) {
			setMountOptions([]);
			return;
		}
		let cancelled = false;
		api.agentDisk(agentId, { type: "quick", range: "1h" })
			.then((disks: DiskMetric[]) => {
				if (cancelled) return;
				const set = new Set<string>();
				for (const d of disks) if (d.mountpoint) set.add(d.mountpoint);
				if (mount) set.add(mount);
				setMountOptions(Array.from(set).sort());
			})
			.catch(() => {
				if (!cancelled) setMountOptions(mount ? [mount] : []);
			});
		return () => {
			cancelled = true;
		};
	}, [scope, conditionType, agentId, mount]);

	// Offer the agent's reported services for service_down. Folds in the existing
	// rule's service name so editing never loses a value the agent isn't currently
	// reporting (e.g. the service is down).
	useEffect(() => {
		if (conditionType !== "service_down" || !agentId) {
			setServiceOptions([]);
			return;
		}
		let cancelled = false;
		api.agentServices(agentId)
			.then((services: Service[]) => {
				if (cancelled) return;
				const set = new Set<string>();
				for (const s of services) if (s.name) set.add(s.name);
				if (serviceName) set.add(serviceName);
				setServiceOptions(Array.from(set).sort());
			})
			.catch(() => {
				if (!cancelled) setServiceOptions(serviceName ? [serviceName] : []);
			});
		return () => {
			cancelled = true;
		};
	}, [conditionType, agentId, serviceName]);

	// Force service_down to agent scope
	const handleConditionChange = (next: ConditionType) => {
		setConditionType(next);
		if (next === "service_down" && scope === "global") setScope("agent");
	};

	const toggleChannel = (id: string) => {
		setSelectedChannels((prev) => {
			const n = new Set(prev);
			if (n.has(id)) n.delete(id);
			else n.add(id);
			return n;
		});
	};

	const buildParams = (): ConditionParams | { error: string } => {
		if (conditionType === "agent_offline") {
			if (timeoutSeconds <= 0) return { error: "Timeout must be a positive number of seconds." };
			return { timeout_seconds: timeoutSeconds };
		} else if (conditionType === "disk_prediction") {
			const m = mount.trim();
			if (!m) return { error: "Mount path is required for disk prediction." };
			if (warnHours <= 0) return { error: "Warn hours must be a positive number." };
			return { mount: m, warn_hours: warnHours };
		} else if (conditionType === "service_down") {
			const s = serviceName.trim();
			if (!s) return { error: "Service name is required for service down." };
			return { service_name: s };
		} else {
			// Exhaustiveness guard to ensure new ConditionTypes are handled
			const _exhaustive: never = conditionType;
			return { error: `Unsupported condition type: ${String(_exhaustive)}` };
		}
	};

	const handleSave = async () => {
		setError(null);

		// Guard against saving before an edited rule's channels have loaded
		// to prevent sending an empty channel set and wiping the associations
		if (!channelsLoaded) {
			setError("Still loading the rule's channels - please wait.");
			return;
		}

		const trimmedName = name.trim();
		if (!trimmedName) {
			setError("Name is required.");
			return;
		}
		if (scope === "agent" && !agentId) {
			setError("An agent must be selected for agent-scoped rules.");
			return;
		}
		if (conditionType === "service_down" && scope === "global") {
			setError("Service Down rules must target a specific agent.");
			return;
		}
		if (cooldown < 0) {
			setError("Cooldown must not be negative");
			return;
		}

		const params = buildParams();
		if ("error" in params) {
			setError(params.error);
			return;
		}

		const channelIds = Array.from(selectedChannels);

		setSaving(true);
		try {
			if (isEdit) {
				// only send the mutable fields (not scope, agent, or condition_type)
				await api.updateAlertRule(existing.id, {
					name: trimmedName,
					enabled,
					condition_params: params,
					cooldown_seconds: cooldown,
					channel_ids: channelIds,
				});
			} else {
				await api.createAlertRule({
					name: trimmedName,
					enabled,
					scope,
					agent_id: scope === "agent" ? agentId: undefined,
					condition_type: conditionType,
					condition_params: params,
					cooldown_seconds: cooldown,
					channel_ids: channelIds,
				});
			}
			onSaved();
			onClose();
		} catch (err) {
			setError(err instanceof HttpError ? err.message : "Failed to save rule.");
		} finally {
			setSaving(false);
		}
	};

	const globalDisabled = conditionType === "service_down";

	return (
		<div
			style={{
				position: "fixed",
				top: 0,
				left: 0,
				right: 0,
				bottom: 0,
				background: "rgba(0, 0, 0, 0.6)",
				display: "flex",
				alignItems: "center",
				justifyContent: "center",
				zIndex: 100,
			}}
			onClick={(e) => {
				if (e.target === e.currentTarget) onClose();
			}}
		>
			<div
				style={{
					background: themeVars.bg,
					border: `1px solid ${themeVars.border}`,
					padding: 24,
					maxWidth: 520,
					width: "90%",
					maxHeight: "85vh",
					overflowY: "auto",
				}}
			>
				<div
					style={{
						display: "flex",
						justifyContent: "space-between",
						alignItems: "center",
						marginBottom: 16,
					}}
				>
					<div
						style={{
							fontFamily: themeVars.font,
							fontSize: 16,
							fontWeight: 600,
							color: themeVars.text,
						}}
					>
						{isEdit ? "Edit Rule" : "Create Rule"}
					</div>
					<button
						onClick={onClose}
						style={{
							background: "none",
							border: "none",
							color: themeVars.textMuted,
							fontSize: 18,
							cursor: "pointer",
							fontFamily: themeVars.font,
						}}
					>
						×
					</button>
				</div>
 
				{error && (
					<div
						style={{
							fontSize: 12,
							fontFamily: themeVars.font,
							color: themeVars.danger,
							marginBottom: 12,
						}}
					>
						{error}
					</div>
				)}
 
				<div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
					{/* Name */}
					<div>
						<div style={labelStyle}>Name</div>
						<input
							type="text"
							value={name}
							onChange={(e) => setName(e.target.value)}
							placeholder="Production web server offline"
							style={inputStyle}
						/>
					</div>
 
					{/* Condition type — immutable on edit */}
					<div>
						<div style={labelStyle}>
							Condition {isEdit && <span style={{ textTransform: "none" }}>(fixed)</span>}
						</div>
						<select
							value={conditionType}
							onChange={(e) => handleConditionChange(e.target.value as ConditionType)}
							disabled={isEdit}
							style={isEdit ? disabledInputStyle : { ...inputStyle, cursor: "pointer" }}
						>
							{(Object.keys(CONDITION_LABELS) as ConditionType[]).map((c) => (
								<option key={c} value={c}>{CONDITION_LABELS[c]}</option>
							))}
						</select>
					</div>
 
					{/* Scope — immutable on edit */}
					<div>
						<div style={labelStyle}>
							Scope {isEdit && <span style={{ textTransform: "none" }}>(fixed)</span>}
						</div>
						<select
							value={scope}
							onChange={(e) => setScope(e.target.value as AlertScope)}
							disabled={isEdit}
							style={isEdit ? disabledInputStyle : { ...inputStyle, cursor: "pointer" }}
						>
							<option value="global" disabled={globalDisabled}>
								{globalDisabled
									? "Global — unavailable for Service Down"
									: "Global (all agents)"}
							</option>
							<option value="agent">Single agent</option>
						</select>
					</div>
 
					{/* Agent selector — only for agent scope, immutable on edit */}
					{scope === "agent" && (
						<div>
							<div style={labelStyle}>
								Agent {isEdit && <span style={{ textTransform: "none" }}>(fixed)</span>}
							</div>
							<select
								value={agentId}
								onChange={(e) => setAgentId(e.target.value)}
								disabled={isEdit}
								style={isEdit ? disabledInputStyle : { ...inputStyle, cursor: "pointer" }}
							>
								<option value="">Select an agent…</option>
								{agents.map((a) => (
									<option key={a.id} value={a.id}>{a.hostname}</option>
								))}
							</select>
						</div>
					)}
 
					{/* Condition params — swap per condition type */}
					{conditionType === "agent_offline" ? (
						<div>
							<div style={labelStyle}>Timeout (seconds)</div>
							<input
								type="number"
								min={1}
								value={timeoutSeconds}
								onChange={(e) => setTimeoutSeconds(Number(e.target.value))}
								style={inputStyle}
							/>
							<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
								Fires when the agent hasn't reported within this window.
							</div>
						</div>
					) : conditionType === "disk_prediction" ? (
						<>
							<div>
								<div style={labelStyle}>Mount</div>
								<input
									type="text"
									value={mount}
									onChange={(e) => setMount(e.target.value)}
									placeholder="/"
									list="rule-mount-options"
									style={inputStyle}
									autoComplete="off"
								/>
								{mountOptions.length > 0 && (
									<datalist id="rule-mount-options">
										{mountOptions.map((m) => (
											<option key={m} value={m} />
										))}
									</datalist>
								)}
								{scope === "global" && (
									<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
										Matches this mount path on every agent.
									</div>
								)}
							</div>
							<div>
								<div style={labelStyle}>Warn Hours</div>
								<input
									type="number"
									min={1}
									value={warnHours}
									onChange={(e) => setWarnHours(Number(e.target.value))}
									style={inputStyle}
								/>
								<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
									Fires when the mount is projected to fill within this many hours.
								</div>
							</div>
						</>
					) : conditionType === "service_down" ? (
						<div>
							<div style={labelStyle}>Service Name</div>
							<input
								type="text"
								value={serviceName}
								onChange={(e) => setServiceName(e.target.value)}
								placeholder="nginx"
								list="rule-service-options"
								style={inputStyle}
								autoComplete="off"
							/>
							{serviceOptions.length > 0 && (
								<datalist id="rule-service-options">
									{serviceOptions.map((s) => (
										<option key={s} value={s} />
									))}
								</datalist>
							)}
							<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
								Start typing to filter the agent's services, or enter a service not currently running.
							</div>
						</div>
					) : null}
 
					{/* Cooldown */}
					<div>
						<div style={labelStyle}>Cooldown (seconds)</div>
						<input
							type="number"
							min={0}
							value={cooldown}
							onChange={(e) => setCooldown(Number(e.target.value))}
							style={inputStyle}
						/>
						<div style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.textDim, marginTop: 4 }}>
							Suppresses re-firing for this long after the alert resolves.
						</div>
					</div>
 
					{/* Channels */}
					<div>
						<div style={labelStyle}>Notify Channels</div>
						{!channelsLoaded ? (
							<div style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.textDim }}>
								Loading…
							</div>
						) : channels.length === 0 ? (
							<div style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.textDim }}>
								No channels configured. Create one in the Channels tab.
							</div>
						) : (
							<div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
								{channels.map((ch) => (
									<label
										key={ch.id}
										style={{
											display: "flex",
											alignItems: "center",
											gap: 8,
											fontSize: 12,
											fontFamily: themeVars.font,
											color: themeVars.text,
											cursor: "pointer",
										}}
									>
										<input
											type="checkbox"
											checked={selectedChannels.has(ch.id)}
											onChange={() => toggleChannel(ch.id)}
										/>
										<span>{ch.name}</span>
										<span style={{ color: themeVars.textDim, fontSize: 10 }}>({ch.type})</span>
									</label>
								))}
							</div>
						)}
					</div>
 
					{/* Enabled */}
					<label
						style={{
							display: "flex",
							alignItems: "center",
							gap: 8,
							fontSize: 12,
							fontFamily: themeVars.font,
							color: themeVars.text,
							cursor: "pointer",
						}}
					>
						<input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
						<span>Enabled</span>
					</label>
				</div>
 
				<div style={{ display: "flex", gap: 8, marginTop: 20, justifyContent: "flex-end" }}>
					<button
						onClick={onClose}
						style={{
							...btnStyle,
							color: themeVars.textMuted,
							background: "transparent",
							borderColor: themeVars.border,
						}}
					>
						Cancel
					</button>
					<button
						onClick={handleSave}
						disabled={saving || !channelsLoaded}
						style={{ ...btnStyle, opacity: saving || !channelsLoaded ? 0.6 : 1 }}
					>
						{saving ? "Saving..." : isEdit ? "Save Changes" : "Create Rule"}
					</button>
				</div>
			</div>
		</div>
	);
}

interface AlertRulesProps {
	user: User;
}

export function AlertRules({ user: _user }: AlertRulesProps) {
	const [rules, setRules] = useState<AlertRule[]>([]);
	const [agents, setAgents] = useState<Agent[]>([]);
	const [channels, setChannels] = useState<AlertChannel[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [showModal, setShowModal] = useState(false);
	const [editing, setEditing] = useState<AlertRule | null>(null);
	const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

	const loadRules = useCallback(() => {
		api.listAlertRules()
			.then(setRules)
			.catch((err) => setError(err instanceof Error ? err.message : "Failed to load rules."))
			.finally(() => setLoading(false));
	}, []);
	
	const loadModalData = useCallback(() => {
		api.agents().then(setAgents).catch(() => {});
		api.listAlertChannels().then(setChannels).catch(() => {});
	}, []);

	useEffect(() => {
		setLoading(true);
		loadRules();
		loadModalData();
	}, [loadRules, loadModalData]);

	const agentHostname = useCallback((id: string | null) => {
		if (!id) return "—";
		return agents.find((a) => a.id === id)?.hostname ?? id;
	}, [agents]);

	const handleToggleEnabled = async (rule: AlertRule) => {
		try {
			await api.setAlertRuleEnabled(rule.id, !rule.enabled);
			loadRules();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to toggle rule.");
			setTimeout(() => setError(null), 3000);
		}
	};

	const handleDelete = async (id: string) => {
		try {
			await api.deleteAlertRule(id);
			setConfirmDelete(null);
			loadRules();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to delete rule.");
			setTimeout(() => setError(null), 3000);
		}
	};

	const openCreate = () => {
		loadModalData();
		setEditing(null);
		setShowModal(true);
	};

	const openEdit = (rule: AlertRule) => {
		loadModalData();
		setEditing(rule);
		setShowModal(true);
	};

	const { paged, page, setPage, totalPages, total } = usePagination(rules, 20);

	if (loading) return <LoadingSpinner />;

	return (
		<div>
			{error && (
				<div
					style={{
						fontSize: 12,
						fontFamily: themeVars.font,
						color: themeVars.danger,
						marginBottom: 12,
					}}
				>
					{error}
				</div>
			)}
 
			<div style={{ display: "flex", gap: 12, marginBottom: 16, alignItems: "center" }}>
				<button onClick={openCreate} style={btnStyle}>
					+ Create Rule
				</button>
				<span
					style={{
						fontSize: 11,
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						marginLeft: "auto",
					}}
				>
					{rules.length} rule{rules.length === 1 ? "" : "s"}
				</span>
			</div>
 
			{rules.length === 0 ? (
				<div
					style={{
						textAlign: "center",
						padding: "40px 0",
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						fontSize: 13,
					}}
				>
					No rules configured. Create one to start evaluating alert conditions.
				</div>
			) : (
				<div style={{ overflowX: "auto" }}>
					<table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left" }}>
						<thead>
							<tr>
								<th style={tableHeaderStyle}>Name</th>
								<th style={tableHeaderStyle}>Condition</th>
								<th style={tableHeaderStyle}>Scope</th>
								<th style={tableHeaderStyle}>Target</th>
								<th style={tableHeaderStyle}>Enabled</th>
								<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Actions</th>
							</tr>
						</thead>
						<tbody>
							{paged.map((rule, i) => (
								<tr
									key={rule.id}
									style={{ background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover }}
								>
									<td style={{ ...tableCellStyle, fontWeight: 500 }}>{rule.name}</td>
									<td style={tableMutedCellStyle}>{CONDITION_LABELS[rule.condition_type]}</td>
									<td style={tableCellStyle}>
										<span style={scopeBadge(rule.scope)}>{rule.scope}</span>
									</td>
									<td style={tableMutedCellStyle}>
										{rule.scope === "agent" ? agentHostname(rule.agent_id) : "All agents"}
									</td>
									<td style={tableCellStyle}>
										<button
											onClick={() => handleToggleEnabled(rule)}
											style={{
												fontSize: 9,
												fontFamily: themeVars.font,
												fontWeight: 600,
												color: rule.enabled ? themeVars.ok : themeVars.textDim,
												background: `color-mix(in srgb, ${rule.enabled ? themeVars.ok : themeVars.textDim} 15%, transparent)`,
												border: `1px solid ${rule.enabled ? themeVars.ok : themeVars.textDim}`,
												padding: "2px 8px",
												letterSpacing: "0.04em",
												textTransform: "uppercase",
												cursor: "pointer",
											}}
											title="Click to toggle"
										>
											{rule.enabled ? "Enabled" : "Disabled"}
										</button>
									</td>
									<td style={{ ...tableCellStyle, textAlign: "right" }}>
										{confirmDelete === rule.id ? (
											<div style={{ display: "flex", gap: 6, justifyContent: "flex-end", alignItems: "center" }}>
												<span style={{ fontSize: 11, fontFamily: themeVars.font, color: themeVars.danger }}>
													Delete {rule.name}?
												</span>
												<button
													onClick={() => handleDelete(rule.id)}
													style={{ ...btnStyle, color: "#fff", background: themeVars.danger, borderColor: themeVars.danger, padding: "3px 10px" }}
												>
													Confirm
												</button>
												<button
													onClick={() => setConfirmDelete(null)}
													style={{ ...btnStyle, color: themeVars.textMuted, background: "transparent", borderColor: themeVars.border, padding: "3px 10px" }}
												>
													Cancel
												</button>
											</div>
										) : (
											<div style={{ display: "flex", gap: 6, justifyContent: "flex-end" }}>
												<button
													onClick={() => openEdit(rule)}
													style={{ ...btnStyle, color: themeVars.textMuted, background: "transparent", borderColor: themeVars.border, padding: "3px 10px" }}
												>
													Edit
												</button>
												<button
													onClick={() => setConfirmDelete(rule.id)}
													style={{ ...btnStyle, color: themeVars.danger, background: "transparent", borderColor: themeVars.danger, padding: "3px 10px" }}
												>
													Delete
												</button>
											</div>
										)}
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
 
			{rules.length > 0 && (
				<Pagination
					page={page}
					totalPages={totalPages}
					total={total}
					pageSize={20}
					onPageChange={setPage}
				/>
			)}
 
			{showModal && (
				<RuleModal
					existing={editing}
					agents={agents}
					channels={channels}
					onClose={() => setShowModal(false)}
					onSaved={loadRules}
				/>
			)}
		</div>
	);
}