import { useState, useCallback, useMemo } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { useMetric } from "../hooks/useMetric";
import { usePolling } from "../hooks/usePolling";
import {
    tableHeaderStyle,
    tableCellStyle,
    tableMutedCellStyle,
    LoadingSpinner,
} from "./ui";
import { MetricChart, type SeriesDef } from "./MetricChart";
import { themeVars } from "../theme";
import type { ContainerMetric, RangeSelection } from "../types";

interface ContainersTabProps {
    agentId: string;
    rangeSel: RangeSelection;
}

function containerStateColor(state: string): string {
    switch (state.toLowerCase()) {
        case "running":
            return themeVars.ok;
        case "paused":
            return themeVars.warn;
        case "exited":
        case "stopped":
        case "dead":
            return themeVars.danger;
        case "created":
        case "restarting":
            return themeVars.accent;
        default:
            return themeVars.textDim;
    }
}

interface ContainerSnapshot {
    container_id: string;
    name: string;
    image: string;
    state: string;
    source: string;
    kind: string;
    cpu_percent: number;
    memory_bytes: number;
    memory_limit: number;
    net_rx_bytes: number;
    net_tx_bytes: number;
}

function latestSnapshots(data: ContainerMetric[]): ContainerSnapshot[] {
    const latest = new Map<string, ContainerMetric>();
    for (const d of data) {
        const existing = latest.get(d.container_id);
        if (!existing || d.time > existing.time) {
            latest.set(d.container_id, d);
        }
    }

    return [...latest.values()]
        .map((d) => ({
            container_id: d.container_id,
            name: d.name,
            image: d.image,
            state: d.state,
            source: d.source,
            kind: d.kind,
            cpu_percent: d.cpu_percent,
            memory_bytes: d.memory_bytes,
            memory_limit: d.memory_limit,
            net_rx_bytes: d.net_rx_bytes,
            net_tx_bytes: d.net_tx_bytes,
        }))
        .sort((a, b) => {
            const aRunning = a.state.toLowerCase() === "running" ? 0 : 1;
            const bRunning = b.state.toLowerCase() === "running" ? 0 : 1;
            if (aRunning !== bRunning) return aRunning - bRunning;
            return a.name.localeCompare(b.name);
        });
}

function deriveRates<T extends { time: string }>(
    data: T[],
    keys: string[]
): (T & Record<string, number>)[] {
    if (data.length < 2) return [];

    const result: (T & Record<string, number>)[] = [];

    for (let i = 1; i < data.length; i++) {
        const prev = data[i - 1]!;
        const curr = data[i]!;
        const dtSec = (Date.parse(curr.time) - Date.parse(prev.time)) / 1000;

        if (dtSec <= 0) continue;

        const row = { ...curr } as T & Record<string, number>;
        for (const key of keys) {
            const prevVal = (prev as Record<string, unknown>)[key] as number;
            const currVal = (curr as Record<string, unknown>)[key] as number;
            const delta = currVal - prevVal;
            (row as Record<string, unknown>)[`${key}_rate`] = delta >= 0 ? delta / dtSec : 0;
        }
        result.push(row);
    }

    return result;
}

const CPU_SERIES: SeriesDef[] = [
    { key: "cpu_percent", label: "CPU", area: true },
];

const MEMORY_SERIES: SeriesDef[] = [
    { key: "memory_bytes", label: "Memory", area: true },
];

const NET_SERIES: SeriesDef[] = [
    { key: "net_rx_bytes_rate", label: "RX" },
    { key: "net_tx_bytes_rate", label: "TX" },
];

function ContainerDetail({
    agentId,
    containerId,
    containerName,
    rangeSel,
}: {
    agentId: string;
    containerId: string;
    containerName: string;
    rangeSel: RangeSelection;
}) {
    const fetcher = useCallback(
        (sel: RangeSelection, signal?: AbortSignal) =>
            api.agentContainers(agentId, sel, { signal }),
        [agentId]
    );

    const { data, loading, error } = useMetric(fetcher, rangeSel);

    const filtered = useMemo(
        () => data.filter((d) => d.container_id === containerId),
        [data, containerId]
    );

    const rateData = useMemo(
        () => deriveRates(filtered, ["net_rx_bytes", "net_tx_bytes"]),
        [filtered]
    );

    return (
        <div
            style={{
                padding: "16px 0",
                borderTop: `1px solid ${themeVars.border}`,
            }}
        >
            <div
                style={{
                    fontFamily: themeVars.font,
                    fontSize: 13,
                    fontWeight: 600,
                    color: themeVars.text,
                    marginBottom: 12,
                }}
            >
                {containerName}
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12 }}>
                <MetricChart
                    title="CPU"
                    data={filtered}
                    loading={loading}
                    error={error}
                    unit="%"
                    series={CPU_SERIES}
                    height={160}
                    rangeSel={rangeSel}
                />
                <MetricChart
                    title="Memory"
                    data={filtered}
                    loading={loading}
                    error={error}
                    formatter={(v) => formatBytes(v)}
                    series={MEMORY_SERIES}
                    height={160}
                    rangeSel={rangeSel}
                />
                <MetricChart
                    title="Network"
                    data={rateData}
                    loading={loading}
                    error={error}
                    formatter={(v) => `${formatBytes(v)}/s`}
                    series={NET_SERIES}
                    height={160}
                    rangeSel={rangeSel}
                />
            </div>
        </div>
    );
}

export function ContainersTab({ agentId, rangeSel }: ContainersTabProps) {
    const [selectedId, setSelectedId] = useState<string | null>(null);

    const fetcher = useCallback(
        () => api.agentContainers(agentId, { type: "quick", range: "1h" }).then(d => {
            console.log("containers response:", d?.length, d);
            return d;
        }),
        [agentId]
    );

    const { data: rawData, loading, error } = usePolling(fetcher, 15_000);

    const snapshots = useMemo(
        () => latestSnapshots(rawData ?? []),
        [rawData]
    );

    const selected = snapshots.find((s) => s.container_id === selectedId);

    if (loading && !rawData) return <LoadingSpinner />

    if (error) {
        return (
            <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                {error}
            </div>
        );
    }

    if (snapshots.length === 0) {
        return (
            <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                No containers found.
            </div>
        );
    }

    return (
        <div>
            {/* Summary */}
            <div
                style={{
                    fontSize: 11,
                    fontFamily: themeVars.font,
                    color: themeVars.textDim,
                    marginBottom: 12,
                }}
            >
                {snapshots.filter((s) => s.state.toLowerCase() === "running").length} of{" "}
                {snapshots.length} running
            </div>

            {/* Table */}
            <div style={{ overflowX: "auto" }}>
                <table
                    style={{
                        width: "100%",
                        borderCollapse: "collapse",
                        textAlign: "left",
                    }}
                >
                    <thead>
                        <tr>
                            <th style={tableHeaderStyle}>Name</th>
                            <th style={tableHeaderStyle}>Image</th>
                            <th style={tableHeaderStyle}>Type</th>
                            <th style={tableHeaderStyle}>State</th>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>CPU</th>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Memory</th>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Net RX</th>
                            <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Net TX</th>
                        </tr>
                    </thead>
                    <tbody>
                        {snapshots.map((c, i) => (
                            <tr
                                key={c.container_id}
                                onClick={() =>
                                    setSelectedId(
                                        selectedId === c.container_id ? null : c.container_id
                                    )
                                }
                                style={{
                                    cursor: "pointer",
                                    background:
                                        selectedId === c.container_id
                                            ? themeVars.accentDim
                                            : i % 2 === 0
                                                ? "transparent"
                                                : themeVars.surfaceHover,
                                }}
                            >
                                <td style={tableCellStyle}>
                                    <span style={{ fontWeight: 500 }}>{c.name}</span>
                                </td>
                                <td style={tableMutedCellStyle}>
                                    {c.image.length > 40 ? `…${c.image.slice(-40)}` : c.image}
                                </td>
                                <td style={tableMutedCellStyle}>{c.kind}</td>
                                <td style={tableCellStyle}>
                                    <span
                                        style={{
                                            display: "flex",
                                            alignItems: "center",
                                            gap: 6,
                                        }}
                                    >
                                        <span
                                            style={{
                                                width: 7,
                                                height: 7,
                                                borderRadius: "50%",
                                                background: containerStateColor(c.state),
                                                flexShrink: 0,
                                            }}
                                        />
                                        {c.state}
                                    </span>
                                </td>
                                <td style={{ ...tableCellStyle, textAlign: "right" }}>
                                    {c.state.toLowerCase() === "running"
                                        ? `${c.cpu_percent.toFixed(1)}%`
                                        : "—"}
                                </td>
                                <td style={{ ...tableCellStyle, textAlign: "right" }}>
                                    {c.state.toLowerCase() === "running"
                                        ? `${formatBytes(c.memory_bytes)} / ${formatBytes(c.memory_limit)}`
                                        : "—"}
                                </td>
                                <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                    {c.state.toLowerCase() === "running"
                                        ? formatBytes(c.net_rx_bytes)
                                        : "—"}
                                </td>
                                <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>
                                    {c.state.toLowerCase() === "running"
                                        ? formatBytes(c.net_tx_bytes)
                                        : "—"}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
            {/* Expanded detail */}
            {selected && (
                <ContainerDetail
                    agentId={agentId}
                    containerId={selected.container_id}
                    containerName={selected.name}
                    rangeSel={rangeSel}
                />
            )}
        </div>
    );
}