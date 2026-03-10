import { theme } from "../theme";
import type { User, Page } from "../types";

interface HeaderProps {
    user: User;
    onLogout: () => void;
    onNavigate: (page: Page) => void;
    currentPage: Page;
}

export function Header({ user, onLogout, onNavigate, currentPage }: HeaderProps) {
    const navItems: { key: Page; label: string }[] = [
        { key: "overview", label: "Overview" },
        { key: "agents", label: "Agents" },
    ];
    if (user.role === "admin") {
        navItems.push({ key: "admin", label: "Admin" });
    }

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "0 24px",
                height: 48,
                background: theme.surface,
                borderBottom: `1px solid ${theme.border}`,
                fontFamily: theme.font,
            }}
        >
            <div style={{ display: "flex", alignItems: "center", gap: 24 }}>
                <span
                    style={{
                        fontSize: 15,
                        fontWeight: 600,
                        color: theme.text,
                        letterSpacing: "-0.02em",
                        cursor: "pointer",
                    }}
                    onClick={() => onNavigate("overview")}
                >
                    SPECTRA
                </span>
                <div style={{ display: "flex", gap: 4 }}>
                    {navItems.map((item) => (
                        <button
                            key={item.key}
                            onClick={() => onNavigate(item.key)}
                            style={{
                                padding: "6px 12px",
                                fontSize: 12,
                                fontFamily: theme.font,
                                color: currentPage === item.key ? theme.text : theme.textMuted,
                                background:
                                    currentPage === item.key ? theme.accentDim : "transparent",
                                border: "none",
                                cursor: "pointer",
                                letterSpacing: "0.02em",
                            }}
                        >
                            {item.label}
                        </button>
                    ))}
                </div>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                <span style={{ fontSize: 12, color: theme.textMuted }}>
                    {user.username}
                </span>
                <button
                    onClick={onLogout}
                    style={{
                        padding: "4px 10px",
                        fontSize: 11,
                        fontFamily: theme.font,
                        color: theme.textMuted,
                        background: "transparent",
                        border: `1px solid ${theme.border}`,
                        cursor: "pointer",
                        letterSpacing: "0.02em",
                    }}
                >
                    LOGOUT
                </button>
            </div>
        </div>
    );
}