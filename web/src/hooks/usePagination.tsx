import { useState, useCallback } from "react";
import { themeVars } from "../theme";

export function usePagination<T>(items: T[], pageSize: number) {
    const [page, setPage] = useState(0);
    const totalPages = Math.ceil(items.length / pageSize);

    const safePage = Math.min(page, Math.max(0, totalPages - 1));
    if (safePage !== page) setPage(safePage);

    const paged = items.slice(safePage * pageSize, (safePage + 1) * pageSize);
    const needsPagination = items.length > pageSize;

    const reset = useCallback(() => setPage(0), []);

    return { paged, page: safePage, setPage, totalPages, needsPagination, total: items.length, reset };
}

export function Pagination({
    page,
    totalPages,
    total,
    pageSize,
    onPageChange,
}: {
    page: number;
    totalPages: number;
    total: number;
    pageSize: number;
    onPageChange: (p: number) => void;
}) {
    if (totalPages <= 1) return null;

    return (
        <div
            style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                marginTop: 12,
                fontFamily: themeVars.font,
                fontSize: 11,
            }}
        >
            <span style={{ color: themeVars.textDim }}>
                {page * pageSize + 1}–{Math.min((page + 1) * pageSize, total)} of {total}
            </span>
            <div style={{ display: "flex", gap: 4 }}>
                <button
                    onClick={() => onPageChange(page - 1)}
                    disabled={page === 0}
                    style={{
                        padding: "3px 8px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: page === 0 ? themeVars.textDim : themeVars.text,
                        background: "transparent",
                        border: `1px solid ${themeVars.border}`,
                        cursor: page === 0 ? "default" : "pointer",
                        opacity: page === 0 ? 0.5 : 1,
                    }}
                >
                    ← Prev
                </button>
                <button
                    onClick={() => onPageChange(page + 1)}
                    disabled={page >= totalPages - 1}
                    style={{
                        padding: "3px 8px",
                        fontSize: 11,
                        fontFamily: themeVars.font,
                        color: page >= totalPages - 1 ? themeVars.textDim : themeVars.text,
                        background: "transparent",
                        border: `1px solid ${themeVars.border}`,
                        cursor: page >= totalPages - 1 ? "default" : "pointer",
                        opacity: page >= totalPages - 1 ? 0.5 : 1,
                    }}
                >
                    Next →
                </button>
            </div>
        </div>
    );
}