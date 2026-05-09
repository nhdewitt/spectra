import React, { useState, useEffect, useCallback } from "react";
import { api } from "../api";
import { themeVars } from "../theme";
import {
	tableHeaderStyle,
	tableCellStyle,
	tableMutedCellStyle,
	LoadingSpinner,
} from "../components/ui";
import type { User, ManagedUser } from "../types";

const btnStyle: React.CSSProperties = {
	padding: "6px 14px",
	fontSize: 11,
	fontFamily: themeVars.font,
	color: themeVars.text,
	background: themeVars.accentDim,
	border: `1px solid ${themeVars.accent}`,
	cursor: "pointer",
	textTransform: "uppercase",
	letterSpacing: "0.03em"	
};

const roleBadge = (role: string): React.CSSProperties => {
	const color =
		role === "superadmin"
			? themeVars.accent
			: role === "admin"
				? themeVars.warn
				: themeVars.textDim;
	
	return {
		fontSize: 9,
		fontFamily: themeVars.font,
		fontWeight: 600,
		color,
		background: `color-mix(in-srgb, ${color} 15%, transparent)`,
		border: `1px solid ${color}`,
		padding: "1px 6px",
		letterSpacing: "0.04em",
		textTransform: "uppercase",
	};
};

function CreateUserModal({
	callerRole,
	onClose,
	onCreated,
}: {
	callerRole: string;
	onClose: () => void;
	onCreated: () => void;
}) {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [role, setRole] = useState("viewer");
	const [error, setError] = useState<string | null>(null);
	const [creating, setCreating] = useState(false);

	const handleCreate = async () => {
		setError(null);
		if (!username.trim()) {
			setError("Username is required.");
			return;
		}
		if (password.length < 8) {
			setError("Password must be at least 8 characters.");
			return;
		}
		setCreating(true);
		try {
			await api.createUser(username.trim(), password, role);
			onCreated();
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to create user.");
		} finally {
			setCreating(false);
		}
	};

	const availableRoles =
		callerRole === "superadmin"
			? ["viewer", "admin", "superadmin"]
			: ["viewer"];

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
					maxWidth: 440,
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
						Create User
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
						<div
							style={{
								fontSize: 11,
								fontFamily: themeVars.font,
								color: themeVars.textDim,
								textTransform: "uppercase",
								letterSpacing: "0.04em",
								marginBottom: 4,
							}}
						>
							Username
						</div>
						<input
							type="text"
							value={username}
							onChange={(e) => setUsername(e.target.value)}
							style={{
								width: "100%",
								padding: "6px 10px",
								fontSize: 12,
								fontFamily: themeVars.font,
								color: themeVars.text,
								background: themeVars.surface,
								border: `1px solid ${themeVars.border}`,
								boxSizing: "border-box",
							}}
						/>
					</div>

					<div>
						<div
							style={{
								fontSize: 11,
								fontFamily: themeVars.font,
								color: themeVars.textDim,
								textTransform: "uppercase",
								letterSpacing: "0.04em",
								marginBottom: 4,
							}}
						>
							Password
						</div>
						<input
							type="password"
							value={password}
							onChange={(e) => setPassword(e.target.value)}
							style={{
								width: "100%",
								padding: "6px 10px",
								fontSize: 12,
								fontFamily: themeVars.font,
								color: themeVars.text,
								background: themeVars.surface,
								border: `1px solid ${themeVars.border}`,
								boxSizing: "border-box",
							}}
						/>
					</div>

					<div>
						<div
							style={{
								fontSize: 11,
								fontFamily: themeVars.font,
								color: themeVars.textDim,
								textTransform: "uppercase",
								letterSpacing: "0.04em",
								marginBottom: 4,
							}}
						>
							Role
						</div>
						<select
							value={role}
							onChange={(e) => setRole(e.target.value)}
							style={{
								padding: "6px 10px",
								fontSize: 12,
								fontFamily: themeVars.font,
								color: themeVars.text,
								background: themeVars.surface,
								border: `1px solid ${themeVars.border}`,
								cursor: "pointer",
							}}
						>
							{availableRoles.map((r) => (
								<option key={r} value={r}>
									{r}
								</option>
							))}
						</select>
					</div>
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
						onClick={handleCreate}
						disabled={creating}
						style={{
							...btnStyle,
							opacity: creating ? 0.6 : 1,
						}}
					>
						{creating ? "Creating..." : "Create User"}
					</button>
				</div>
			</div>
		</div>
	);
}

interface UserManagementProps {
	user: User;
}

export function UserManagement({ user }: UserManagementProps) {
	const [users, setUsers] = useState<ManagedUser[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [showCreate, setShowCreate] = useState(false);
	const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
	const [roleEdit, setRoleEdit] = useState<string | null>(null);

	const loadUsers = useCallback(() => {
		api.listUsers()
			.then(setUsers)
			.catch((err) =>
				setError(err instanceof Error ? err.message : "Failed to load users")
			)
			.finally(() => setLoading(false));
	}, []);

	useEffect(() => {
		setLoading(true);
		loadUsers();
	}, [loadUsers]);

	const handleDelete = async (id: string) => {
		try {
			await api.deleteUser(id);
			setConfirmDelete(null);
			loadUsers();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to delete user.");
			setTimeout(() => setError(null), 3000);
		}
	};

	const handleRoleChange = async (id: string, newRole: string) => {
		try {
			await api.updateUserRole(id, newRole);
			setRoleEdit(null);
			loadUsers();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to update role.");
			setTimeout(() => setError(null), 3000);
		}
	};

	const isSuperAdmin = user.role === "superadmin";

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
				User Management
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
				<button onClick={() => setShowCreate(true)} style={btnStyle}>
					+ Create User
				</button>
				<span
					style={{
						fontSize: 11,
						fontFamily: themeVars.font,
						color: themeVars.textDim,
						marginLeft: "auto",
					}}
				>
					{users.length} user{users.length === 1 ? "" : "s"}
				</span>
			</div>

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
							<th style={tableHeaderStyle}>Username</th>
							<th style={tableHeaderStyle}>Role</th>
							<th style={tableHeaderStyle}>Created</th>
							<th style={{ ...tableHeaderStyle, textAlign: "right" }}>Actions</th>
						</tr>
					</thead>
					<tbody>
						{users.map((u, i) => {
							const isSelf = u.id === user.id;
							return (
								<tr
									key={u.id}
									style={{
										background: i % 2 === 0 ? "transparent" : themeVars.surfaceHover,
									}}
								>
									<td style={{ ...tableCellStyle, fontWeight: 500 }}>
										{u.username}
										{isSelf && (
											<span
												style={{
													fontSize: 9,
													fontFamily: themeVars.font,
													color: themeVars.textDim,
													marginLeft: 6,
												}}
											>
												(you)
											</span>
										)}
									</td>
									<td style={tableCellStyle}>
										{roleEdit === u.id && isSuperAdmin ? (
											<select
												value={u.role}
												onChange={(e) => handleRoleChange(u.id, e.target.value)}
												onBlur={() => setRoleEdit(null)}
												autoFocus
												style={{
													padding: "2px 6px",
													fontSize: 11,
													fontFamily: themeVars.font,
													color: themeVars.text,
													background: themeVars.surface,
													border: `1px solid ${themeVars.border}`,
													cursor: "pointer",
												}}
											>
												<option value="viewer">viewer</option>
												<option value="admin">admin</option>
												<option value="superadmin">superadmin</option>
											</select>
										) : (
											<span
												style={{
													...roleBadge(u.role),
													cursor: isSuperAdmin && !isSelf ? "pointer" : "default",
												}}
												onClick={() => {
													if (isSuperAdmin && !isSelf) setRoleEdit(u.id);
												}}
												title={isSuperAdmin && !isSelf ? "Click to change role" : undefined}
											>
												{u.role}
											</span>
										)}
									</td>
									<td style={tableMutedCellStyle}>
										{new Date(u.created_at).toLocaleDateString(undefined, {
											month: "short",
											day: "numeric",
											year: "numeric",
										})}
									</td>
									<td style={{ ...tableCellStyle, textAlign: "right" }}>
										{confirmDelete === u.id ? (
											<div style={{ display: "flex", gap: 6, justifyContent: "flex-end", alignItems: "center" }}>
												<span
													style={{
														fontSize: 11,
														fontFamily: themeVars.font,
														color: themeVars.danger,
													}}
												>
													Delete {u.username}?
												</span>
												<button
													onClick={() => handleDelete(u.id)}
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
											!isSelf && (
												<button
													onClick={() => setConfirmDelete(u.id)}
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
											)
										)}
									</td>
								</tr>
							);
						})}
					</tbody>
				</table>
			</div>

			{showCreate && (
				<CreateUserModal
					callerRole={user.role}
					onClose={() => setShowCreate(false)}
					onCreated={loadUsers}
				/>
			)}
		</div>
	);
}