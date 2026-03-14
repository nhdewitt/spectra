import { useCallback, useMemo, useState } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { useMetric } from "../hooks/useMetric";
import { MetricChart, type SeriesDef } from "./MetricChart";
import type {
    RangeSelection,
    DiskMetric,
    NetworkMetric,
    TemperatureMetric,
} from "../types";
import { themeVars } from "../theme";
import { MetricSelector } from "./ui";

interface PanelProps {
    agentId: string;
    rangeSel: RangeSelection;
}

interface MetricsTabProps extends PanelProps {
    cores: number;
}

type PivotedRow = { time: string; [sensor: string]: string | number | null };

type MetricFetcher<T extends { time: string }> = (
    id: string,
    sel?: RangeSelection,
    opts?: { signal?: AbortSignal }
) => Promise<T[]>;

function useAgentMetricFetcher<T extends { time: string }>(
    agentId: string,
    fn: MetricFetcher<T>
) {
    return useCallback(
        (sel: RangeSelection, signal?: AbortSignal) =>
            fn(agentId, sel, { signal }),
        [agentId, fn]
    );
}

function pollInterval(sel: RangeSelection): number {
    if (sel.type === "custom") return 0;
    return ["5m", "15m", "1h"].includes(sel.range) ? 30_000 : 0;
}

function roundToInterval(iso: string, intervalMs: number): string {
    const t = new Date(iso).getTime();
    return new Date(Math.round(t / intervalMs) * intervalMs).toISOString();
}

const CPU_SERIES: SeriesDef[] = [
    { key: "usage", label: "Usage", area: true },
    { key: "iowait", label: "IO Wait" },
];

const LOAD_SERIES: SeriesDef[] = [
    { key: "load_1m", label: "1m" },
    { key: "load_5m", label: "5m" },
    { key: "load_15m", label: "15m" },
];

const MEMORY_PERCENT_SERIES: SeriesDef[] = [
    { key: "ram_percent", label: "RAM", area: true },
    { key: "swap_percent", label: "Swap" },
];

const MEMORY_ABSOLUTE_SERIES: SeriesDef[] = [
    { key: "ram_used", label: "RAM Used", area: true },
    { key: "ram_available", label: "Available" },
    { key: "swap_used", label: "Swap Used" },
];

const DISK_IO_SERIES: SeriesDef[] = [
    { key: "read_bytes", label: "Bytes Read" },
    { key: "write_bytes", label: "Bytes Written" },
];

const NETWORK_SERIES: SeriesDef[] = [
    { key: "rx_bytes", label: "RX" },
    { key: "tx_bytes", label: "TX" },
];

const WIFI_SERIES: SeriesDef[] = [
    { key: "signal_dbm", label: "Signal" },
    { key: "noise_dbm", label: "Noise" },
];

function CPUPanel({ agentId, rangeSel }: PanelProps) {
    const fetchCPU = useAgentMetricFetcher(agentId, api.agentCPU);
    const { data, loading, error } = useMetric(
        fetchCPU,
        rangeSel,
        pollInterval(rangeSel)
    );

    return (
        <MetricChart
            title="CPU"
            data={data}
            loading={loading}
            error={error}
            unit="%"
            yDomain={[0, 100]}
            series={CPU_SERIES}
            rangeSel={rangeSel}
        />
    );
}

function LoadPanel({ agentId, rangeSel, cores }: PanelProps & { cores: number }) {
    const fetchCPU = useAgentMetricFetcher(agentId, api.agentCPU);
    const { data, loading, error } = useMetric(
        fetchCPU,
        rangeSel,
        pollInterval(rangeSel)
    );

    const refLines = useMemo(
        () => [{ y: cores, label: `${cores} cores`, color: themeVars.textDim }],
        [cores]
    );

    return (
        <MetricChart
            title="Load Average"
            data={data}
            loading={loading}
            error={error}
            series={LOAD_SERIES}
            refLines={refLines}
            rangeSel={rangeSel}
        />
    );
}

function MemoryPanel({ agentId, rangeSel }: PanelProps) {
    const fetchMemory = useAgentMetricFetcher(agentId, api.agentMemory);
    const { data, loading, error } = useMetric(
        fetchMemory,
        rangeSel,
        pollInterval(rangeSel)
    );

    return (
        <>
            <MetricChart
                title="Memory"
                data={data}
                loading={loading}
                error={error}
                unit="%"
                yDomain={[0, 100]}
                series={MEMORY_PERCENT_SERIES}
                rangeSel={rangeSel}
            />
            <MetricChart
                title="Memory (Absolute)"
                data={data}
                loading={loading}
                error={error}
                formatter={formatBytes}
                series={MEMORY_ABSOLUTE_SERIES}
                rangeSel={rangeSel}
            />
        </>
    );
}

function DiskPanel({ agentId, rangeSel }: PanelProps) {
    const fetchDisk = useAgentMetricFetcher(agentId, api.agentDisk);
    const { data, loading, error } = useMetric(
        fetchDisk,
        rangeSel,
        pollInterval(rangeSel)
    );

    const mounts = useMemo(
        () => [...new Set(data.map((d: DiskMetric) => d.mountpoint))],
        [data]
    );

    const [selected, setSelected] = useState("");
    const active = mounts.includes(selected) ? selected : mounts[0] ?? "";

    const filteredData = useMemo(
        () => data.filter((d: DiskMetric) => d.mountpoint === active),
        [data, active]
    );

    const series = useMemo<SeriesDef[]>(
        () => [{ key: "used_percent", label: active, area: true }],
        [active]
    );

    const diskFormatter = useCallback(
        (v: number, key: string) => {
            if (key === active) {
                const latest = filteredData.find((d) => d.used_percent === v);
                if (latest) {
                    return `${v.toFixed(1)}% (${formatBytes(latest.free_bytes)} free of ${formatBytes(latest.total_bytes)})`;
                }
            }
            return `${v.toFixed(1)}`;
        },
        [filteredData, active]
    );

    return (
        <div>
            <MetricSelector
                label="Mount"
                options={mounts}
                value={active}
                onChange={setSelected}
            />
            <MetricChart
                title="Disk Usage"
                data={filteredData}
                loading={loading}
                error={error}
                formatter={diskFormatter}
                yDomain={[0, 100]}
                series={series}
                rangeSel={rangeSel}
            />
        </div>
    );
}

function DiskIOPanel({ agentId, rangeSel }: PanelProps) {
    const fetchDiskIO = useAgentMetricFetcher(agentId, api.agentDiskIO);
    const { data, loading, error } = useMetric(
        fetchDiskIO,
        rangeSel,
        pollInterval(rangeSel)
    );

    const formatBytesPerSecond = useCallback((v: number) => `${formatBytes(v)}/s`, []);

    return (
        <MetricChart
            title="Disk I/O"
            data={data}
            loading={loading}
            error={error}
            formatter={formatBytesPerSecond}
            series={DISK_IO_SERIES}
            rangeSel={rangeSel}
        />
    );
}

function NetworkPanel({ agentId, rangeSel }: PanelProps) {
    const fetchNetwork = useAgentMetricFetcher(agentId, api.agentNetwork);
    const { data, loading, error } = useMetric(
        fetchNetwork,
        rangeSel,
        pollInterval(rangeSel)
    );

    const ifaces = useMemo(() => {
        const trafficByIface = new Map<string, number>();
        for (const d of data) {
            const total = (trafficByIface.get(d.interface) ?? 0) + d.rx_bytes + d.tx_bytes;
            trafficByIface.set(d.interface, total);
        }
        return [...trafficByIface.entries()]
            .sort((a, b) => b[1] - a[1])
            .map(([name]) => name);
    }, [data]);

    const [selected, setSelected] = useState("");
    const active = ifaces.includes(selected) ? selected : ifaces[0] ?? "";

    const filteredData = useMemo(
        () => (active ? data.filter((d: NetworkMetric) => d.interface === active) : data),
        [data, active]
    );

    const formatBytesPerSecond = useCallback(
        (v: number) => `${formatBytes(v)}/s`,
        []
    );

    return (
        <div>
            <MetricSelector
                label="Interface"
                options={ifaces}
                value={active}
                onChange={setSelected}
            />
            <MetricChart
                title="Network"
                data={filteredData}
                loading={loading}
                error={error}
                formatter={formatBytesPerSecond}
                series={NETWORK_SERIES}
                rangeSel={rangeSel}
            />
        </div>
    );
}

function TemperaturePanel({ agentId, rangeSel }: PanelProps) {
    const fetchTemperature = useAgentMetricFetcher(agentId, api.agentTemperature);
    const { data, loading, error } = useMetric(
        fetchTemperature,
        rangeSel,
        pollInterval(rangeSel)
    );

    const sensors = useMemo(
        () => [...new Set(data.map((d: TemperatureMetric) => d.sensor))],
        [data]
    );

    const pivoted = useMemo(() => {
        const interval = 5000;
        const byTime = new Map<string, PivotedRow>();

        for (const d of data) {
            const key = roundToInterval(d.time, interval);
            let row = byTime.get(key);
            if (!row) {
                row = { time: key };
                byTime.set(key, row);
            }
            row[d.sensor] = d.temperature;
        }

        const rows = [...byTime.values()];
        const last: Record<string, number> = {};
        for (const row of rows) {
            for (const s of sensors) {
                if (row[s] != null) {
                    last[s] = row[s] as number;
                } else if (last[s] != null) {
                    row[s] = last[s];
                }
            }
        }
        return rows;
    }, [data, sensors]);

    const series = useMemo<SeriesDef[]>(
        () => sensors.map((s) => ({ key: s, label: s })),
        [sensors]
    );

    return (
        <MetricChart
            title="Temperature"
            data={pivoted}
            loading={loading}
            error={error}
            unit="°C"
            series={series}
            rangeSel={rangeSel}
        />
    );
}

function WifiPanel({ agentId, rangeSel }: PanelProps) {
    const fetchWifi = useAgentMetricFetcher(agentId, api.agentWifi);
    const { data, loading, error } = useMetric(
        fetchWifi,
        rangeSel,
        pollInterval(rangeSel)
    );

    if (!loading && !error && data.length === 0) return null;

    return (
        <MetricChart
            title="WiFi Signal"
            data={data}
            loading={loading}
            error={error}
            unit="dBm"
            series={WIFI_SERIES}
            rangeSel={rangeSel}
        />
    );
}

export function MetricsTab({ agentId, rangeSel, cores }: MetricsTabProps) {
    return (
        <div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <CPUPanel agentId={agentId} rangeSel={rangeSel} />
                <LoadPanel agentId={agentId} rangeSel={rangeSel} cores={cores} />
            </div>

            <MemoryPanel agentId={agentId} rangeSel={rangeSel} />

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <DiskPanel agentId={agentId} rangeSel={rangeSel} />
                <DiskIOPanel agentId={agentId} rangeSel={rangeSel} />
            </div>

            <NetworkPanel agentId={agentId} rangeSel={rangeSel} />

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <TemperaturePanel agentId={agentId} rangeSel={rangeSel} />
                <WifiPanel agentId={agentId} rangeSel={rangeSel} />
            </div>
        </div>
    );
}