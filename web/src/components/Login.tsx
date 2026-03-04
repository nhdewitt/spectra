import React, { useState } from "react";
import { api, HttpError } from "../api";
import { theme } from "../theme";
import type { User } from "../types";

const labelStyle: React.CSSProperties = {
    display: "block",
    fontSize: 11,
    fontFamily: theme.font,
    color: theme.textMuted,
    marginBottom: 6,
    letterSpacing: "0.05em",
    textTransform: "uppercase",
};

const inputStyle: React.CSSProperties = {
    width: "100%",
    padding: "10px 12px",
    background: theme.bg,
    border: `1px solid ${theme.border}`,
    color: theme.text,
    fontSize: 14,
    fontFamily: theme.font,
    outline: "none",
    boxSizing: "border-box",
};

export function Login({ onLogin }: { onLogin: (user: User) => void }) {
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        setError("");
        setLoading(true);
        try {
            const user = await api.login(username, password);
            onLogin(user);
        } catch (err) {
            if (err instanceof HttpError && err.status === 404) {
                setError("Login service is unavailable. Check server URL/route.")
            } else {
                setError(err instanceof Error ? err.message : "Login failed");
            }
        } finally {
            setLoading(false);
        }
    };

    return (
        <div
            style={{
                minHeight: "100vh",
                background: theme.bg,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontFamily: theme.fontSans,
                position: "relative",
                overflow: "hidden",
            }}
        >
            {/* Halftone background pattern */}
            <svg
                style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: "100%",
                    pointerEvents: "none",
                }}
                xmlns="http://www.w3.org/2000/svg"
                preserveAspectRatio="none"
            >
                <defs>
                    <pattern
                        id="halftone"
                        x="0"
                        y="0"
                        width="16"
                        height="16"
                        patternUnits="userSpaceOnUse"
                        patternTransform="rotate(-45)"
                    >
                        <rect width="16" height="16" fill="transparent" />
                        <rect
                            x="6"
                            y="6"
                            width="4.5"
                            height="4.5"
                            fill={theme.text}
                            transform="rotate(45 8 8)"
                        />
                    </pattern>
                    <linearGradient id="halftone-fade" x1="1" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="white" stopOpacity="0.18" />
                        <stop offset="45%" stopColor="white" stopOpacity="0.08" />
                        <stop offset="100%" stopColor="white" stopOpacity="0" />
                    </linearGradient>
                    <mask id="halftone-mask">
                        <rect width="100%" height="100%" fill="url(#halftone-fade)" />
                    </mask>
                </defs>
                <rect
                    width="100%"
                    height="100%"
                    fill="url(#halftone)"
                    mask="url(#halftone-mask"
                />
            </svg>

            {/* Login form */}
            <div
                style={{
                    width: 360,
                    background: theme.surface,
                    border: `1px solid ${theme.border}`,
                    padding: "40px 32px",
                    position: "relative",
                    zIndex: 1,
                }}
            >
                <div
                    style={{
                        fontFamily: theme.font,
                        fontSize: 20,
                        fontWeight: 600,
                        color: theme.text,
                        marginBottom: 4,
                        letterSpacing: "-0.02em",
                    }}
                >
                    SPECTRA
                </div>
                <div style={{ fontSize: 13, color: theme.textMuted, marginBottom: 32 }}>
                    System Monitoring
                </div>

                <form onSubmit={handleSubmit}>
                    <label style={labelStyle}>Username</label>
                    <input
                        type="text"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        style={inputStyle}
                        autoFocus
                        autoComplete="username"
                    />

                    <label style={{ ...labelStyle, marginTop: 16 }}>Password</label>
                    <input
                        type="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        style={inputStyle}
                        autoComplete="current-password"
                    />

                    {error && (
                        <div
                            style={{
                                color: theme.danger,
                                fontSize: 13,
                                marginTop: 12,
                                fontFamily: theme.font,
                            }}
                        >
                            {error}
                        </div>
                    )}

                    <button
                        type="submit"
                        disabled={loading || !username || !password}
                        style={{
                            width: "100%",
                            marginTop: 24,
                            padding: "10px 0",
                            background: loading ? theme.accentDim : theme.accent,
                            color: "#fff",
                            border: "none",
                            fontSize: 14,
                            fontFamily: theme.font,
                            fontWeight: 500,
                            cursor: loading ? "wait" : "pointer",
                            letterSpacing: "0.02em",
                            opacity: !username || !password ? 0.5 : 1,
                        }}
                    >
                        {loading ? "..." : "LOGIN" }
                    </button>
                </form>
            </div>
        </div>
    );
}