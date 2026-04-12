import React, { useState, useEffect, useRef, useCallback, useMemo } from "react";
import { getThemeName, themeVars, type ThemeName } from "../theme";

const THEME_PALETTES: Record<ThemeName, string[]> = {
  midnight: [
      "#5b9cf5",
      "#ef6b6b",
      "#36cda4",
      "#e8b832",
      "#b07ce8",
      "#e87e3a",
  ],
  terminal: [
      "#33ff33",
      "#ff5555",
      "#55bbff",
      "#ffff55",
      "#ff55ff",
      "#55ffdd",
  ],
  classic: [
      "#1a56db",
      "#c81e1e",
      "#138a3e",
      "#c27803",
      "#7e22ce",
      "#0e7490",
  ],
  nord: [
      "#88c0d0",
      "#bf616a",
      "#a3be8c",
      "#ebcb8b",
      "#b48ead",
      "#d08770",
  ],
  solarized: [
      "#268bd2",
      "#dc322f",
      "#859900",
      "#b58900",
      "#6c71c4",
      "#2aa198",
  ],
  light: [
      "#1a56db",
      "#c81e1e",
      "#138a3e",
      "#c27803",
      "#7e22ce",
      "#0e7490",
  ],
};

// Line styles: solid -> duotone -> dashed -> duotone+dashed -> ...
const DASH_PATTERNS = ["", "6 3", "2 3"];

interface LineStyle {
  colorA: string;
  colorB: string | null; // solid | duotone
  dash: string;
  gradientId: string | null;
}

function getLineStyle(index: number, palette: string[]): LineStyle {
  const n = palette.length;
  const colorIdx = index % n;
  const tier = Math.floor(index / n) % 6;
  const isDuotone = tier % 2 === 1;
  const dashIdx = Math.floor(tier / 2) % DASH_PATTERNS.length;
  const colorA = palette[colorIdx] ?? "#888";
  let colorB: string | null = null;
  if (isDuotone) {
    colorB = palette[(colorIdx + Math.floor(n / 2)) % n] ?? "#888";
  }
  return { colorA, colorB, dash: DASH_PATTERNS[dashIdx] ?? "", gradientId: isDuotone ? `fleet-grad-${index}` : null };
}

interface Point { t: string; v: number; }
type FleetMetricData = Record<string, Point[]>;
type Metric = "cpu" | "mem" | "disk";

const METRIC_LABELS: Record<Metric, string> = { cpu: "CPU", mem: "MEM", disk: "DISK" };
const METRIC_AXIS_LABELS: Record<Metric, string> = { cpu: "CPU %", mem: "Memory %", disk: "Disk %" };

interface TimePreset { label: string; hours: number; }
const TIME_PRESETS: TimePreset[] = [
    { label: "1h", hours: 1 },
    { label: "6h", hours: 6 },
    { label: "24h", hours: 24 },
];

function buildTimeParams(hours: number): { start: string; end: string } {
    const end = new Date();
    const start = new Date(end.getTime() - hours * 60 * 60 * 1000);
    return { start: start.toISOString(), end: end.toISOString() };
}

function formatAxisTime(iso: string, rangeHours: number): string {
    const d = new Date(iso);
    if (rangeHours <= 6) return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    return d.toLocaleTimeString([], {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
    });
}

function displayName(agentId: string, agentNames?: Record<string, string>): string {
    if (agentNames?.[agentId]) return agentNames[agentId];
    return agentId.length > 8 ? agentId.slice(0, 8) + "…" : agentId;
}

function cacheKey(metric: Metric, hours: number): string {
    return `${metric}:${hours}`;
}

// Duotone swatch for legend/tooltip
function LineSwatch({ style, width = 12, height = 4 }: { style: LineStyle; width?: number; height?: number }) {
  if (style.colorB) {
    const seg = 3;
    const count = Math.ceil(width / seg);
    return (
      <svg width={width} height={height} style={{ flexShrink: 0 }}>
        {Array.from({ length: count }).map((_, i) => (
          <line key={i} x1={i * seg} y1={height / 2} x2={Math.min((i + 1) * seg, width)} y2={height / 2}
            stroke={i % 2 === 0 ? style.colorA : style.colorB!} strokeWidth={1.2} />
        ))}
      </svg>
    );
  }
  return (
    <svg width={width} height={height} style={{ flexShrink: 0 }}>
      <line x1={0} y1={height / 2} x2={width} y2={height / 2}
        stroke={style.colorA} strokeWidth={1.2} strokeDasharray={style.dash} />
    </svg>
  );
}

interface FleetChartProps {
    apiBase?: string;
    agentNames?: Record<string, string>;
}

export function FleetChart({ apiBase = "", agentNames}: FleetChartProps) {
    const [cache, setCache] = useState<Record<string, FleetMetricData>>({});
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [metric, setMetric] = useState<Metric>("cpu");
    const [hours, setHours] = useState(1);
    const [visible, setVisible] = useState<Record<string, boolean>>({});
    const [themeName, setThemeName] = useState<ThemeName>(getThemeName);
    const [hoverIdx, setHoverIdx] = useState<number | null>(null);
    const [hoveredAgent, setHoveredAgent] = useState<string | null>(null);

    const svgRef = useRef<SVGSVGElement>(null);
    const activeKey = cacheKey(metric, hours);
    const data: FleetMetricData | null = cache[activeKey] ?? null;
    const fetchingRef = useRef(false);

    // Track theme changes via CSS variable mutations
    useEffect(() => {
        const observer = new MutationObserver(() => setThemeName(getThemeName()));
        observer.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ["style"],
        });
        const onStorage = () => setThemeName(getThemeName());
        window.addEventListener("storage", onStorage);
        return () => {
            observer.disconnect();
            window.removeEventListener("storage", onStorage);
        };
    }, []);

    // Fetch single metric
    const fetchMetric = useCallback(async (m: Metric, h: number) => {
        if (fetchingRef.current) return;
        fetchingRef.current = true;
        setLoading(true);
        setError(null);
        const { start, end } = buildTimeParams(h);
        try {
            const res = await fetch(
                `${apiBase}/api/v1/overview/fleet/chart?metric=${m}&start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`,
                { credentials: "include" }
            );
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            const json: FleetMetricData = await res.json();

            const key = cacheKey(m, h);
            setCache((prev) => ({ ...prev, [key]: json }));

            setVisible((prev) => {
                const next = { ...prev };
                for (const id of Object.keys(json)) {
                    if (!(id in next)) next[id] = true;
                }
                return next;
            });
        } catch (e: unknown) {
            setError(e instanceof Error ? e.message : "fetch failed");
        } finally {
            setLoading(false);
            fetchingRef.current = false;
        }
    }, [apiBase]);

    useEffect(() => {
        if (!cache[activeKey]) {
            fetchMetric(metric, hours);
        }
    }, [activeKey, cache, fetchMetric, metric, hours]);

    useEffect(() => {
        const iv = setInterval(() => {
            setCache({});
            fetchMetric(metric, hours);
        }, 30_000);
        return () => clearInterval(iv);
    }, [fetchMetric, metric, hours]);

    const palette = useMemo(
        () => THEME_PALETTES[themeName] ?? THEME_PALETTES.midnight,
        [themeName]
    );

    const agentIds = useMemo(() => {
        const ids = new Set<string>();
        for (const dataset of Object.values(cache)) {
            for (const id of Object.keys(dataset)) ids.add(id);
        }
        return Array.from(ids).sort();
    }, [cache]);

    const agentStyleMap = useMemo(() => {
      const m: Record<string, LineStyle> = {};
      agentIds.forEach((id, i) => { m[id] = getLineStyle(i, palette); });
      return m;
    }, [agentIds, palette]);

    // --- SVG layout ---
    const W = 900;
    const H = 240;
    const PAD = { top: 12, right: 12, bottom: 28, left: 36 };
    const plotW = W - PAD.left - PAD.right;
    const plotH = H - PAD.top - PAD.bottom;

    // Build lines
    const { lines, xMin, xMax } = useMemo(() => {
        if (!data) {
            const { start, end } = buildTimeParams(hours);
            return {
                lines: [] as { id: string; pts: { t: number; v: number }[] }[],
                xMin: new Date(start).getTime(),
                xMax: new Date(end).getTime(),
            };
        }

        const result : { id: string; pts: { t: number; v: number }[] }[] = [];
        let mn = Infinity;
        let mx = -Infinity;

        for (const id of agentIds) {
            if (!visible[id]) continue;
            const series = data[id];
            if (!series || series.length === 0) continue;
            const pts = series.map((p) => {
                const ts = new Date(p.t).getTime();
                if (ts < mn) mn = ts;
                if (ts > mx) mx = ts;
                return { t: ts, v: p.v };
            });
            result.push({ id, pts });
        }

        if (mn === Infinity) {
            const { start, end } = buildTimeParams(hours);
            mn = new Date(start).getTime();
            mx = new Date(end).getTime();
        }

        return { lines: result, xMin: mn, xMax: mx };
    }, [data, agentIds, visible, hours]);

    const yMax = 100;

    const toX = useCallback((t: number) => 
        PAD.left + ((t - xMin) / (xMax - xMin || 1)) * plotW,
        [xMin, xMax, plotW]);
    const toY = useCallback((v: number) =>
        PAD.top + plotH - (v / yMax) * plotH,
        [plotH]);

    // Hover
    const handleMouseMove = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
        const svg = svgRef.current;
        if (!svg) return;
        const rect = svg.getBoundingClientRect();
        const scaleX = W / rect.width;
        const x = (e.clientX - rect.left) * scaleX;
        if (x < PAD.left || x > W - PAD.right) {
            setHoverIdx(null);
            return;
        }
        setHoverIdx(Math.round(x));
    }, []);
    const handleMouseLeave = useCallback(() => setHoverIdx(null), []);

    const tooltip = useMemo(() => {
        if (hoverIdx === null || lines.length === 0) return null;
        const hoverT = xMin + ((hoverIdx - PAD.left) / plotW) * (xMax - xMin);

        const entries: { id: string; style: LineStyle; value: number; cx: number; cy: number }[] = [];
        for (const line of lines) {
            let closest = line.pts[0]!;
            let bestDist = Math.abs(closest.t - hoverT);
            for (const p of line.pts) {
                const d = Math.abs(p.t - hoverT);
                if (d < bestDist) {
                    bestDist = d;
                    closest = p;
                }
            }
            entries.push({
                id: line.id,
                style: agentStyleMap[line.id] ?? { colorA: "#888", colorB: null, dash: "", gradientId: null },
                value: closest.v,
                cx: toX(closest.t),
                cy: toY(closest.v),
            });
        }
        entries.sort((a, b) => b.value - a.value)

        return {
            x: hoverIdx,
            time: new Date(hoverT).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
            }),
            entries,
        };
    }, [hoverIdx, lines, xMin, xMax, plotW, agentStyleMap, toX, toY]);

    // Axis ticks
    const xTicks = useMemo(() => {
        const count = 6;
        const ticks: { x: number; label: string }[] = [];
        for (let i = 0; i <= count; i++) {
            const t = xMin + (i / count) * (xMax - xMin);
            ticks.push({
                x: toX(t),
                label: formatAxisTime(new Date(t).toISOString(), hours),
            });
        }
        return ticks;
    }, [xMin, xMax, toX, hours]);
    
    const yTicks = useMemo(() => {
        return [0, 25, 50, 75, 100].map((v) => ({ y: toY(v), label: `${v}% `}));
    }, [toY]);

    // Visibility
    const toggleAgent = (id: string) =>
        setVisible((prev) => ({ ...prev, [id]: !prev[id] }));
    const setAll = (val: boolean) =>
        setVisible((prev) => {
            const next = { ...prev };
            for (const k of Object.keys(next)) next[k] = val;
            return next;
        });

    // Collect gradient definitions for visible duotone agents
    const gradientDefs = useMemo(() => {
      const defs: { id: string; colorA: string; colorB: string }[] = [];
      for (const id of agentIds) {
        const s = agentStyleMap[id];
        if (s?.gradientId && s.colorB) {
          defs.push({ id: s.gradientId, colorA: s.colorA, colorB: s.colorB });
        }
      }
      return defs;
    }, [agentIds, agentStyleMap]);

    // Button styles
    const btnStyle = (active: boolean): React.CSSProperties => ({
      padding: "3px 10px", fontSize: 10, fontFamily: themeVars.font,
      color: active ? themeVars.text : themeVars.textMuted,
      background: active ? themeVars.accentDim : "transparent",
      border: `1px solid ${active ? themeVars.accent : themeVars.border}`,
      cursor: "pointer", textTransform: "uppercase", letterSpacing: "0.03em",
    });

  return (
    <div style={{ background: themeVars.surface, border: `1px solid ${themeVars.border}`, padding: 16 }}>
      {/* Toolbar */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10, flexWrap: "wrap", gap: 8 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span style={{ fontSize: 10, fontFamily: themeVars.font, color: themeVars.text, fontWeight: 600, letterSpacing: "0.02em", textTransform: "uppercase" }}>
            Fleet Chart
          </span>
          <div style={{ display: "flex", gap: 2 }}>
            {TIME_PRESETS.map((p) => (
              <button key={p.hours} onClick={() => setHours(p.hours)} style={btnStyle(p.hours === hours)}>{p.label}</button>
            ))}
          </div>
        </div>
        <div style={{ display: "flex", gap: 2 }}>
          {(["cpu", "mem", "disk"] as Metric[]).map((m) => (
            <button key={m} onClick={() => setMetric(m)} style={btnStyle(m === metric)}>{METRIC_LABELS[m]}</button>
          ))}
        </div>
      </div>
 
      {error && (
        <div style={{ color: themeVars.danger, fontSize: 9, fontFamily: themeVars.font, marginBottom: 6 }}>Error: {error}</div>
      )}
 
      <div style={{ display: "flex", gap: 10, alignItems: "flex-start" }}>
        <div style={{ flex: 1, minWidth: 0, opacity: loading ? 0.5 : 1, transition: "opacity 0.2s ease", position: "relative" }}>
          <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%" style={{ display: "block" }}
            onMouseMove={handleMouseMove} onMouseLeave={handleMouseLeave}>
            <defs>
              {gradientDefs.map((g) => (
                <linearGradient key={g.id} id={g.id} gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="8" y2="0" spreadMethod="repeat">
                  <stop offset="0%" stopColor={g.colorA} /><stop offset="50%" stopColor={g.colorA} />
                  <stop offset="50%" stopColor={g.colorB} /><stop offset="100%" stopColor={g.colorB} />
                </linearGradient>
              ))}
            </defs>
 
            {yTicks.map((t) => (
              <line key={t.label} x1={PAD.left} x2={W - PAD.right} y1={t.y} y2={t.y}
                stroke={themeVars.border} strokeWidth={0.5} strokeDasharray="4 3" />
            ))}
            {yTicks.map((t) => (
              <text key={`yl-${t.label}`} x={PAD.left - 4} y={t.y} textAnchor="end" dominantBaseline="middle"
                fontSize={8} fill={themeVars.textDim} fontFamily="var(--font-sans)">{t.label}</text>
            ))}
            {xTicks.map((t, i) => (
              <text key={`xl-${i}`} x={t.x} y={H - PAD.bottom + 12} textAnchor="middle"
                fontSize={7} fill={themeVars.textDim} fontFamily="var(--font-mono)">{t.label}</text>
            ))}
            <text x={8} y={PAD.top + plotH / 2} textAnchor="middle" dominantBaseline="middle"
              fontSize={8} fill={themeVars.textDim} fontFamily="var(--font-sans)"
              transform={`rotate(-90, 8, ${PAD.top + plotH / 2})`}>{METRIC_AXIS_LABELS[metric]}</text>
 
            {lines.map((line) => {
              if (line.pts.length === 0) return null;
              const s = agentStyleMap[line.id] ?? { colorA: "#888", colorB: null, dash: "", gradientId: null };
              const d = line.pts.map((p, i) => `${i === 0 ? "M" : "L"}${toX(p.t)},${toY(p.v)}`).join(" ");
              return <path key={line.id} d={d} fill="none" stroke={s.gradientId ? `url(#${s.gradientId})` : s.colorA}
                strokeWidth={hoveredAgent === line.id ? 2.4 : 1.2} strokeDasharray={s.dash} strokeLinejoin="round" strokeLinecap="round"
                opacity={hoveredAgent === null || hoveredAgent === line.id ? 1 : 0.15} />;
            })}
 
            {tooltip && <line x1={tooltip.x} x2={tooltip.x} y1={PAD.top} y2={PAD.top + plotH}
              stroke={themeVars.textDim} strokeWidth={0.5} strokeDasharray="3 3" opacity={0.5} />}
            {tooltip?.entries.map((e) => (
              <circle key={`dot-${e.id}`} cx={e.cx} cy={e.cy} r={2} fill={e.style.colorA} stroke={themeVars.bg} strokeWidth={0.8} />
            ))}
          </svg>
 
          {tooltip && tooltip.entries.length > 0 && (
            <div style={{
              position: "absolute", bottom: "1rem", left: `${(tooltip.x / W) * 100}%`, transform: "translateX(-50%)",
              background: themeVars.bg, border: `1px solid ${themeVars.border}`, borderRadius: 3, padding: "3px 6px",
              fontSize: 14, fontFamily: themeVars.font, color: themeVars.text, whiteSpace: "nowrap",
              boxShadow: "0 2px 8px rgba(0,0,0,0.3)", zIndex: 10, pointerEvents: "none",
            }}>
              <div style={{ fontWeight: 600, marginBottom: 1, color: themeVars.textMuted, fontSize: 12 }}>{tooltip.time}</div>
              {tooltip.entries.slice(0, 20).map((e) => (
                <div key={e.id} style={{ display: "flex", alignItems: "center", gap: 3, lineHeight: 1.5 }}>
                  <LineSwatch style={e.style} />
                  <span style={{ color: themeVars.textMuted }}>{displayName(e.id, agentNames)}</span>
                  <span style={{ fontWeight: 600, marginLeft: "auto", paddingLeft: 4 }}>{e.value.toFixed(1)}%</span>
                </div>
              ))}
            </div>
          )}
        </div>
 
        {/* Legend */}
        <div style={{ width: 120, flexShrink: 0, maxHeight: H, overflowY: "auto" }}>
          <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 3, fontSize: 10, fontFamily: themeVars.font }}>
            <button onClick={() => setAll(true)} style={{ background: "none", border: "none", color: themeVars.accent, cursor: "pointer", fontSize: 12, fontFamily: themeVars.font, padding: 0 }}>All</button>
            <button onClick={() => setAll(false)} style={{ background: "none", border: "none", color: themeVars.accent, cursor: "pointer", fontSize: 12, fontFamily: themeVars.font, padding: 0 }}>None</button>
          </div>
          {agentIds.map((id) => {
            const s = agentStyleMap[id] ?? { colorA: "#888", colorB: null, dash: "", gradientId: null };
            return (
              <div
                key={id}
                onMouseEnter={() => setHoveredAgent(id)}
                onMouseLeave={() => setHoveredAgent(null)}
                style={{
                  display: "flex", alignItems: "center", gap: 3, fontSize: 8, fontFamily: themeVars.font,
                  cursor: "pointer", padding: "1px 0",
                  color: visible[id] ? themeVars.text : themeVars.textDim, opacity: visible[id] ? 1 : 0.4,
                }}
              >
                <input
                  type="checkbox"
                  checked={!!visible[id]}
                  onChange={(e) => {
                    e.stopPropagation();
                    toggleAgent(id);
                  }}
                  onMouseEnter={(e) => e.stopPropagation()}
                  onMouseLeave={(e) => e.stopPropagation()}
                  style={{ accentColor: s.colorA, margin: 0, width: 8, height: 8 }}
                />
                <span
                  onClick={() => {
                    setVisible((prev) => {
                      const onlyThisVisible = Object.entries(prev).every(
                        ([k, v]) => k === id ? v : !v
                      );
                      if (onlyThisVisible) {
                        const next: Record<string, boolean> = {};
                        for (const k of Object.keys(prev)) next[k] = true;
                        return next;
                      }
                      const next: Record<string, boolean> = {};
                      for (const k of Object.keys(prev)) next[k] = k === id;
                      return next;
                    });
                  }}
                  style={{ display: "flex", alignItems: "center", gap: 3, overflow: "hidden" }}
                >
                  <LineSwatch style={s} width={20} height={4} />
                  <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {displayName(id, agentNames)}
                  </span>
                </span>
              </div>
            );
          })}
          {agentIds.length === 0 && !loading && (
            <div style={{ fontSize: 8, fontFamily: themeVars.font, color: themeVars.textDim, fontStyle: "italic" }}>No agents reporting</div>
          )}
        </div>
      </div>
    </div>
  );
}