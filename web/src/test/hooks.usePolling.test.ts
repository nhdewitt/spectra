import { describe, it, expect, vi, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { usePolling } from '../hooks/usePolling'

afterEach(() => {
    vi.useRealTimers()
})

describe('usePolling', () => {
    it('fetches on mount and transitions from loading to loaded', async () => {
        const fetcher = vi.fn().mockResolvedValue({ value: 1 })
        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        expect(result.current.loading).toBe(true)

        await waitFor(() => expect(result.current.loading).toBe(false))
        expect(result.current.data).toEqual({ value: 1 })
        expect(result.current.error).toBeNull()
        expect(fetcher).toHaveBeenCalledTimes(1)
    })

    it('surfaces a string error message on rejection', async () => {
        const fetcher = vi.fn().mockRejectedValue(new Error('network down'))
        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await waitFor(() => expect(result.current.loading).toBe(false))
        expect(result.current.error).toBe('network down')
        expect(result.current.data).toBeNull()
    })

    it('falls back to a generic message for non-Error rejections', async () => {
        const fetcher = vi.fn().mockRejectedValue('boom')
        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await waitFor(() => expect(result.current.loading).toBe(false))
        expect(result.current.error).toBe('Failed to load')
    })

    it('polls again after intervalMs', async () => {
        vi.useFakeTimers()
        const fetcher = vi.fn().mockResolvedValue({ value: 1 })
        renderHook(() => usePolling(fetcher, 10_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(fetcher).toHaveBeenCalledTimes(1)

        await act(async () => {
            await vi.advanceTimersByTimeAsync(10_000)
        })
        expect(fetcher).toHaveBeenCalledTimes(2)

        vi.useRealTimers()
    })

    it('refetch() triggers an immediate fetch outside the interval schedule', async () => {
        vi.useFakeTimers()
        const fetcher = vi.fn().mockResolvedValue({ value: 1 })
        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(fetcher).toHaveBeenCalledTimes(1)

        await act(async () => {
            result.current.refetch()
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(fetcher).toHaveBeenCalledTimes(2)

        vi.useRealTimers()
    })

    it('picks up a new fetcher on the next poll without restarting the interval', async () => {
        vi.useFakeTimers()
        const fetcherA = vi.fn().mockResolvedValue({ value: 'a' })
        const fetcherB = vi.fn().mockResolvedValue({ value: 'b' })

        const { result, rerender } = renderHook(
            ({ fetcher }: { fetcher: () => Promise<{ value: string }> }) => usePolling(fetcher, 10_000),
            { initialProps: { fetcher: fetcherA } }
        )

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.data).toEqual({ value: 'a' })

        rerender({ fetcher: fetcherB })

        await act(async () => {
            await vi.advanceTimersByTimeAsync(10_000)
        })

        expect(fetcherB).toHaveBeenCalledTimes(1)
        expect(fetcherA).toHaveBeenCalledTimes(1) // never called again after the swap
        expect(result.current.data).toEqual({ value: 'b' })

        vi.useRealTimers()
    })

    it('does not update state after unmount', async () => {
        vi.useFakeTimers()
        let resolveFetch!: (v: unknown) => void
        const fetcher = vi.fn(() => new Promise((res) => { resolveFetch = res }))

        const { unmount } = renderHook(() => usePolling(fetcher, 10_000))
        unmount()

        resolveFetch({ value: 'late' })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        // The real assertion here is that resolving after unmount doesn't
        // throw or log a "state update on unmounted component" warning.
        expect(fetcher).toHaveBeenCalledTimes(1)

        vi.useRealTimers()
    })

    // Every tick starts a request whether or not the last one finished, so
    // two can be in flight at once. The older one must not win.
    it('ignores a slow first response that lands after a newer one', async () => {
        vi.useFakeTimers()

        const resolvers: Array<(v: unknown) => void> = []
        const fetcher = vi.fn(
            () => new Promise((res) => { resolvers.push(res as (v: unknown) => void) })
        )

        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(10_000)
        })
        expect(fetcher).toHaveBeenCalledTimes(2)

        // Second request answers first.
        await act(async () => {
            resolvers[1]!({ value: 'newer' })
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.data).toEqual({ value: 'newer' })

        // First request answers late and must be discarded.
        await act(async () => {
            resolvers[0]!({ value: 'older' })
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.data).toEqual({ value: 'newer' })

        vi.useRealTimers()
    })

    // refetch() runs alongside the interval rather than replacing it, which is
    // the other way two requests overlap.
    it('ignores an in-flight interval response once refetch supersedes it', async () => {
        vi.useFakeTimers()

        const resolvers: Array<(v: unknown) => void> = []
        const fetcher = vi.fn(
            () => new Promise((res) => { resolvers.push(res as (v: unknown) => void) })
        )

        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        act(() => {
            result.current.refetch()
        })
        expect(fetcher).toHaveBeenCalledTimes(2)

        await act(async () => {
            resolvers[1]!({ value: 'refetched' })
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.data).toEqual({ value: 'refetched' })

        await act(async () => {
            resolvers[0]!({ value: 'mount' })
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.data).toEqual({ value: 'refetched' })

        vi.useRealTimers()
    })

    // A stale rejection must not clear the error state belonging to a newer
    // request, or blank out a good result with a failure message.
    it('ignores a stale rejection', async () => {
        vi.useFakeTimers()

        const resolvers: Array<{ res: (v: unknown) => void; rej: (e: unknown) => void }> = []
        const fetcher = vi.fn(
            () => new Promise((res, rej) => {
                resolvers.push({ res: res as (v: unknown) => void, rej })
            })
        )

        const { result } = renderHook(() => usePolling(fetcher, 10_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(10_000)
        })

        await act(async () => {
            resolvers[1]!.res({ value: 'good' })
            await vi.advanceTimersByTimeAsync(0)
        })

        await act(async () => {
            resolvers[0]!.rej(new Error('stale failure'))
            await vi.advanceTimersByTimeAsync(0)
        })

        expect(result.current.error).toBeNull()
        expect(result.current.data).toEqual({ value: 'good' })

        vi.useRealTimers()
    })
})