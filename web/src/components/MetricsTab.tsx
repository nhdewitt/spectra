import { useCallback, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { useMetric } from "../hooks/useMetric";
import { MetricChart, type SeriesDef } from "./MetricChart";
import type { Anomaly } from "../anomaly";
import type {
    CPUMetric,
    RangeSelection,
    DiskMetric,
    NetworkMetric,
    TemperatureMetric,
} from "../types";
import { themeVars } from "../theme";
import { MetricSelector } from "./ui";
import { PiPanels } from "./PiPanels";
import { MetricDetail, FAMILY_TITLES, type MetricFamily } from "./MetricDetail";
import {
    FindingsBadge,
    FindingsStrip,
    DetailsHint,
    useReportFindings,
    type FindingsMap,
    type ReportFindings,
} from "./Findings";
import { useThresholds } from "../ThresholdsContext";
import {
    cpuAnomalies,
    loadAnomalies,
    memoryAnomalies,
    diskAnomalies,
    diskIOAnomalies,
    networkAnomalies,
    byGroup,
} from "../metricAnomalies";

interface PanelProps {
    agentId: string;
    rangeSel: RangeSelection;
}

/**
 * Findings plumbing every summary panel shares: it reports its own detections
 * upward and receives back what MetricsTab has aggregated for its family.
 */
interface PanelReporting {
    report?: ReportFindings;
    findings?: Anomaly[];
}

/**
 * Makes a summary panel open its drill-in.
 */
function DrilldownPanel({
    family,
    onOpen,
    children,
}: {
    family: MetricFamily;
    onOpen: (f: MetricFamily) => void;
    children: ReactNode;
}) {
    return (
        <div
            role="button"
            tabIndex={0}
            aria-label={`Show detailed ${FAMILY_TITLES[family]} metrics`}
            title={`Show detailed ${FAMILY_TITLES[family]} metrics`}
            onClick={() => onOpen(family)}
            onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onOpen(family);
                }
            }}
            style={{ cursor: "pointer" }}
        >
            {children}
        </div>
    );
}

/**
 * Wraps a control so interacting with it does not trigger an
 * enclosing DrilldownPanel.
 */
function StopPropagation({ children }: { children: ReactNode }) {
    return (
        <div
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
        >
            {children}
        </div>
    );
}

interface MetricsTabProps extends PanelProps {
    cores: number;
}

interface CPUPanelProps {
    data: CPUMetric[];
    loading: boolean;
    error: string | null;
    rangeSel: RangeSelection;
}

type PivotedRow = { time: string; _ts?: number; [sensor: string]: string | number | null | undefined };

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

function CPUPanel({ data, loading, error, rangeSel, report, findings }: CPUPanelProps & PanelReporting) {
    const thresholds = useThresholds();
    const anomalies = useMemo(() => cpuAnomalies(data, thresholds), [data, thresholds]);
    useReportFindings("cpu", anomalies, report);
 
    return (
        <MetricChart
            badge={<FindingsBadge anomalies={findings} />}
            action={<DetailsHint />}
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

function LoadPanel({ data, loading, error, rangeSel, cores, report, findings }: CPUPanelProps & { cores: number } & PanelReporting) {
    const anomalies = useMemo(() => loadAnomalies(data, cores), [data, cores]);
    useReportFindings("load", anomalies, report);
 
    const refLines = useMemo(
        () => [{ y: cores, label: `${cores} cores`, color: themeVars.textDim }],
        [cores]
    );
 
    return (
        <MetricChart
            badge={<FindingsBadge anomalies={findings} />}
            action={<DetailsHint />}
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

function MemoryPanel({ agentId, rangeSel, report, findings }: PanelProps & PanelReporting) {
    const fetchMemory = useAgentMetricFetcher(agentId, api.agentMemory);
    const { data, loading, error } = useMetric(
        fetchMemory,
        rangeSel,
        pollInterval(rangeSel)
    );
    const thresholds = useThresholds();
    const anomalies = useMemo(() => memoryAnomalies(data, thresholds), [data, thresholds]);
    useReportFindings("memory", anomalies, report);
 
    return (
        <>
            <MetricChart
                badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
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

function DiskPanel({ agentId, rangeSel, report, findings }: PanelProps & PanelReporting) {
    const fetchDisk = useAgentMetricFetcher(agentId, api.agentDisk);
    const { data, loading, error } = useMetric(
        fetchDisk,
        rangeSel,
        pollInterval(rangeSel)
    );
    const thresholds = useThresholds();
    const anomalies = useMemo(() => byGroup(data, "mountpoint", (g) => diskAnomalies(g, thresholds)), [data, thresholds]);
    useReportFindings("disk", anomalies, report);
 
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
            <StopPropagation>
                <MetricSelector
                    label="Mount"
                    options={mounts}
                    value={active}
                    onChange={setSelected}
                />
            </StopPropagation>
            <MetricChart
                badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
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

function DiskIOPanel({ agentId, rangeSel, report, findings }: PanelProps & PanelReporting) {
    const fetchDiskIO = useAgentMetricFetcher(agentId, api.agentDiskIO);
    const { data, loading, error } = useMetric(
        fetchDiskIO,
        rangeSel,
        pollInterval(rangeSel)
    );
    const anomalies = useMemo(() => byGroup(data, "device", diskIOAnomalies), [data]);
    useReportFindings("diskio", anomalies, report);
 
    const formatBytesPerSecond = useCallback((v: number) => `${formatBytes(v)}/s`, []);
 
    return (
        <MetricChart
            badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
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

function NetworkPanel({ agentId, rangeSel, report, findings }: PanelProps & PanelReporting) {
    const fetchNetwork = useAgentMetricFetcher(agentId, api.agentNetwork);
    const { data, loading, error } = useMetric(
        fetchNetwork,
        rangeSel,
        pollInterval(rangeSel)
    );
    const anomalies = useMemo(() => byGroup(data, "interface", networkAnomalies), [data]);
    useReportFindings("network", anomalies, report);
 
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
            <StopPropagation>
                <MetricSelector
                    label="Interface"
                    options={ifaces}
                    value={active}
                    onChange={setSelected}
                />
            </StopPropagation>
            <MetricChart
                badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
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

function TemperaturePanel({ agentId, rangeSel, findings }: PanelProps & PanelReporting) {
    const fetchTemperature = useAgentMetricFetcher(agentId, api.agentTemperature);
    const { data, loading, error } = useMetric(
        fetchTemperature,
        rangeSel,
        pollInterval(rangeSel)
    );
 
    const sensors = useMemo(
        () => [...new Set(data.map((d: TemperatureMetric) => d.sensor))].filter(Boolean) as string[],
        [data]
    );
 
    const pivoted = useMemo(() => {
        const interval = 5000;
        const byTime = new Map<string, PivotedRow>();
 
        for (const d of data) {
            if (!d.sensor) continue;
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
            row._ts = Date.parse(row.time);
        }
        return rows;
    }, [data, sensors]);
 
    const series = useMemo<SeriesDef[]>(
        () => sensors.map((s) => ({ key: s, label: s })),
        [sensors]
    );
 
    return (
        <MetricChart
            badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
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

function WifiPanel({ agentId, rangeSel, onOpen, findings }: PanelProps & PanelReporting & { onOpen: (f: MetricFamily) => void }) {
    const fetchWifi = useAgentMetricFetcher(agentId, api.agentWifi);
    const { data, loading, error } = useMetric(
        fetchWifi,
        rangeSel,
        pollInterval(rangeSel)
    );
 
    // Guard first: an outer wrapper would render an empty clickable box for
    // every agent without a wireless interface.
    if (!loading && !error && data.length === 0) return null;
 
    return (
        <DrilldownPanel family="wifi" onOpen={onOpen}>
        <MetricChart
            badge={<FindingsBadge anomalies={findings} />}
                action={<DetailsHint />}
                title="WiFi Signal"
            data={data}
            loading={loading}
            error={error}
            unit="dBm"
            series={WIFI_SERIES}
            rangeSel={rangeSel}
        />
        </DrilldownPanel>
    );
}

export function MetricsTab({ agentId, rangeSel, cores }: MetricsTabProps) {
    const fetchCPU = useAgentMetricFetcher(agentId, api.agentCPU);
    const cpu = useMetric(fetchCPU, rangeSel, pollInterval(rangeSel));

    const [drilldown, setDrilldown] = useState<MetricFamily | null>(null);
    const closeDrilldown = useCallback(() => setDrilldown(null), []);

    const [findings, setFindings] = useState<FindingsMap>({});

    // Bail out when a family's findings are unchanged, instead of
    // re-rendering the whole tab every 30s on a healthy host.
    const report = useCallback<ReportFindings>((family, anomalies) => {
        setFindings((prev) => {
            const before = prev[family];
            if (before === anomalies) return prev;
            if (
                before &&
                before.length === anomalies.length &&
                before.every((a, i) => {
                    const anomaly = anomalies[i];
                    return anomaly !== undefined && a.key === anomaly.key && a.count === anomaly.count;
                })
            ) {
                return prev;
            }
            return { ...prev, [family]: anomalies };
        });
    }, []);

    // The drill-in replaces the grid rather than sitting over it, so the time
    // range picker in AgentDetail stays visible and keeps driving the fetch.
    if (drilldown) {
        return (
            <MetricDetail
                family={drilldown}
                agentId={agentId}
                rangeSel={rangeSel}
                cores={cores}
                onBack={closeDrilldown}
            />
        );
    }

    return (
        <div>
            <FindingsStrip findings={findings} titles={FAMILY_TITLES} onOpen={setDrilldown} />
 
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <DrilldownPanel family="cpu" onOpen={setDrilldown}>
                    <CPUPanel {...cpu} rangeSel={rangeSel} report={report} findings={findings.cpu} />
                </DrilldownPanel>
                <DrilldownPanel family="load" onOpen={setDrilldown}>
                    <LoadPanel {...cpu} rangeSel={rangeSel} cores={cores} report={report} findings={findings.load} />
                </DrilldownPanel>
            </div>
 
            <DrilldownPanel family="memory" onOpen={setDrilldown}>
                <MemoryPanel agentId={agentId} rangeSel={rangeSel} report={report} findings={findings.memory} />
            </DrilldownPanel>
 
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <DrilldownPanel family="disk" onOpen={setDrilldown}>
                    <DiskPanel agentId={agentId} rangeSel={rangeSel} report={report} findings={findings.disk} />
                </DrilldownPanel>
                <DrilldownPanel family="diskio" onOpen={setDrilldown}>
                    <DiskIOPanel agentId={agentId} rangeSel={rangeSel} report={report} findings={findings.diskio} />
                </DrilldownPanel>
            </div>
 
            <DrilldownPanel family="network" onOpen={setDrilldown}>
                <NetworkPanel agentId={agentId} rangeSel={rangeSel} report={report} findings={findings.network} />
            </DrilldownPanel>
 
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                <DrilldownPanel family="temperature" onOpen={setDrilldown}>
                    <TemperaturePanel agentId={agentId} rangeSel={rangeSel} findings={findings.temperature} />
                </DrilldownPanel>
                <WifiPanel agentId={agentId} rangeSel={rangeSel} onOpen={setDrilldown} findings={findings.wifi} />
            </div>
 
            <PiPanels agentId={agentId} rangeSel={rangeSel} />
        </div>
    );
}