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
})