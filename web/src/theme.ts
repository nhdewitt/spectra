export interface Theme {
    bg: string;
    surface: string;
    surfaceHover: string;
    border: string;
    borderLight: string;
    text: string;
    textMuted: string;
    textDim: string;
    accent: string;
    accentDim: string;
    danger: string;
    warn: string;
    ok: string;
    font: string;
    fontSans: string;
}

// --- Themes ---

const midnight: Theme = {
    bg: "#0a0a0a",
    surface: "#141414",
    surfaceHover: "#1a1a1a",
    border: "#262626",
    borderLight: "#333",
    text: "#e5e5e5",
    textMuted: "#737373",
    textDim: "#525252",
    accent: "#3b82f6",
    accentDim: "#1e3a5f",
    danger: "#ef4444",
    warn: "#eab308",
    ok: "#22c55e",
    font: "'JetBrains Mono', 'Fira Code', 'SF Mono', monospace",
    fontSans: "'IBM Plex Sans', -apple-system, -sans-serif",
};

const terminal: Theme = {
    bg: "#000000",
    surface: "#0a0a0a",
    surfaceHover: "#111",
    border: "#1a1a1a",
    borderLight: "#222",
    text: "#00ff00",
    textMuted: "#00aa00",
    textDim: "#006600",
    accent: "#00ff00",
    accentDim: "#003300",
    danger: "#ff3333",
    warn: "#ffff00",
    ok: "#00ff00",
    font: "'VT323', 'Courier New', monospace",
    fontSans: "'VT323', 'Courier New', monospace",
};

const classic: Theme = {
    bg: "#c0c0c0",
    surface: "#dfdfdf",
    surfaceHover: "#d0d0d0",
    border: "#808080",
    borderLight: "#a0a0a0",
    text: "#000",
    textMuted: "#444",
    textDim: "#666",
    accent: "#000080",
    accentDim: "#c0c0e0",
    danger: "#cc0000",
    warn: "#808000",
    ok: "#008000",
    font: "'Consolas', 'Courier New', monospace",
    fontSans: "'Segoe UI', 'Tahoma', sans-serif",
};

const nord: Theme = {
    bg: "#2e3440",
    surface: "#3b4252",
    surfaceHover: "#434c5e",
    border: "#4c566a",
    borderLight: "#555e6e",
    text: "#eceff4",
    textMuted: "#d8dee9",
    textDim: "#7b88a1",
    accent: "#88c0d0",
    accentDim: "#3b5468",
    danger: "#bf616a",
    warn: "#ebcb8b",
    ok: "#a3be8c",
    font: "'JetBrains Mono', 'Fira Code', monospace",
    fontSans: "'Inter', -apple-system, sans-serif",
};

const solarized: Theme = {
  bg: "#002b36",
  surface: "#073642",
  surfaceHover: "#094050",
  border: "#586e75",
  borderLight: "#657b83",
  text: "#fdf6e3",
  textMuted: "#93a1a1",
  textDim: "#657b83",
  accent: "#268bd2",
  accentDim: "#0d3a5c",
  danger: "#dc322f",
  warn: "#b58900",
  ok: "#859900",
  font: "'JetBrains Mono', 'Fira Code', monospace",
  fontSans: "'IBM Plex Sans', -apple-system, sans-serif",
};

const light: Theme = {
    bg: "#f5f5f5",
    surface: "#ffffff",
    surfaceHover: "#f0f0f0",
    border: "#d4d4d4",
    borderLight: "#e5e5e5",
    text: "#111",
    textMuted: "#666",
    textDim: "#999",
    accent: "#2563eb",
    accentDim: "#dbeafe",
    danger: "#dc2626",
    warn: "#ca8a04",
    ok: "#16a34a",
    font: "'JetBrains Mono', 'Fira Code', monospace",
    fontSans: "'Inter', sans-serif",
};

export const themes = {
    midnight,
    terminal,
    classic,
    nord,
    solarized,
    light,
} satisfies Record<string, Theme>;

export type ThemeName = keyof typeof themes;

// --- Active Theme ---

const STORAGE_KEY = "spectra-theme";
const DEFAULT_THEME: ThemeName = "midnight";

function loadThemeName(): ThemeName {
    try {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored && stored in themes) return stored as ThemeName;
    } catch {}
    return DEFAULT_THEME;
}

function saveThemeName(name: ThemeName) {
    try {
        localStorage.setItem(STORAGE_KEY, name);
    } catch {}
}

let currentThemeName: ThemeName = loadThemeName();

export function getThemeName(): ThemeName {
    return currentThemeName;
}

export function getTheme(name: ThemeName = currentThemeName): Theme {
    return themes[name];
}

function setCSSVar(name: string, value: string) {
    document.documentElement.style.setProperty(name, value);
}

export function applyTheme(name: ThemeName) {
    const t = themes[name];

    setCSSVar("--bg", t.bg);
    setCSSVar("--surface", t.surface);
    setCSSVar("--surface-hover", t.surfaceHover);
    setCSSVar("--border", t.border);
    setCSSVar("--border-light", t.borderLight);
    setCSSVar("--text", t.text);
    setCSSVar("--text-muted", t.textMuted);
    setCSSVar("--text-dim", t.textDim);
    setCSSVar("--accent", t.accent);
    setCSSVar("--accent-dim", t.accentDim);
    setCSSVar("--danger", t.danger);
    setCSSVar("--warn", t.warn);
    setCSSVar("--ok", t.ok);
    setCSSVar("--font-mono", t.font);
    setCSSVar("--font-sans", t.fontSans);

    currentThemeName = name;
    saveThemeName(name);    
}

export function initTheme() {
    applyTheme(currentThemeName);
}

export const themeVars = {
    bg: "var(--bg)",
    surface: "var(--surface)",
    surfaceHover: "var(--surface-hover)",
    border: "var(--border)",
    borderLight: "var(--border-light)",
    text: "var(--text)",
    textMuted: "var(--text-muted)",
    textDim: "var(--text-dim)",
    accent: "var(--accent)",
    accentDim: "var(--accent-dim)",
    danger: "var(--danger)",
    warn: "var(--warn)",
    ok: "var(--ok)",
    font: "var(--font-mono)",
    fontSans: "var(--font-sans)",    
} as const;