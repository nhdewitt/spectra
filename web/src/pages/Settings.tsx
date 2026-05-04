import { useState } from "react";
import { themeVars, themes, getThemeName, applyTheme } from "../theme";
import type { ThemeName } from "../theme";
import type { User } from "../types";

interface SettingsProps {
    user: User;
    onLogout: () => void;
}

export function Settings({ user, onLogout }: SettingsProps) {
    const themeNames = Object.keys(themes) as ThemeName[];
    const [activeTheme, setActiveTheme] = useState<ThemeName>(getThemeName());

    const handleThemeSelect = (name: ThemeName) => {
        applyTheme(name);
        setActiveTheme(name);
    };

    return (
        <div style={{ padding: 24, maxWidth: 600 }}>
        <div
            style={{
            fontFamily: themeVars.font,
            fontSize: 18,
            fontWeight: 600,
            color: themeVars.text,
            marginBottom: 24,
            }}
        >
            Settings
        </div>

        {/* Account info */}
        <div
            style={{
            background: themeVars.surface,
            border: `1px solid ${themeVars.border}`,
            padding: 20,
            marginBottom: 20,
            }}
        >
            <div
            style={{
                fontSize: 10,
                fontFamily: themeVars.font,
                color: themeVars.textDim,
                letterSpacing: "0.05em",
                textTransform: "uppercase",
                marginBottom: 12,
            }}
            >
            Account
            </div>
            <div
            style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                fontSize: 13,
                fontFamily: themeVars.font,
            }}
            >
            <div>
                <div style={{ color: themeVars.text, fontWeight: 500 }}>
                {user.username}
                </div>
                <div style={{ color: themeVars.textDim, fontSize: 11, marginTop: 2 }}>
                {user.role}
                </div>
            </div>
            <button
                onClick={onLogout}
                style={{
                padding: "6px 14px",
                fontSize: 11,
                fontFamily: themeVars.font,
                color: themeVars.danger,
                background: "transparent",
                border: `1px solid ${themeVars.danger}`,
                cursor: "pointer",
                letterSpacing: "0.03em",
                textTransform: "uppercase",
                }}
            >
                Logout
            </button>
            </div>
        </div>

        {/* Theme selector */}
        <div
            style={{
            background: themeVars.surface,
            border: `1px solid ${themeVars.border}`,
            padding: 20,
            }}
        >
            <div
            style={{
                fontSize: 10,
                fontFamily: themeVars.font,
                color: themeVars.textDim,
                letterSpacing: "0.05em",
                textTransform: "uppercase",
                marginBottom: 12,
            }}
            >
            Theme
            </div>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            {themeNames.map((name) => (
                <button
                key={name}
                onClick={() => handleThemeSelect(name)}
                style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    padding: "8px 14px",
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: activeTheme === name ? themeVars.text : themeVars.textMuted,
                    background: activeTheme === name ? themeVars.accentDim : "transparent",
                    border: `1px solid ${activeTheme === name ? themeVars.accent : themeVars.border}`,
                    cursor: "pointer",
                    letterSpacing: "0.02em",
                }}
                >
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
        </div>
    </div>
  );
}