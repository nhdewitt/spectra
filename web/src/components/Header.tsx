import { useState } from "react";
import { themeVars, themes, getThemeName, applyTheme } from "../theme";
import type { ThemeName } from "../theme";
import type { User, Page } from "../types";

interface HeaderProps {
    user: User;
    onLogout: () => void;
    onNavigate: (page: Page) => void;
    currentPage: Page;
}

export function Header({ user, onLogout, onNavigate, currentPage }: HeaderProps) {
    const [showThemes, setShowThemes] = useState(false);

    const navItems: { key: Page; label: string }[] = [
        { key: "overview", label: "Overview" },
        { key: "agents", label: "Agents" },
    ];
    if (user.role === "admin") {
        navItems.push({ key: "admin", label: "Admin" });
    }

    const themeNames = Object.keys(themes) as ThemeName[];
    const [activeTheme, setActiveTheme] = useState<ThemeName>(getThemeName());

    const handleThemeSelect = (name: ThemeName) => {
        if (name === activeTheme) {
            setShowThemes(false);
            return;
        }
        applyTheme(name);
        setActiveTheme(name);
        setShowThemes(false);
    };

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "0 24px",
                height: 48,
                background: themeVars.surface,
                borderBottom: `1px solid ${themeVars.border}`,
                fontFamily: themeVars.font,
            }}
        >
            <div style={{ display: "flex", alignItems: "center", gap: 24 }}>
                <span
                    style={{
                        fontSize: 15,
                        fontWeight: 600,
                        color: themeVars.text,
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
                                fontFamily: themeVars.font,
                                color: currentPage === item.key ? themeVars.text : themeVars.textMuted,
                                background:
                                    currentPage === item.key ? themeVars.accentDim : "transparent",
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
                {/* Theme picker */}
                <div style={{ position: "relative" }}>
                    <button
                        onClick={() => setShowThemes((v) => !v)}
                        style={{
                            padding: "4px 10px",
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.textMuted,
                            background: "transparent",
                            border: `1px solid ${themeVars.border}`,
                            cursor: "pointer",
                            letterSpacing: "0.02em",
                        }}
                    >
                        {getThemeName().toUpperCase()}
                    </button>

                    {showThemes && (
                        <>
                            {/* Backdrop to close */}
                            <div
                                style={{
                                    position: "fixed",
                                    top: 0,
                                    left: 0,
                                    right: 0,
                                    bottom: 0,
                                    zIndex: 99,
                                }}
                                onClick={() => setShowThemes(false)}
                            />
                            <div
                                style={{
                                    position: "absolute",
                                    top: "calc(100% + 4px)",
                                    right: 0,
                                    background: themeVars.surface,
                                    border: `1px solid ${themeVars.border}`,
                                    zIndex: 100,
                                    minWidth: 140,
                                }}
                            >
                                {themeNames.map((name) => (
                                    <button
                                        key={name}
                                        onClick={() => handleThemeSelect(name)}
                                        style={{
                                            display: "flex",
                                            alignItems: "center",
                                            gap: 8,
                                            width: "100%",
                                            padding: "8px 12px",
                                            fontSize: 11,
                                            fontFamily: themeVars.font,
                                            color:
                                                getThemeName() === name
                                                    ? themeVars.text
                                                    : themeVars.textMuted,
                                            background:
                                                getThemeName() === name
                                                    ? themeVars.accentDim
                                                    : "transparent",
                                            border: "none",
                                            cursor: "pointer",
                                            textAlign: "left",
                                            letterSpacing: "0.02em",
                                        }}
                                    >
                                        {/* Color preview dots */}
                                        <span style={{ display: "flex", gap: 2 }}>
                                            <span
                                                style={{
                                                    width: 8,
                                                    height: 8,
                                                    borderRadius: "50%",
                                                    background: themes[name]?.bg,
                                                    border: `1px solid ${themes[name]?.border}`,
                                                }}
                                            />
                                            <span
                                                style={{
                                                    width: 8,
                                                    height: 8,
                                                    borderRadius: "50%",
                                                    background: themes[name]?.accent,
                                                }}
                                            />
                                            <span
                                                style={{
                                                    width: 8,
                                                    height: 8,
                                                    borderRadius: "50%",
                                                    background: themes[name]?.text,
                                                }}
                                            />
                                        </span>
                                        {name.toUpperCase()}
                                    </button>
                                ))}
                            </div>
                        </>
                    )}
                </div>

                <span style={{ fontSize: 12, color: themeVars.textMuted }}>
                    {user.username}
                </span>
                <button
                    onClick={onLogout}
                    style={{
                        padding: "4px 10px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textMuted,
                        background: "transparent",
                        border: `1px solid ${themeVars.border}`,
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