import { useState, useEffect, useCallback } from "react";
import { api, HttpError } from "../api";
import { themeVars } from "../theme";
import { tableHeaderStyle, tableCellStyle, tableMutedCellStyle, LoadingSpinner } from "../components/ui";
import type { AlertChannel, ChannelType, ChannelConfig, WebhookConfig, EmailConfig } from "../types";

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

const typeBadge = (type: string): React.CSSProperties => {
	const color = type === "webhook" ? themeVars.accent : themeVars.ok;
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
};

// Pulls the display target out of a channel's config for the table, tolerating
// malformed/legacy configs without throwing.
function channelTarget(ch: AlertChannel): string {
	const cfg = ch.config as Partial<WebhookConfig & EmailConfig>;
	if (ch.type === "webhook") return cfg.url ?? "—";
	if (ch.type === "email") return cfg.to ?? "—";
	return "—";
}

function ChannelModal({
	existing,
	onClose,
	onSaved,
}: {
	existing: AlertChannel | null;
	onClose: () => void;
	onSaved: () => void;
}) {
	const [name, setName] = useState(existing?.name ?? "");
	const [type, setType] = useState<ChannelType>(existing?.type ?? "webhook");
	const [url, setUrl] = useState(existing?.type === "webhook" ? (existing.config as WebhookConfig).url ?? "" : "");
	const [to, setTo] = useState(existing?.type === "email" ? (existing.config as EmailConfig).to ?? "" : "");
	const [error, setError] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);

	const handleSave = async () => {
		setError(null);

		const trimmedName = name.trim();
		if (!trimmedName) {
			setError("Name is required.");
			return;
		}

		let config: ChannelConfig;
		if (type === "webhook") {
			const u = url.trim();
			if (!u) {
				setError("Webhook URL is required.");
				return;
			}
			config = { url: u };
		} else if (type === "email") {
			const t = to.trim();
			if (!t) {
				setError("Email recipient is required.");
				return;
			}
			config = { to: t };
		} else {
			// Exhaustiveness guard if ChannelType gains a variant
			const _exhaustive: never = type;
			setError(`Unsupported channel type: ${String(_exhaustive)}`);
			return;
		}

		setSaving(true);
		try {
			if (existing) {
				await api.updateAlertChannel(existing.id, trimmedName, type, config);
			} else {
				await api.createAlertChannel(trimmedName, type, config);
			}
			onSaved();
			onClose();
		} catch (err) {
			setError(err instanceof HttpError ? err.message : "Failed to save channel.");
		} finally {
			setSaving(false);
		}
	};

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
					maxWidth: 480,
					width: "90%",
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
						{existing ? "Edit Channel" : "Create Channel"}
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
					<div>
						<div style={labelStyle}>Name</div>
						<input
							type="text"
							value={name}
							onChange={(e) => setName(e.target.value)}
							placeholder="e.g. ops-webhook"
							style={inputStyle}
						/>
					</div>
 
					<div>
						<div style={labelStyle}>Type</div>
						<select
							value={type}
							onChange={(e) => setType(e.target.value as ChannelType)}
							style={{ ...inputStyle, cursor: "pointer" }}
						>
							<option value="webhook">Webhook</option>
							<option value="email">Email</option>
						</select>
					</div>
 
					{type === "webhook" ? (
						<div>
							<div style={labelStyle}>Webhook URL</div>
							<input
								type="text"
								value={url}
								onChange={(e) => setUrl(e.target.value)}
								placeholder="https://example.com/hook"
								style={inputStyle}
							/>
						</div>
					) : type === "email" ? (
						<div>
							<div style={labelStyle}>Recipient</div>
							<input
								type="text"
								value={to}
								onChange={(e) => setTo(e.target.value)}
								placeholder="alerts@example.com"
								style={inputStyle}
							/>
							<div
								style={{
									fontSize: 10,
									fontFamily: themeVars.font,
									color: themeVars.textDim,
									marginTop: 4,
								}}
							>
								Delivered via the server-wide SMTP transport (configured by an admin).
							</div>
						</div>
					) : null}
				</div>
 
				<div
					style={{
						display: "flex",
						gap: 8,
						marginTop: 20,
						justifyContent: "flex-end",
					}}
				>
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
						disabled={saving}
						style={{ ...btnStyle, opacity: saving ? 0.6 : 1 }}
					>
						{saving ? "Saving..." : existing ? "Save Changes" : "Create Channel"}
					</button>
				</div>
			</div>
		</div>
	);	
}

export function AlertChannels() {
	const [channels, setChannels] = useState<AlertChannel[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [showModal, setShowModal] = useState(false);
	const [editing, setEditing] = useState<AlertChannel | null>(null);
	const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

	const loadChannels = useCallback(() => {
		api.listAlertChannels()
			.then(setChannels)
			.catch((err) =>
				setError(err instanceof Error ? err.message : "Failed to load channels.")
			)
			.finally(() => setLoading(false));
	}, []);

	useEffect(() => {
		setLoading(true);
		loadChannels();
	}, [loadChannels]);

	const handleDelete = async (id: string) => {
		try {
			await api.deleteAlertChannel(id);
			setConfirmDelete(null);
			loadChannels();
		} catch (err) {
			setError(err instanceof HttpError ? err.message : "Failed to delete channel.");
			setTimeout(() => setError(null), 3000);
		}
	};

	const openCreate = () => {
		setEditing(null);
		setShowModal(true);
	};

	const openEdit = (ch: AlertChannel) => {
		setEditing(ch);
		setShowModal(true);
	};

	if (loading) return <LoadingSpinner />;

	return (
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
				Alert Channels
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
 
			<div style={{ display: "flex", gap: 12, marginBottom: 16, alignItems: "center" }}>
				<button onClick={openCreate} style={btnStyle}>
					+ Create Channel
				</button>
				<span
					style={{
						fontSize: 11,
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						marginLeft: "auto",
					}}
				>
					{channels.length} channel{channels.length === 1 ? "" : "s"}
				</span>
			</div>
 
			{channels.length === 0 ? (
				<div
					style={{
						textAlign: "center",
						padding: "40px 0",
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						fontSize: 13,
					}}
				>
					No channels configured. Create one to start receiving alert notifications.
				</div>
			) : (
				<div style={{ overflowX: "auto" }}>
					<table
						style={{
							width: "100%",
							borderCollapse: "collapse",
							textAlign: "left",
						}}
					>
						<thead>
							<tr>
								<th style={tableHeaderStyle}>Name</th>
								<th style={tableHeaderStyle}>Type</th>
								<th style={tableHeaderStyle}>Target</th>
								<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Actions</th>
							</tr>
						</thead>
						<tbody>
							{channels.map((ch, i) => (
								<tr
									key={ch.id}
									style={{
										background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
									}}
								>
									<td style={{ ...tableCellStyle, fontWeight: 500 }}>{ch.name}</td>
									<td style={tableCellStyle}>
										<span style={typeBadge(ch.type)}>{ch.type}</span>
									</td>
									<td
										style={{
											...tableMutedCellStyle,
											fontFamily: "monospace",
											fontSize: 11,
											maxWidth: 360,
											overflow: "hidden",
											textOverflow: "ellipsis",
											whiteSpace: "nowrap",
										}}
										title={channelTarget(ch)}
									>
										{channelTarget(ch)}
									</td>
									<td style={{ ...tableCellStyle, textAlign: "right" }}>
										{confirmDelete === ch.id ? (
											<div
												style={{
													display: "flex",
													gap: 6,
													justifyContent: "flex-end",
													alignItems: "center",
												}}
											>
												<span
													style={{
														fontSize: 11,
														fontFamily: themeVars.font,
														color: themeVars.danger,
													}}
												>
													Delete {ch.name}?
												</span>
												<button
													onClick={() => handleDelete(ch.id)}
													style={{
														...btnStyle,
														color: "#fff",
														background: themeVars.danger,
														borderColor: themeVars.danger,
														padding: "3px 10px",
													}}
												>
													Confirm
												</button>
												<button
													onClick={() => setConfirmDelete(null)}
													style={{
														...btnStyle,
														color: themeVars.textMuted,
														background: "transparent",
														borderColor: themeVars.border,
														padding: "3px 10px",
													}}
												>
													Cancel
												</button>
											</div>
										) : (
											<div
												style={{
													display: "flex",
													gap: 6,
													justifyContent: "flex-end",
												}}
											>
												<button
													onClick={() => openEdit(ch)}
													style={{
														...btnStyle,
														color: themeVars.textMuted,
														background: "transparent",
														borderColor: themeVars.border,
														padding: "3px 10px",
													}}
												>
													Edit
												</button>
												<button
													onClick={() => setConfirmDelete(ch.id)}
													style={{
														...btnStyle,
														color: themeVars.danger,
														background: "transparent",
														borderColor: themeVars.danger,
														padding: "3px 10px",
													}}
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
 
			{showModal && (
				<ChannelModal
					existing={editing}
					onClose={() => setShowModal(false)}
					onSaved={loadChannels}
				/>
			)}
		</div>
	);
}