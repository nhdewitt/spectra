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
 * The fetcher is called immediately on mount and again whenever it changes,
 * then every `intervalMs`. Callers memoize it with useCallback keyed on the
 * resource it reads, so a new agent, sort, or limit fetches at once rather
 * than on the next tick.
 * 
 * Every call takes a generation number and only writes state if it is still
 * the newest. A tick fires whether or not the previous one finished, and
 * refetch() starts another alongside both, so without this a slow reqwuest
 * can land after a fast one and overwrite fresher data. Changing the fetcher
 * or unmounting bumps the generation, so a late response for the previous
 * resource is discarded. The previous data stays up until the new response
 * lands. useMetric solves the same problem with an AbortController.
 */
export function usePolling<T>(
    fetcher: () => Promise<T>,
    intervalMs: number
): UsePollingResult<T> {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const genRef = useRef(0);

    const load = useCallback(async () => {
        const gen = ++genRef.current;
        const current = () => gen === genRef.current;

        try {
            const result = await fetcher();
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
    }, [fetcher]);

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