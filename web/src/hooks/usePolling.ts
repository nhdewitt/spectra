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
 * Unmounting cancels in-progress updates.
 * 
 * Every call takes a generation number and only writes state if it is
 * still the newest. A tick fires whether or not the previous one finished,
 * and refetch() starts another alongside both, so without this a slow
 * request can land after a fast one and overwrite fresher data. useMetric
 * solves the same problem with an AbortController.
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
    const genRef = useRef(0);

    const load = useCallback(async () => {
        const gen = ++genRef.current;
        const current = () => gen === genRef.current;

        try {
            const result = await fetcherRef.current();
            if (current()) {
                setData(result);
                setError(null);
            }
        } catch (err) {
            if (current()) {
                setError(err instanceof Error ? err.message : "Failed to load");
            }
        } finally {
            if (current()) {
                setLoading(false);
            }
        }
    }, []);

    const refetch = useCallback(() => {
        void load();
    }, [load]);

    useEffect(() => {
        void load();
        const id = setInterval(() => void load(), intervalMs);
        return () => {
            genRef.current++;
            clearInterval(id);
        };
    }, [load, intervalMs]);

    return { data, loading, error, refetch };
}