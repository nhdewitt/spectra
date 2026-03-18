import { useCallback, useMemo } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { useMetric } from "../hooks/useMetric";
import { MetricChart, type SeriesDef } from "./MetricChart";
import { StatBlock } from "./ui";
import { themeVars } from "../theme";
import type { PiMetric, RangeSelection } from "../types";

const GPU_TEMP_SERIES: SeriesDef[] = [
    { key: "gpu_temp", label: "GPU", area: true },
];

const FREQ_SERIES: SeriesDef[] = [
    { key: "arm_freq_mhz", label: "ARM" },
    { key: "core_freq_mhz", label: "Core" },
    { key: "gpu_freq_mhz", label: "GPU" },
];

const VOLTAGE_SERIES: SeriesDef[] = [
    { key: "core_volts", label: "Core" },
    { key: "sdram_c_volts", label: "SDRAM C" },
    { key: "sdram_i_volts", label: "SDRAM I" },
    { key: "sdram_p_volts", label: "SDRAM P" },
];

const GPU_MEM_SERIES: SeriesDef[] = [
    { key: "gpu_mem_used", label: "Used", area: true },
    { key: "gpu_mem_total", label: "Total" },
];

const voltFormatter = (v: number) => `${v.toFixed(3)} V`;
const memFormatter = (v: number) => formatBytes(v);

interface PiPanelsProps {
    agentId: string;
    rangeSel: RangeSelection;
}

function pollInterval(sel: RangeSelection): number {
    if (sel.type === "custom") return 0;
    return ["5m", "15m", "1h"].includes(sel.range) ? 30_000 : 0;
}

function rangeLabel(sel: RangeSelection): string {
    if (sel.type === "quick") return `last ${sel.range}`;
    return "selected range";
}

interface ThrottleCounts {
    throttled: number;
    underVoltage: number;
    freqCapped: number;
    softTempLimit: number;
    total: number;
}

function countThrottleEvents(data: PiMetric[]): ThrottleCounts {
    let throttled = 0;
    let underVoltage = 0;
    let freqCapped = 0;
    let softTempLimit = 0;
    let total = 0;

    for (const d of data) {
        if (d.throttled) throttled++;
        if (d.under_voltage) underVoltage++;
        if (d.freq_capped) freqCapped++;
        if (d.soft_temp_limit_occurred) softTempLimit++;
        total++;
    }

    return {
        throttled,
        underVoltage,
        freqCapped,
        softTempLimit,
        total,
    };
}

function ThrottleSummary({
    data,
    rangeSel,
}: {
    data: PiMetric[];
    rangeSel: RangeSelection;
}) {
    const counts = useMemo(() => countThrottleEvents(data), [data]);

    const latest = data.length > 0 ? data[data.length - 1] : null;

    return (
        <div style={{ marginBottom: 24 }}>
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 12,
                    fontWeight: 600,
                    color: themeVars.textMuted,
                    textTransform: "uppercase",
                    letterSpacing: "0.04em",
                    marginBottom: 12,
                }}
            >
                Throttle Status
            </div>

            {/* Current State */}
            <div
                style={{
                    display: "flex",
                    gap: 24,
                    marginBottom: 12,
                    flexWrap: "wrap",
                    background: themeVars.surface,
                    border: `1px solid ${themeVars.border}`,
                    padding: "12px 16px",
                }}
            >
                <StatusIndicator
                    label="Throttled"
                    active={latest?.throttled ?? false}
                />
                <StatusIndicator
                    label="Undervoltage"
                    active={latest?.throttled ?? false}
                />
                <StatusIndicator
                    label="Freq Capped"
                    active={latest?.freq_capped ?? false}
                />
                <StatusIndicator
                    label="Soft Temp Limit"
                    active={latest?.soft_temp_limit_occurred ?? false}
                />
            </div>

            {/* Historical Count */}
            {counts.total > 0 ? (
                <div
                    style={{
                        fontFamily: themeVars.font,
                        fontSize: 11,
                        color: themeVars.warn,
                        padding: "8px 12px",
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                >
                    In {rangeLabel(rangeSel)}:{" "}
                    {[
                        counts.throttled > 0 && `${counts.throttled} throttle`,
                        counts.underVoltage > 0 && `${counts.underVoltage} undervoltage`,
                        counts.freqCapped > 0 && `${counts.freqCapped} freq cap`,
                        counts.softTempLimit > 0 && `${counts.softTempLimit} temp limit`,
                    ]
                        .filter(Boolean)
                        .join(", ")}{" "}
                    event{counts.total !== 1 ? "s" : ""}
                </div>
            ) : (
                <div
                    style={{
                        fontFamily: themeVars.font,
                        fontSize: 11,
                        color: themeVars.ok,
                        padding: "8px 12px",
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                >
                    No throttle events in {rangeLabel(rangeSel)}.
                </div>
            )}
        </div>
    );
}

function StatusIndicator({
    label,
    active,
}: {
    label: string;
    active: boolean;
}) {
    return (
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <span
                style={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    background: active ? themeVars.danger : themeVars.ok,
                    flexShrink: 0,
                }}
            />
            <span
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 11,
                    color: active ? themeVars.danger : themeVars.textMuted,
                }}
            >
                {label}: {active ? "Yes" : "No"}
            </span>
        </div>
    );
}

export function PiPanels({ agentId, rangeSel }: PiPanelsProps) {
    const fetcher = useCallback(
        (sel: RangeSelection, signal?: AbortSignal) =>
            api.agentPi(agentId, sel, { signal }),
        [agentId]
    );

    const { data, loading, error } = useMetric(
        fetcher,
        rangeSel,
        pollInterval(rangeSel)
    );

    const freqData = useMemo(
        () => data.map((d) => ({
            ...d,
            arm_freq_mhz: d.arm_freq_hz / 1_000_000,
            core_freq_mhz: d.core_freq_hz / 1_000_000,
            gpu_freq_mhz: d.gpu_freq_hz / 1_000_000,
        })),
        [data]
    );

    if (!loading && !error && data.length === 0) return null;

    return (
        <div>
            {/* Throttle summary */}
             {!loading && data.length > 0 && (
                <ThrottleSummary data={data} rangeSel={rangeSel} />
             )}

             {/* Charts */}
             <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <MetricChart
                    title="GPU Temperature"
                    data={data}
                    loading={loading}
                    error={error}
                    unit="°C"
                    series={GPU_TEMP_SERIES}
                    rangeSel={rangeSel}
                />
                <MetricChart
                    title="Frequencies"
                    data={freqData}
                    loading={loading}
                    error={error}
                    unit="MHz"
                    series={FREQ_SERIES}
                    rangeSel={rangeSel}
                />
             </div>
             <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <MetricChart
                    title="Voltages"
                    data={data}
                    loading={loading}
                    error={error}
                    formatter={voltFormatter}
                    series={VOLTAGE_SERIES}
                    rangeSel={rangeSel}
                />
                <MetricChart
                    title="GPU Memory"
                    data={data}
                    loading={loading}
                    error={error}
                    formatter={memFormatter}
                    series={GPU_MEM_SERIES}
                    rangeSel={rangeSel}
                />
             </div>
        </div>
    );
}