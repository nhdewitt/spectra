import { useState, useEffect, useCallback } from "react";
import { api, HttpError } from "../api";
import { themeVars } from "../theme";
import type { SMTPConfig, SMTPConfigUpdate, TLSMode } from "../types";

const cardStyle: React.CSSProperties = {
	background: themeVars.surface,
	border: `1px solid ${themeVars.border}`,
	padding: 20,
	marginTop: 20,
};
 
const sectionHeaderStyle: React.CSSProperties = {
	fontSize: 10,
	fontFamily: themeVars.font,
	color: themeVars.textDim,
	letterSpacing: "0.05em",
	textTransform: "uppercase",
	marginBottom: 12,
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
	background: themeVars.bg,
	border: `1px solid ${themeVars.border}`,
	boxSizing: "border-box",
};
 
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
 
const smallBtn: React.CSSProperties = {
	padding: "3px 10px",
	fontSize: 10,
	fontFamily: themeVars.font,
	color: themeVars.textMuted,
	background: "transparent",
	border: `1px solid ${themeVars.border}`,
	cursor: "pointer",
	textTransform: "uppercase",
	letterSpacing: "0.03em",
};

const STARTTLS_PORT = 587;
const IMPLICIT_TLS_PORT = 465;
const PLAINTEXT_SMTP_PORT = 25;

const TLS_MODES: { value: TLSMode; label: string; port: number }[] = [
	{ value: "starttls", label: "STARTTLS", port: STARTTLS_PORT },
	{ value: "implicit", label: "Implicit TLS", port: IMPLICIT_TLS_PORT },
	{ value: "none", label: "None (LAN relay)", port: PLAINTEXT_SMTP_PORT },
];

const DEFAULT_TLS_MODE: TLSMode = "starttls";
const DEFAULT_SMTP_PORT = TLS_MODES.find((m) => m.value === DEFAULT_TLS_MODE)!.port;

export function SMTPSettings() {
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [notice, setNotice] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);
	const [testing, setTesting] = useState(false);

	const [enabled, setEnabled] = useState(false);
	const [host, setHost] = useState("");
	const [port, setPort] = useState(DEFAULT_SMTP_PORT);
	const [username, setUsername] = useState("");
	const [fromAddress, setFromAddress] = useState("");
	const [tlsMode, setTlsMode] = useState<TLSMode>(DEFAULT_TLS_MODE);

	const [passwordSet, setPasswordSet] = useState(false);
	const [passwordIntent, setPasswordIntent] = useState<"keep" | "set" | "clear">("keep");
	const [password, setPassword] = useState("");

	const [testTo, setTestTo] = useState("");

	const applyConfig = useCallback((c: SMTPConfig) => {
		setEnabled(c.enabled);
		setHost(c.host);
		setPort(c.port || DEFAULT_SMTP_PORT);
		setUsername(c.username);
		setFromAddress(c.from_address);
		setTlsMode(c.tls_mode);
		setPasswordSet(c.password_set);
		setPassword("");
		setPasswordIntent("keep");
	}, []);

	useEffect(() => {
		api.smtpConfig()
			.then(applyConfig)
			.catch((err) => setError(err instanceof Error ? err.message : "Failed to load SMTP config."))
			.finally(() => setLoading(false));
	}, [applyConfig]);

	const buildPayload = (): SMTPConfigUpdate => {
		const payload: SMTPConfigUpdate = {
			enabled,
			host: host.trim(),
			port,
			username: username.trim(),
			from_address: fromAddress.trim(),
			tls_mode: tlsMode,
		};
		if (passwordIntent === "clear") {
			payload.password = "";
		} else if (passwordIntent === "set" && password) {
			payload.password = password;
		}
		// "keep" (or "set" with an empty field): omit password
		return payload;
	};

	const flashNotice = (msg: string) => {
		setNotice(msg);
		setTimeout(() => setNotice(null), 3000);
	};

	const handleSave = async () => {
		setError(null);
		setNotice(null);
		setSaving(true);
		try {
			const updated = await api.updateSMTPConfig(buildPayload());
			applyConfig(updated);
			flashNotice("SMTP configuration saved.");
		} catch (err) {
			setError(err instanceof HttpError ? err.message : "Failed to save SMTP config.");
		} finally {
			setSaving(false);
		}
	};

	const handleTest = async () => {
		setError(null);
		setNotice(null);
		if (!testTo.trim()) {
			setError("Enter a recipient address to send a test message.");
			return;
		}
		setTesting(true);
		try {
			await api.testSMTPConfig({ ...buildPayload(), test_to: testTo.trim() });
			flashNotice(`Test message sent to ${testTo.trim()}.`);
		} catch (err) {
			setError(err instanceof HttpError ? err.message : "Test send failed.");
		} finally {
			setTesting(false);
		}
	};

	if (loading) {
		return (
			<div style={cardStyle}>
				<div style={sectionHeaderStyle}>Email (SMTP)</div>
				<div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.textDim }}>
					Loading…
				</div>
			</div>
		);
	}
 
	return (
		<div style={cardStyle}>
			<div style={sectionHeaderStyle}>Email (SMTP)</div>
 
			<div
				style={{
					fontSize: 11,
					fontFamily: themeVars.font,
					color: themeVars.textDim,
					marginBottom: 16,
				}}
			>
				Server-wide transport for email alert channels. Configured once for all users.
			</div>
 
			{error && (
				<div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.danger, marginBottom: 12 }}>
					{error}
				</div>
			)}
			{notice && (
				<div style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.ok, marginBottom: 12 }}>
					{notice}
				</div>
			)}
 
			<div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
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
					<span>Enable email delivery</span>
				</label>
 
				{/* Host + port */}
				<div style={{ display: "flex", gap: 12 }}>
					<div style={{ flex: 1 }}>
						<div style={labelStyle}>Host</div>
						<input
							type="text"
							value={host}
							onChange={(e) => setHost(e.target.value)}
							placeholder="smtp.example.com"
							style={inputStyle}
						/>
					</div>
					<div style={{ width: 120 }}>
						<div style={labelStyle}>Port</div>
						<input
							type="number"
							min={1}
							max={65535}
							value={port}
							onChange={(e) => setPort(Number(e.target.value))}
							style={inputStyle}
						/>
					</div>
				</div>
 
				{/* TLS mode */}
				<div>
					<div style={labelStyle}>TLS Mode</div>
					<select
						value={tlsMode}
						onChange={(e) => {
							const mode = e.target.value as TLSMode;
							setTlsMode(mode);
							// Suggest the conventional port for the chosen mode if the
							// current port is empty or a known default for another mode.
							const known = TLS_MODES.map((m) => m.port);
							if (!port || known.includes(port)) {
								const match = TLS_MODES.find((m) => m.value === mode);
								if (match) setPort(match.port);
							}
						}}
						style={{ ...inputStyle, cursor: "pointer" }}
					>
						{TLS_MODES.map((m) => (
							<option key={m.value} value={m.value}>{m.label}</option>
						))}
					</select>
				</div>
 
				{/* Username */}
				<div>
					<div style={labelStyle}>Username</div>
					<input
						type="text"
						value={username}
						onChange={(e) => setUsername(e.target.value)}
						placeholder="(optional for unauthenticated relays)"
						style={inputStyle}
						autoComplete="off"
					/>
				</div>
 
				{/* Password — explicit intent (keep / set / clear), committed on Save */}
				<div>
					<div style={labelStyle}>Password</div>
 
					{!passwordSet && passwordIntent !== "set" ? (
						// No password stored yet: offer to set one.
						<div style={{ display: "flex", alignItems: "center", gap: 8 }}>
							<span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.textDim }}>
								No password stored.
							</span>
							<button
								type="button"
								onClick={() => setPasswordIntent("set")}
								style={smallBtn}
							>
								Set Password
							</button>
						</div>
					) : passwordIntent === "set" ? (
						// Setting (or replacing) the password: reveal the input.
						<div>
							<input
								type="password"
								value={password}
								onChange={(e) => setPassword(e.target.value)}
								placeholder="Enter new password"
								style={inputStyle}
								autoComplete="new-password"
								autoFocus
							/>
							<button
								type="button"
								onClick={() => {
									setPassword("");
									setPasswordIntent("keep");
								}}
								style={{ ...smallBtn, marginTop: 6 }}
							>
								Cancel
							</button>
						</div>
					) : passwordIntent === "clear" ? (
						// Marked for clearing on save.
						<div style={{ display: "flex", alignItems: "center", gap: 8 }}>
							<span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.warn }}>
								Password will be cleared on save.
							</span>
							<button
								type="button"
								onClick={() => setPasswordIntent("keep")}
								style={smallBtn}
							>
								Keep Current
							</button>
						</div>
					) : (
						// passwordSet && intent === "keep": stored, untouched.
						<div style={{ display: "flex", alignItems: "center", gap: 8 }}>
							<span style={{ fontSize: 12, fontFamily: themeVars.font, color: themeVars.text }}>
								•••••••• stored
							</span>
							<button
								type="button"
								onClick={() => setPasswordIntent("set")}
								style={smallBtn}
							>
								Change
							</button>
							<button
								type="button"
								onClick={() => setPasswordIntent("clear")}
								style={{ ...smallBtn, color: themeVars.danger, borderColor: themeVars.danger }}
							>
								Clear
							</button>
						</div>
					)}
				</div>
 
				{/* From address */}
				<div>
					<div style={labelStyle}>From Address</div>
					<input
						type="text"
						value={fromAddress}
						onChange={(e) => setFromAddress(e.target.value)}
						placeholder="alerts@example.com"
						style={inputStyle}
					/>
				</div>
			</div>
 
			{/* Save */}
			<div style={{ display: "flex", gap: 8, marginTop: 20, alignItems: "center" }}>
				<button
					onClick={handleSave}
					disabled={saving}
					style={{ ...btnStyle, opacity: saving ? 0.6 : 1 }}
				>
					{saving ? "Saving..." : "Save"}
				</button>
			</div>
 
			{/* Test send */}
			<div
				style={{
					marginTop: 20,
					paddingTop: 16,
					borderTop: `1px solid ${themeVars.border}`,
				}}
			>
				<div style={labelStyle}>Send Test Message</div>
				<div style={{ display: "flex", gap: 8, alignItems: "center" }}>
					<input
						type="text"
						value={testTo}
						onChange={(e) => setTestTo(e.target.value)}
						placeholder="you@example.com"
						style={{ ...inputStyle, flex: 1 }}
					/>
					<button
						onClick={handleTest}
						disabled={testing}
						style={{
							...btnStyle,
							color: themeVars.text,
							background: "transparent",
							borderColor: themeVars.border,
							opacity: testing ? 0.6 : 1,
							whiteSpace: "nowrap",
						}}
					>
						{testing ? "Sending..." : "Send Test"}
					</button>
				</div>
				<div
					style={{
						fontSize: 10,
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						marginTop: 4,
					}}
				>
					Sends using the current form values (live send, not saved).
				</div>
			</div>
		</div>
	);
}