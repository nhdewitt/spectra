import { useCallback, useMemo, useState } from "react";
import type { ReactElement, ReactNode } from "react";
import { api } from "../api";
import { formatBytes, formatNetworkRate } from "../utils";
import { useMetric, withChartMeta } from "../hooks/useMetric";
import { useThresholds } from "../ThresholdsContext";
import { MetricChart, type SeriesDef } from "./MetricChart";
import { MetricSelector, StatBlock, LoadingSpinner } from "./ui";
import { AnomalyBanner } from "./AnomalyBanner";
import { themeVars } from "../theme";
import type { Anomaly } from "../anomaly";
import {
	cpuAnomalies,
	loadAnomalies,
	memoryAnomalies,
	diskAnomalies,
	diskIOAnomalies,
	networkAnomalies,
	temperatureAnomalies,
} from "../metricAnomalies";
import type {
	CPUMetric,
	DiskMetric,
	DiskIOMetric,
	NetworkMetric,
	RangeSelection,
} from "../types";

/** Drill-in targets. Each corresponds to one clickable panel on MetricsTab. */
export type MetricFamily =
	| "cpu"
	| "load"
	| "memory"
	| "disk"
	| "diskio"
	| "network"
	| "temperature"
	| "wifi";

export const FAMILY_TITLES: Record<MetricFamily, string> = {
	cpu: "CPU",
	load: "Load Average",
	memory: "Memory",
	disk: "Disk Usage",
	diskio: "Disk I/O",
	network: "Network",
	temperature: "Temperature",
	wifi: "WiFi Signal",
};

interface DetailProps {
	agentId: string;
	rangeSel: RangeSelection;
	cores: number;
}

function pollInterval(sel: RangeSelection): number {
	if (sel.type === "custom") return 0;
	return ["5m", "15m", "1h"].includes(sel.range) ? 30_000 : 0;
}

function useAgentMetric<T extends { time: string }>(
	agentId: string,
	fn: (id: string, sel?: RangeSelection, opts?: { signal?: AbortSignal }) => Promise<T[]>,
	rangeSel: RangeSelection
) {
	const fetcher = useCallback(
		(sel: RangeSelection, signal?: AbortSignal) => fn(agentId, sel, { signal }),
		[agentId, fn]
	);
	return useMetric(fetcher, rangeSel, pollInterval(rangeSel));
}

const perSec = (v: number) => `${formatBytes(v)}/s`;
const rate = (v: number) => v.toFixed(2);

/** Static facts about the selected entity. */
function FactRow({ children }: { children: ReactNode }) {
	return (
		<div style={{ display: "flex", gap: 24, flexWrap: "wrap", marginBottom: 16 }}>
			{children}
		</div>
	);
}

function Section({ title, children }: { title: string; children: ReactNode }) {
    return (
        <div style={{ marginBottom: 8 }}>
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 10,
                    color: themeVars.textDim,
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    marginTop: 20,
                    marginBottom: 8,
                }}
            >
                {title}
            </div>
            {children}
        </div>
    );
}

const CPU_SERIES: SeriesDef[] = [
	{ key: "usage", label: "Usage", area: true },
	{ key: "iowait", label: "IO Wait" },
];

function CPUDetail({ agentId, rangeSel, cores }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentCPU, rangeSel);
	const thresholds = useThresholds();

	// core_usages arrives as an array per sample. recharts needs one key per
	// line, so widen each row into core_0..core_n.
	const perCore = useMemo(() => {
		return data.map((d: CPUMetric) => {
			const row: Record<string, unknown> = { time: d.time };
			if (Array.isArray(d.core_usages)) {
				d.core_usages.forEach((v, i) => {
					row[`core_${i}`] = v;
				});
			}
			return withChartMeta(d, row as unknown as { time: string });
		});
	}, [data]);

	const coreCount = useMemo(() => {
		let max = 0;
		for (const d of data as CPUMetric[]) {
			if (Array.isArray(d.core_usages)) max = Math.max(max, d.core_usages.length);
		}
		return max;
	}, [data]);

	const coreSeries = useMemo<SeriesDef[]>(
		() => Array.from({ length: coreCount }, (_, i) => ({ key: `core_${i}`, label: `Core ${i}` })),
		[coreCount]
	);

	const anomalies = useMemo<Anomaly[]>(() => cpuAnomalies(data, thresholds), [data, thresholds]);

    return (
        <div>
            <AnomalyBanner anomalies={anomalies} />
 
            <MetricChart
                title="Usage and IO Wait"
                data={data}
                loading={loading}
                error={error}
                unit="%"
                yDomain={[0, 100]}
                series={CPU_SERIES}
                rangeSel={rangeSel}
                refLines={[
                    { y: thresholds.cpu_warn, label: "warn", color: themeVars.warn },
                    { y: thresholds.cpu_crit, label: "crit", color: themeVars.danger },
                ]}
            />
 
            {coreCount === 0 && data.length > 0 && (
                <Section title="Per-core usage">
                    <div
                        style={{
                            padding: 24,
                            border: `1px solid ${themeVars.border}`,
                            background: themeVars.surface,
                            color: themeVars.textDim,
                            fontFamily: themeVars.font,
                            fontSize: 12,
                            textAlign: "center",
                        }}
                    >
                        Per-core data is only stored for ranges of 1 hour or less.
                        <div style={{ marginTop: 6, fontSize: 11 }}>
                            Longer ranges are served from time_bucket aggregates, and a
                            per-core array cannot be averaged across a bucket
                            (GetCPUBucketed returns NULL for it). Core Imbalance is not
                            reported at this range either.
                        </div>
                    </div>
                </Section>
            )}
 
            {coreCount > 0 && (
                <Section title={`Per-core usage (${coreCount} of ${cores} reported)`}>
                    <MetricChart
                        title="Per-Core"
                        data={perCore as { time: string }[]}
                        loading={loading}
                        error={error}
                        unit="%"
                        yDomain={[0, 100]}
                        series={coreSeries}
                        rangeSel={rangeSel}
                        height={260}
                    />
                </Section>
            )}
        </div>
    );
}

const LOAD_SERIES: SeriesDef[] = [
	{ key: "load_1m", label: "1m" },
	{ key: "load_5m", label: "5m" },
	{ key: "load_15m", label: "15m" },
];

function LoadDetail({ agentId, rangeSel, cores }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentCPU, rangeSel);

	const anomalies = useMemo<Anomaly[]>(() => loadAnomalies(data, cores), [data, cores]);

    return (
        <div>
            <AnomalyBanner anomalies={anomalies} />
            <MetricChart
                title="Load Average"
                data={data}
                loading={loading}
                error={error}
                series={LOAD_SERIES}
                rangeSel={rangeSel}
                height={320}
                refLines={[{ y: cores, label: `${cores} cores`, color: themeVars.textDim }]}
            />
        </div>
    );
}

const MEM_PERCENT: SeriesDef[] = [
	{ key: "ram_percent", label: "RAM", area: true },
	{ key: "swap_percent", label: "Swap" },
];

const MEM_ABSOLUTE: SeriesDef[] = [
	{ key: "ram_used", label: "RAM Used", area: true },
	{ key: "ram_available", label: "Available" },
	{ key: "swap_used", label: "Swap Used" },
];

const MEM_PAGING: SeriesDef[] = [
    { key: "swap_in_pages", label: "Swap In" },
    { key: "swap_out_pages", label: "Swap Out" },
];

const pages = (v: number) => v.toFixed(0);

function MemoryDetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentMemory, rangeSel);
	const thresholds = useThresholds();
	const latest = data.length > 0 ? data[data.length - 1] : null;

	const anomalies = useMemo<Anomaly[]>(() => memoryAnomalies(data, thresholds), [data, thresholds]);

    const hasPaging = useMemo(
        () => data.some((d) => d.has_paging ?? d.swap_in_pages != null),
        [data]
    );

    return (
        <div>
            <AnomalyBanner anomalies={anomalies} />

            {latest && (
                <FactRow>
                    <StatBlock label="RAM Total" value={formatBytes(latest.ram_total)} />
                    <StatBlock label="Swap Total" value={formatBytes(latest.swap_total)} />
                </FactRow>
            )}

            {hasPaging && (
                <MetricChart
                    title="Utilization"
                    data={data}
                    loading={loading}
                    error={error}
                    unit="%"
                    yDomain={[0, 100]}
                    series={MEM_PERCENT}
                    rangeSel={rangeSel}
                    refLines={[
                        { y: thresholds.mem_warn, label: "warn", color: themeVars.warn },
                        { y: thresholds.mem_crit, label: "crit", color: themeVars.danger },
                    ]}
                />
            )}

            <Section title="Absolute">
                <MetricChart
                    title="Bytes"
                    data={data}
                    loading={loading}
                    error={error}
                    formatter={formatBytes}
                    series={MEM_ABSOLUTE}
                    rangeSel={rangeSel}
                />
            </Section>

            <Section title="Swap paging">
                {hasPaging ? (
                    <MetricChart
                        title="Pages In / Out per second"
                        data={data}
                        loading={loading}
                        error={error}
                        formatter={pages}
                        series={MEM_PAGING}
                        rangeSel={rangeSel}
                    />
                ) : (
                    !loading && (
                        <div
                            style={{
                                padding: 24,
                                border: `1px solid ${themeVars.border}`,
                                background: themeVars.surface,
                                color: themeVars.textDim,
                                fontFamily: themeVars.font,
                                fontSize: 12,
                                textAlign: "center",
                            }}
                        >
                            Swap paging is not measured on this agent.
                            <div style={{ marginTop: 6, fontSize: 11 }}>
                                Darwin exposes it only through host_statistics64, and Windows
                                has no pagefile-specific counter. Rows collected before
                                migration 022 also predate it.
                            </div>
                        </div>
                    )
                )}
            </Section>
        </div>
    );
}

function DiskDetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentDisk, rangeSel);
	const thresholds = useThresholds();

	const mounts = useMemo(
		() => [...new Set(data.map((d: DiskMetric) => d.mountpoint))],
		[data]
	);
	const [selected, setSelected] = useState("");
	const active = mounts.includes(selected) ? selected : mounts[0] ?? "";

	const rows = useMemo(
		() => data.filter((d: DiskMetric) => d.mountpoint === active),
		[data, active]
	);
	const latest = rows.length > 0 ? rows[rows.length - 1] : null;

	const anomalies = useMemo<Anomaly[]>(() => diskAnomalies(rows, thresholds), [rows, thresholds]);

    return (
        <div>
            <MetricSelector label="Mount" options={mounts} value={active} onChange={setSelected} />
            <AnomalyBanner anomalies={anomalies} />
 
            {latest && (
                <FactRow>
                    <StatBlock label="Device" value={latest.device} />
                    <StatBlock label="Filesystem" value={latest.filesystem} />
                    <StatBlock label="Type" value={latest.disk_type} />
                    <StatBlock label="Total" value={formatBytes(latest.total_bytes)} />
                    <StatBlock label="Used" value={formatBytes(latest.used_bytes)} />
                    <StatBlock label="Free" value={formatBytes(latest.free_bytes)} />
                    <StatBlock label="Inodes Used" value={`${latest.inodes_used} / ${latest.inodes_total}`} />
                </FactRow>
            )}
 
            <MetricChart
                title="Space Used"
                data={rows}
                loading={loading}
                error={error}
                unit="%"
                yDomain={[0, 100]}
                series={[{ key: "used_percent", label: active || "Used", area: true }]}
                rangeSel={rangeSel}
                refLines={[
                    { y: thresholds.disk_warn, label: "warn", color: themeVars.warn },
                    { y: thresholds.disk_crit, label: "crit", color: themeVars.danger },
                ]}
            />
 
            <Section title="Inodes">
                <MetricChart
                    title="Inodes Used"
                    data={rows}
                    loading={loading}
                    error={error}
                    unit="%"
                    yDomain={[0, 100]}
                    series={[{ key: "inodes_percent", label: "Inodes", area: true }]}
                    rangeSel={rangeSel}
                    refLines={[
                        { y: thresholds.disk_warn, label: "warn", color: themeVars.warn },
                        { y: thresholds.disk_crit, label: "crit", color: themeVars.danger },
                    ]}
                />
            </Section>
        </div>
    );
}

const IO_THROUGHPUT: SeriesDef[] = [
	{ key: "read_bytes", label: "Read" },
	{ key: "write_bytes", label: "Write" },
];

const IO_OPS: SeriesDef[] = [
	{ key: "read_ops", label: "Read Ops" },
	{ key: "write_ops", label: "Write Ops" },
];

const IO_LATENCY: SeriesDef[] = [
	{ key: "read_latency_ms", label: "Read Latency" },
	{ key: "write_latency_ms", label: "Write Latency" },
];

const IO_BUSY: SeriesDef[] = [
    { key: "read_busy_pct", label: "Read %util" },
    { key: "write_busy_pct", label: "Write %util" },
];

function DiskIODetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentDiskIO, rangeSel);

	const devices = useMemo(
		() => [...new Set(data.map((d: DiskIOMetric) => d.device))].filter(Boolean),
		[data]
	);
	const [selected, setSelected] = useState("");
	const active = devices.includes(selected) ? selected : devices[0] ?? "";
	const rows = useMemo(
		() => (active ? data.filter((d: DiskIOMetric) => d.device === active) : data),
		[data, active]
	);

	const anomalies = useMemo<Anomaly[]>(() => diskIOAnomalies(rows), [rows]);

    const hasIODetail = useMemo(
        () => rows.some((d) => d.has_io_detail ?? d.read_latency_ms != null),
        [rows]
    );

    return (
        <div>
            {devices.length > 0 && (
                <MetricSelector label="Device" options={devices} value={active} onChange={setSelected} />
            )}
            <AnomalyBanner anomalies={anomalies} />
 
            <MetricChart
                title="Throughput"
                data={rows}
                loading={loading}
                error={error}
                formatter={perSec}
                series={IO_THROUGHPUT}
                rangeSel={rangeSel}
            />
 
            <Section title="Operations">
                <MetricChart
                    title="IOPS"
                    data={rows}
                    loading={loading}
                    error={error}
                    formatter={rate}
                    series={IO_OPS}
                    rangeSel={rangeSel}
                />
            </Section>
 
            {hasIODetail && (
                <Section title="Utilization">
                    {/* Can exceed 100% on a device with a parallel queue: the
                        counter sums the service time of every in-flight request,
                        so 400% means roughly four requests in flight on average. */}
                    <MetricChart
                        title="Device Busy (%util)"
                        data={rows}
                        loading={loading}
                        error={error}
                        unit="%"
                        series={IO_BUSY}
                        rangeSel={rangeSel}
                    />
                </Section>
            )}
 
            <Section title="Latency and queue depth">
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                    <MetricChart
                        title="Latency (per operation)"
                        data={rows}
                        loading={loading}
                        error={error}
                        unit="ms"
                        series={IO_LATENCY}
                        rangeSel={rangeSel}
                    />
                    <MetricChart
                        title="Requests In Progress"
                        data={rows}
                        loading={loading}
                        error={error}
                        series={[{ key: "io_in_progress", label: "In Progress", area: true }]}
                        rangeSel={rangeSel}
                    />
                </div>
            </Section>
        </div>
    );
}

const NET_THROUGHPUT: SeriesDef[] = [
	{ key: "rx_bytes", label: "RX" },
	{ key: "tx_bytes", label: "TX" },
];

const NET_PACKETS: SeriesDef[] = [
	{ key: "rx_packets", label: "RX Packets" },
	{ key: "tx_packets", label: "TX Packets" },
];

const NET_ERRORS: SeriesDef[] = [
	{ key: "rx_errors", label: "RX Errors" },
	{ key: "tx_errors", label: "TX Errors" },
	{ key: "rx_drops", label: "RX Drops" },
	{ key: "tx_drops", label: "TX Drops" },
];

function NetworkDetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentNetwork, rangeSel);

	const ifaces = useMemo(() => {
		const traffic = new Map<string, number>();
		for (const d of data as NetworkMetric[]) {
			traffic.set(d.interface, (traffic.get(d.interface) ?? 0) + d.rx_bytes + d.tx_bytes);
		}
		return [...traffic.entries()].sort((a, b) => b[1] - a[1]).map(([name]) => name);
	}, [data]);

	const [selected, setSelected] = useState("");
	const active = ifaces.includes(selected) ? selected : ifaces[0] ?? "";
	const rows = useMemo(
		() => (active ? data.filter((d: NetworkMetric) => d.interface === active) : data),
		[data, active]
	);
	const latest = rows.length > 0 ? (rows[rows.length - 1] as NetworkMetric) : null;

	const anomalies = useMemo<Anomaly[]>(() => networkAnomalies(rows), [rows]);

    return (
        <div>
            {ifaces.length > 0 && (
                <MetricSelector label="Interface" options={ifaces} value={active} onChange={setSelected} />
            )}
            <AnomalyBanner anomalies={anomalies} />
 
            {latest && (
                <FactRow>
                    <StatBlock label="MAC" value={latest.mac || null} copyable />
                    <StatBlock label="MTU" value={latest.mtu ? String(latest.mtu) : null} />
                    <StatBlock label="Link Speed" value={formatNetworkRate(latest.speed)} />
                </FactRow>
            )}
 
            <MetricChart
                title="Throughput"
                data={rows}
                loading={loading}
                error={error}
                formatter={perSec}
                series={NET_THROUGHPUT}
                rangeSel={rangeSel}
            />
 
            <Section title="Packets">
                <MetricChart
                    title="Packet Rate"
                    data={rows}
                    loading={loading}
                    error={error}
                    formatter={rate}
                    series={NET_PACKETS}
                    rangeSel={rangeSel}
                />
            </Section>
 
            <Section title="Errors and drops (should be flat zero)">
                <MetricChart
                    title="Errors / Drops"
                    data={rows}
                    loading={loading}
                    error={error}
                    formatter={rate}
                    series={NET_ERRORS}
                    rangeSel={rangeSel}
                />
            </Section>
        </div>
    );
}

// Index signature covers both the sensor columns and the _ts/_gap metadata
type PivotedRow = { time: string; [sensor: string]: unknown };

function TemperatureDetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentTemperature, rangeSel);
	const thresholds = useThresholds();

	const sensors = useMemo(
		() => [...new Set(data.map((d) => d.sensor))].filter(Boolean) as string[],
		[data]
	);

	const pivoted = useMemo<PivotedRow[]>(() => {
		const byTime = new Map<string, PivotedRow>();
		for (const d of data) {
			if (!d.sensor) continue;
			let row = byTime.get(d.time);
			if (!row) {
				row = withChartMeta(d, { time: d.time } as PivotedRow);
				byTime.set(d.time, row);
			}
			row[d.sensor] = d.temperature;
		}
		return [...byTime.values()];
	}, [data]);

	const anomalies = useMemo<Anomaly[]>(
		() => temperatureAnomalies(pivoted, sensors, thresholds),
		[pivoted, sensors, thresholds]
	);

    return (
        <div>
            <AnomalyBanner anomalies={anomalies} />
            <MetricChart
                title="Temperature"
                data={pivoted}
                loading={loading}
                error={error}
                unit="°C"
                series={sensors.map((s) => ({ key: s, label: s }))}
                rangeSel={rangeSel}
                height={320}
                refLines={[
                    { y: thresholds.temp_warn, label: "warn", color: themeVars.warn },
                    { y: thresholds.temp_crit, label: "crit", color: themeVars.danger },
                ]}
            />
        </div>
    );
}

function WifiDetail({ agentId, rangeSel }: DetailProps) {
	const { data, loading, error } = useAgentMetric(agentId, api.agentWifi, rangeSel);

    return (
        <div>
            <MetricChart
                title="Signal and Noise"
                data={data}
                loading={loading}
                error={error}
                unit="dBm"
                series={[
                    { key: "signal_dbm", label: "Signal" },
                    { key: "noise_dbm", label: "Noise" },
                ]}
                rangeSel={rangeSel}
                height={320}
            />
        </div>
    );
}

const DETAILS: Record<MetricFamily, (p: DetailProps) => ReactElement> = {
	cpu: CPUDetail,
	load: LoadDetail,
	memory: MemoryDetail,
	disk: DiskDetail,
	diskio: DiskIODetail,
	network: NetworkDetail,
	temperature: TemperatureDetail,
	wifi: WifiDetail,
};

export function MetricDetail({
	family,
	onBack,
	...props
}: DetailProps & { family: MetricFamily; onBack: () => void }) {
	const Body = DETAILS[family];
	if (!Body) return <LoadingSpinner />;

    return (
        <div>
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
                <button
                    type="button"
                    onClick={onBack}
                    style={{
                        padding: "6px 12px",
                        fontSize: 12,
                        fontFamily: themeVars.font,
                        color: themeVars.textMuted,
                        background: "transparent",
                        border: `1px solid ${themeVars.border}`,
                        cursor: "pointer",
                    }}
                >
                    ← ALL METRICS
                </button>
                <div
                    style={{
                        fontFamily: themeVars.font,
                        fontSize: 15,
                        fontWeight: 600,
                        color: themeVars.text,
                    }}
                >
                    {FAMILY_TITLES[family]}
                </div>
            </div>
 
            <Body {...props} />
        </div>
    );
}