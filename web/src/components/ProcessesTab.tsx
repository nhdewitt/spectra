import { useState, useCallback, useEffect } from "react";
import { api } from "../api";
import { formatBytes } from "../utils";
import { usePolling } from "../hooks/usePolling";
import { usePagination, Pagination } from "../hooks/usePagination";
import {
    LoadingText,
    MetricSelector,
    tableHeaderStyle,
    tableCellStyle,
    tableMutedCellStyle
} from "./ui";
import { themeVars } from "../theme";
import type { Process, ProcessSort } from "../types";

interface ProcessesTabProps {
    agentId: string;
}

const SORT_OPTIONS = ["cpu", "memory"];
const LIMIT_OPTIONS = ["20", "50", "100"];
const PAGE_SIZE = 20;

function cpuColor(v: number): string {
    if (v >= 80) return themeVars.danger;
    if (v >= 50) return themeVars.warn;
    return themeVars.text;
}

function memColor(v: number): string {
    if (v >= 80) return themeVars.danger;
    if (v >= 50) return themeVars.warn;
    return themeVars.text;
}

function procStatusColor(status: string): string {
    switch (status) {
        case "running":
            return themeVars.ok;
        case "runnable":
            return themeVars.accent;
        case "waiting":
            return themeVars.textMuted;
        case "other":
            return themeVars.textDim;
        default:
            return themeVars.textDim;
    }
}

export function ProcessesTab({ agentId }: ProcessesTabProps) {
    const [sort, setSort] = useState<ProcessSort>("cpu");
    const [limit, setLimit] = useState(20);


    const handleSortChange = useCallback((v: string) => {
        setSort(v as ProcessSort);
        setPage(0);
    }, []);

    const handleLimitChange = useCallback((v: string) => {
        setLimit(Number(v));
        setPage(0);
    }, []);

    const fetcher = useCallback(
        () => api.agentProcesses(agentId, sort, limit),
        [agentId, sort, limit]
    );

    const { data, loading, error } = usePolling(fetcher, 10_000);

    const { paged, page, setPage, totalPages, needsPagination, total } = usePagination(data ?? [], 20);

    useEffect(() => { setPage(0); }, [sort, limit, setPage]);

    return (
        <div>
            <div style={{ display: "flex", gap: 16, marginBottom: 12 }}>
                <MetricSelector
                    label="Sort by"
                    options={SORT_OPTIONS}
                    value={sort}
                    onChange={handleSortChange}
                />
                <MetricSelector
                    label="Show"
                    options={LIMIT_OPTIONS}
                    value={String(limit)}
                    onChange={handleLimitChange}
                />
            </div>

            {loading && !data && <LoadingText />}
            {error && (
                <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                    {error}
                </div>
            )}

            {data && data.length === 0 && (
                <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                    No processes found.
                </div>
            )}

            {data && data.length > 0 && (
                <>
                    <div style={{ display: "flex", flexDirection: "column", minHeight: "60vh" }}>
                        <div style={{ flex: 1, overflowX: "auto" }}>
                            <table
                                style={{
                                    width: "100%",
                                    borderCollapse: "collapse",
                                    fontFamily: themeVars.font,
                                }}
                            >
                                <thead>
                                    <tr>
                                        <th style={tableMutedCellStyle}>#</th>
                                        <th style={tableHeaderStyle}>PID</th>
                                        <th style={tableHeaderStyle}>Name</th>
                                        <th style={{ ...tableHeaderStyle, textAlign: "right" }}>CPU %</th>
                                        <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Mem %</th>
                                        <th style={tableHeaderStyle}>Status</th>
                                        <th style={{ ...tableHeaderStyle, textAlign: "right" }}>Threads</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {paged.map((p: Process, index: number) => {
                                        const rank = page * PAGE_SIZE + index + 1;
                                        
                                        return (
                                            <tr
                                                key={p.pid}
                                                style={{
                                                    background: index % 2 === 0 ? "transparent" : `color-mix(in srgb, ${themeVars.surfaceHover} 70%, transparent)`,
                                                }}
                                            >
                                                <td style={tableMutedCellStyle}>{rank}</td>
                                                <td style={tableMutedCellStyle}>{p.pid}</td>
                                                <td style={tableCellStyle}>{p.name}</td>
                                                <td style={{ ...tableCellStyle, textAlign: "right", color: cpuColor(p.cpu_percent) }}>
                                                    {p.cpu_percent.toFixed(1)}
                                                </td>
                                                <td style={{ ...tableCellStyle, textAlign: "right", color: memColor(p.mem_percent) }}>
                                                    {p.mem_percent.toFixed(1)}% ({formatBytes(p.mem_rss)})
                                                </td>
                                                <td style={{ ...tableMutedCellStyle, color: procStatusColor(p.status) }}>{p.status}</td>
                                                <td style={{ ...tableMutedCellStyle, textAlign: "right" }}>{p.threads}</td>
                                            </tr>
                                        )
                                    })}
                                </tbody>
                            </table>
                        </div>
                    <Pagination page ={page} totalPages={totalPages} total={total} pageSize={PAGE_SIZE} onPageChange={setPage} />
                    </div>
                </>
            )}
        </div>
    );
}