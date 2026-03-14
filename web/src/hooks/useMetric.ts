import { useState, useEffect, useCallback, useRef } from "react";
import type { RangeSelection } from "../types";

interface UseMetricResult<T extends { time: string }> {
    data: T[];
    loading: boolean;
    error: string | null;
    refetch: () => void;
}

function downsample<T extends { time: string }>(data: T[], maxPoints: number): T[] {
    if (data.length <= maxPoints) return data;

    const step = data.length / maxPoints;
    const result: T[] = [];

    for (let i = 0; i < maxPoints; i++) {
        const point = data[Math.floor(i * step)];
        if (point) result.push(point);
    }

    const last = data[data.length - 1];
    if (last && result[result.length - 1] !== last) {
        result.push(last);
    }

    return result;
}

function insertGaps<T extends { time: string }>(data: T[], maxGapMs: number): T[] {
    if (data.length < 2) return data;

    const result: T[] = [];
    let currMs = Date.parse(data[0]!.time);

    for (let i = 0; i < data.length; i++) {
        const point = data[i]!;
        result.push(point);

        if (i === data.length - 1) break;

        const nextPoint = data[i + 1]!;
        const nextMs = Date.parse(nextPoint.time);

        if (nextMs - currMs > maxGapMs) {
            const gap = { ...point } as T;
            for (const key in gap as Record<string, unknown>) {
                if (key !== "time" && key !== "agent_id") {
                    (gap as Record<string, unknown>)[key] = null;
                }
            }
            gap.time = new Date(currMs + 1).toISOString();
            result.push(gap);
        }

        currMs = nextMs;
    }

    return result;
}

function gapThreshold(sel: RangeSelection): number {
    if (sel.type === "custom") return 5 * 60_000;

    switch (sel.range) {
        case "5m":
        case "15m":
            return 60_000;
        case "1h":
            return 2 * 60_000;
        case "6h":
            return 5 * 60_000;
        case "24h":
            return 15 * 60_000;
        case "7d":
            return 60 * 60_000;
        case "30d":
            return 4 * 60 * 60_000;
        default:
            return 5 * 60_000;
    }
}

function prepareMetricData<T extends { time: string }>(
    result: T[],
    rangeSel: RangeSelection,
    maxPoints = 500
): T[] {
    return insertGaps(
        downsample([...result].reverse(), maxPoints),
        gapThreshold(rangeSel)
    );
}

function isAbortError(err: unknown): boolean {
    return (
        err instanceof DOMException && err.name === "AbortError"
    ) || (
        err instanceof Error && err.name === "AbortError"
    );
}

export function useMetric<T extends { time: string }>(
    fetcher: (sel: RangeSelection, signal?: AbortSignal) => Promise<T[]>,
    rangeSel: RangeSelection,
    pollMs = 0
): UseMetricResult<T> {
    const [data, setData] = useState<T[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const controllerRef = useRef<AbortController | null>(null);

    const load = useCallback(
        async (showLoading: boolean) => {
            controllerRef.current?.abort();

            const controller = new AbortController();
            controllerRef.current = controller;

            if (showLoading) setLoading(true);

            try {
                const result = await fetcher(rangeSel, controller.signal);

                if (controller.signal.aborted) return;

                setData(prepareMetricData(result, rangeSel));
                setError(null);
            } catch (err) {
                if (isAbortError(err)) return;
                setError(err instanceof Error ? err.message : "Failed to load");
            } finally {
                if (!controller.signal.aborted) {
                    setLoading(false);
                }
            }
        },
        [fetcher, rangeSel]
    );

    const refetch = useCallback(() => {
        void load(false);
    }, [load]);

    useEffect(() => {
        void load(true);

        if (pollMs <= 0) {
            return () => {
                controllerRef.current?.abort();
            };
        }

        const id = setInterval(() => {
            void load(false);
        }, pollMs);

        return () => {
            clearInterval(id);
            controllerRef.current?.abort();
        };
    }, [load, pollMs]);

    return { data, loading, error, refetch };
}