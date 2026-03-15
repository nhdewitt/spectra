import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import { usePagination, Pagination } from "../hooks/usePagination";
import { LoadingText, tableHeaderStyle, tableCellStyle, tableMutedCellStyle } from "./ui";
import { themeVars } from "../theme";
import type { Application } from "../types";

interface ApplicationsTabProps {
    agentId: string;
}

const PAGE_SIZE = 20;

export function ApplicationsTab({ agentId }: ApplicationsTabProps) {
    const [data, setData] = useState<Application[] | null>(null);
    const [search, setSearch] = useState("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        api.agentApplications(agentId)
            .then((result) => {
                if (!cancelled) {
                    setData(result);
                    setError(null);
                }
            })
            .catch((err) => {
                if (!cancelled) {
                    setError(err instanceof Error ? err.message : "Failed to load");
                }
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => { cancelled = true; };
    }, [agentId]);

    const filtered = useMemo(() => {
        if (!data) return [];
        if (!search) return data;
        const q = search.toLowerCase();
        return data.filter(
            (a) =>
                a.name.toLowerCase().includes(q) ||
                a.version.toLowerCase().includes(q)
        );
    }, [data, search]);

    const { paged, page, setPage, totalPages, total } = usePagination(filtered, 20);

    useEffect(() => { setPage(0); }, [filtered, search, setPage]);

    return (
        <div>
            <div style={{ display: "flex", gap: 12, marginBottom: 12, alignItems: "center" }}>
                <input
                    type="text"
                    placeholder="Search applications..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    style={{
                        padding: "4px 8px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.text,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                        flex: "0 1 250px",
                    }}
                />
                {data && (
                    <span
                        style={{
                            fontSize: 11,
                            fontFamily: themeVars.font,
                            color: themeVars.textDim,
                        }}
                    >
                        {filtered.length} of {data.length} packages
                    </span>
                )}
            </div>

            {loading && !data && <LoadingText />}
            {error && (
                <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                    {error}
                </div>
            )}

            {data && filtered.length === 0 && (
                <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                    {search ? "No applications match your search." : "No applications found."}
                </div>
            )}

            {filtered.length > 0 && (
                <>
                <div style={{ display: "flex", flexDirection: "column", minHeight: "60vh" }}>
                    <div style={{ flex: 1, overflowX: "auto" }}>
                        <table style={{ width: "100%", borderCollapse: "collapse" }}>
                            <thead>
                                <tr>
                                    <th style={tableHeaderStyle}>Name</th>
                                    <th style={tableHeaderStyle}>Version</th>
                                </tr>
                            </thead>
                            <tbody>
                                {paged.map((a: Application, index: number) => {
                                    return (
                                        <tr
                                            key={`${a.name}-${a.version}`}
                                            style={{
                                                background: index % 2 === 0 ? "transparent" : `color-mix(in srgb, ${themeVars.surfaceHover} 70%, transparent)`,
                                            }}
                                        >
                                            <td style={tableCellStyle}>{a.name}</td>
                                            <td style={tableMutedCellStyle}>{a.version}</td>
                                        </tr>
                                    )
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
                <Pagination page={page} totalPages={totalPages} total={total} pageSize={PAGE_SIZE} onPageChange={setPage} />
                </>
            )}
        </div>
    )
}