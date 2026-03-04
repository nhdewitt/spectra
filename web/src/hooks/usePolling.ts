import { useState, useEffect, useCallback, useRef } from "react";

interface UsePollingResult<T> {
    data: T | null;
    loading: boolean;
    error: string | null;
    refetch: () => void;
}

/**
 * Poll an async fetcher on a fixed interval with cleanup.
 * 
 * Handles loading state, errors, and stale-closure prevention.
 * The fetcher is called immediately on mount, then every `intervalMs`.
 * Unmouinting cancels in-progress updates.
 */
export function usePolling<T>(
    fetcher: () => Promise<T>,
    intervalMs: number
): UsePollingResult<T> {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const fetcherRef = useRef(fetcher);
    fetcherRef.current = fetcher;

    const load = useCallback(async (signal: { cancelled: boolean }) => {
        try {
            const result = await fetcherRef.current();
            if (!signal.cancelled) {
                setData(result);
                setError(null);
            }
        } catch (err) {
            if (!signal.cancelled) {
                setError(err instanceof Error ? err.message : "Failed to load");
            }
        } finally {
            if (!signal.cancelled) {
                setLoading(false);
            }
        }
    }, []);

    const refetch = useCallback(() => {
        load({ cancelled: false });
    }, [load]);

    useEffect(() => {
        const signal = { cancelled: false };
        load(signal);
        const id = setInterval(() => load(signal), intervalMs);
        return () => {
            signal.cancelled = true;
            clearInterval(id);
        };
    }, [load, intervalMs]);

    return { data, loading, error, refetch };
}