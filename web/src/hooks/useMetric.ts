import { useState, useEffect, useCallback, useRef } from "react";
import type { RangeSelection } from "../types";

interface UseMetricResult<T extends { time: string }> {
    data: T[];
    loading: boolean;
    error: string | null;
    refetch: () => void;
}

function rangeStart(sel: RangeSelection): Date {
    if (sel.type === "custom") return new Date(sel.start);

    const now = new Date();
    const ms: Record<string, number> = {
        "5m": 5 * 60_000,
        "15m": 15 * 60_000,
        "1h": 60 * 60_000,
        "6h": 6 * 60 * 60_000,
        "24h": 24 * 60 * 60_000,
        "7d": 7 * 24 * 60 * 60_000,
        "30d": 30 * 24 * 60 * 60_000,
    };
    return new Date(now.getTime() - (ms[sel.range] ?? 60 * 60_000));
}

function downsample<T extends { time: string }>(data: T[], maxPoints: number): T[] {
    if (data.length <= maxPoints) return data;

    const step = data.length / maxPoints;
    const result: T[] = [];
    let nextSample = 0;

    for (let i = 0; i < data.length; i++) {
        const point = data[i]!;
        const isGap = (point as Record<string, unknown>)._gap != null ||
            (point as Record<string, unknown>)._gapEnd === true;

        if (isGap || i >= nextSample) {
            result.push(point);
            if (!isGap) nextSample += step;
        }
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

    for (let i = 0; i < data.length; i++) {
        const point = data[i]!;
        result.push(point);

        if (i === data.length - 1) break;

        const nextPoint = data[i + 1]!;
        const currMs = Date.parse(point.time);
        const nextMs = Date.parse(nextPoint.time);

        if (nextMs - currMs > maxGapMs) {
            const gapStart = { ...point } as T;
            const gapEnd = { ...nextPoint } as T;

            for (const key of Object.keys(gapStart)) {
                if (key !== "time" && key !== "agent_id") {
                    (gapStart as Record<string, unknown>)[key] = null;
                    (gapEnd as Record<string, unknown>)[key] = null;
                }
            }

            gapStart.time = new Date(currMs + 1).toISOString();
            gapEnd.time = new Date(nextMs - 1).toISOString();

            (gapStart as Record<string, unknown>)._gap = {
                gapStart: point.time,
                gapEnd: nextPoint.time,
                gapMinutes: Math.round((nextMs - currMs) / 60_000),
            };
            (gapEnd as Record<string, unknown>)._gapEnd = true;

            result.push(gapStart);
            result.push(gapEnd);
        }
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
    raw: T[],
    sel: RangeSelection,
): T[] {
    const sorted = [...raw].reverse();
    const gapped = insertGaps(sorted, gapThreshold(sel));
    const downsampled = downsample(gapped, 500);

    if (downsampled.length === 0) return downsampled;

    const start = rangeStart(sel);
    const firstPoint = downsampled[0]!;
    const firstMs = Date.parse(firstPoint.time);
    const startMs = start.getTime();

    // If data starts after the range start, insert a leading gap.
    if (firstMs - startMs > gapThreshold(sel)) {
        const empty = { ...firstPoint } as T;
        for (const key of Object.keys(empty)) {
            if (key !== "time" && key !== "agent_id") {
                (empty as Record<string, unknown>)[key] = null;
            }
        }

        const startMarker = { ...empty };
        startMarker.time = start.toISOString();
        (startMarker as Record<string, unknown>)._gap = {
            gapStart: start.toISOString(),
            gapEnd: firstPoint.time,
            gapMinutes: Math.round((firstMs - startMs) / 60_000),
        };

        const gapEnd = { ...empty };
        gapEnd.time = new Date(firstMs - 1).toISOString();
        (gapEnd as Record<string, unknown>)._gapEnd = true;

        downsampled.unshift(gapEnd);
        downsampled.unshift(startMarker);
    }

    for (const point of downsampled) {
        (point as Record<string, unknown>)._ts = Date.parse(point.time);
    }

    return downsampled;
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