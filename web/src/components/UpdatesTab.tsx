import { useCallback, useEffect, useState } from "react";
import { api } from "../api";
import { usePolling } from "../hooks/usePolling";
import { usePagination, Pagination } from "../hooks/usePagination";
import {
    LoadingText,
    StatBlock,
    tableHeaderStyle,
    tableCellStyle,
    tableMutedCellStyle
} from "./ui";
import { themeVars } from "../theme";
import type { Updates } from "../types";

interface UpdatesTabProps {
    agentId: string;
}

export function UpdatesTab({ agentId }: UpdatesTabProps) {
    const [data, setData] = useState<Updates | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        api.agentUpdates(agentId)
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

    if (loading && !data) return <LoadingText />

    if (error) {
        return (
            <div style={{ color: themeVars.danger, fontFamily: themeVars.font, fontSize: 12 }}>
                {error}
            </div>
        );
    }

    if (!data) {
        return (
            <div style={{ color: themeVars.textDim, fontFamily: themeVars.font, fontSize: 12 }}>
                No update information available.
            </div>
        );
    }

    return (
        <div>
            {/* Summary cards */}
            <div
                style={{
                    display: "flex",
                    gap: 24,
                    marginBottom: 20,
                    flexWrap: "wrap",
                }}
            >
                <StatBlock
                    label="Pending"
                    value={String(data.pending_count)}
                    color={data.pending_count > 0 ? themeVars.warn : themeVars.ok}
                />
                <StatBlock
                    label="Security Updates"
                    value={String(data.security_count)}
                    color={data.security_count > 0 ? themeVars.danger : themeVars.ok}
                />
                <StatBlock
                    label="Reboot Required"
                    value={data.reboot_required ? "Yes" : "No"}
                    color={data.reboot_required ? themeVars.danger : themeVars.ok}
                />
                <StatBlock
                    label="Package Manager"
                    value={data.package_manager}
                />
            </div>

            {/* Status message */}
            {data.pending_count === 0 && (
                <div
                    style={{
                        padding: "12px 16px",
                        fontFamily: themeVars.font,
                        fontSize: 12,
                        color: themeVars.ok,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                >
                    System is up to date.
                </div>
            )}

            {data.pending_count > 0 && (
                <div
                    style={{
                        padding: "12px 16px",
                        fontFamily: themeVars.font,
                        fontSize: 12,
                        color: themeVars.warn,
                        background: themeVars.surface,
                        border: `1px solid ${themeVars.border}`,
                    }}
                >
                    {data.pending_count} update{data.pending_count !== 1 ? "s" : ""} available
                    {data.security_count > 0 &&
                        ` (${data.security_count} security updates)`}
                    {data.reboot_required && "— reboot required"}
                </div>
            )}

            {/* Last checked */}
            {data.updated_at && (
                <div
                    style={{
                        marginTop: 12,
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: themeVars.textDim,
                    }}
                >
                    Last checked:{" "}
                    {new Date(data.updated_at).toLocaleString(undefined, {
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                    })}
                </div>
            )}
        </div>
    );
}