import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { usePolling } from "../hooks/usePolling";
import { usePagination, Pagination } from "../hooks/usePagination";
import { LoadingText, tableHeaderStyle, tableCellStyle, tableMutedCellStyle } from "./ui";
import { themeVars } from "../theme";
import type { Service } from "../types";

interface ServicesTabProps {
    agentId: string;
}

const PAGE_SIZE = 20;

function statusColor(subStatus: string): string {
    switch (subStatus) {
        case "running":
            return themeVars.ok;
        case "failed":
            return themeVars.danger;
        case "exited":
        case "dead":
            return themeVars.textDim;
        default:
            return themeVars.textMuted;
    }
}

type FilterMode = "all" | "active" | "failed" | "inactive";

const FILTER_OPTIONS: { value: FilterMode; label: string }[] = [
    { value: "all", label: "All" },
    { value: "active", label: "Active" },
    { value: "failed", label: "Failed" },
    { value: "inactive", label: "Inactive" },
];

function matchesFilter(s: Service, filter: FilterMode): boolean {
    switch (filter) {
        case "all":
            return true;
        case "active":
            return s.status === "active";
        case "failed":
            return s.status === "failed";
        case "inactive":
            return s.status === "inactive";
    }
}

export function ServicesTab({ agentId }: ServicesTabProps) {
    const [filter, setFilter] = useState<FilterMode>("all");
    const [search, setSearch] = useState("");

    const fetcher = useCallback(
        () => api.agentServices(agentId),
        [agentId]
    );

    const { data, loading, error } = usePolling(fetcher, 30_000);

    const filtered = useMemo(() => {
        if (!data) return [];
        return data
            .filter((s) => matchesFilter(s, filter))
            .filter((s) => !search || s.name.toLowerCase().includes(search.toLowerCase()));
    }, [data, filter, search]);

    const counts = useMemo(() => {
        if (!data) return { all: 0, active: 0, failed: 0, inactive: 0 };
        return {
            all: data.length,
            active: data.filter((s) => s.status === "active").length,
            failed: data.filter((s) => s.status === "failed").length,
            inactive: data.filter((s) => s.status === "inactive").length,
        };
    }, [data]);

    const { paged, page, setPage, totalPages, total } = usePagination(filtered, 20);

    useEffect(() => { setPage(0); }, [filter, search, setPage]);

    return (
        <div>
            {/* Filter bar */}
            <div style={{ display: "flex", gap: 12, marginBottom: 12, alignItems: "center", flexWrap: "wrap" }}>
                <div style={{ display: "flex", gap: 4 }}>
                    {FILTER_OPTIONS.map((f) => (
                        <button
                            key={f.value}
                            onClick={() => setFilter(f.value)}
                            style={{
                                padding: "4px 10px",
                                fontSize: 11,
                                fontFamily: themeVars.font,
                                color: filter === f.value ? themeVars.text : themeVars.textMuted,
                                background: filter === f.value ? themeVars.accentDim : "transparent",
                                border: `1px solid ${filter === f.value ? themeVars.accent : themeVars.border}`,
                                cursor: "pointer",
                                textTransform: "uppercase",
                                letterSpacing: "0.03em",
                            }}
                        >
                            {f.label} ({counts[f.value]})
                        </button>
                    ))}
                </div>
                <input
                    type="text"
                    placeholder="Filter services..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    style={{
                        padding: "4px 8px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        flex: "0 1 200px",
                    }}
                />
            </div>

            {loading && !data && <LoadingText />}
            {error && (
                <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                    {error}
                </div>
            )}

            {data && filtered.length === 0 && (
                <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                    No services match the current filter.
                </div>
            )}

            {filtered.length > 0 && (
                <>
                <div style={{ overflowX: "auto" }}>
                    <table style={{ width: "100%", borderCollapse: "collapse" }}>
                        <thead>
                            <tr>
                                <td style={tableHeaderStyle}>Service</td>
                                <td style={tableHeaderStyle}>Status</td>
                            </tr>
                        </thead>
                        <tbody>
                            {paged.map((s: Service, index: number) => {
                                return (
                                    <tr
                                        key={s.name}
                                        style={{
                                            background: index % 2 === 0 ? "transparent" : `color-mix(in srgb, ${themeVars.surfaceHover} 70%, transparent)`,
                                        }}
                                    >
                                        <td style={tableCellStyle}>{s.name}</td>
                                        <td style={tableCellStyle}>
                                            <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
                                                <span
                                                    style={{
                                                        width: 7,
                                                        height: 7,
                                                        borderRadius: "50%",
                                                        background: statusColor(s.sub_status),
                                                        flexShrink: 0,
                                                    }}
                                                />
                                                {s.sub_status}
                                            </span>
                                        </td>
                                    </tr>
                                )
                            })}
                        </tbody>
                    </table>
                </div>
                <Pagination page={page} totalPages={totalPages} total={total} pageSize={PAGE_SIZE} onPageChange={setPage} />
                </>
            )}
        </div>
    );
}