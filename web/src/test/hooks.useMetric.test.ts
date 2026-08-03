import { describe, it, expect, vi, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useMetric } from '../hooks/useMetric'
import type { RangeSelection } from '../types'

interface Point {
    time: string
    value: number | null
}

// Same safety net as the other fake-timer test files: guarantee real
// timers are restored even if an assertion throws mid-test.
afterEach(() => {
    vi.useRealTimers()
})

function customRange(startIso: string, endIso: string): RangeSelection {
    return { type: 'custom', start: startIso, end: endIso }
}

describe('useMetric - basic fetch/loading/error', () => {
    it('fetches on mount and reflects loading -> loaded', async () => {
        const fetcher = vi.fn().mockResolvedValue([])
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        expect(result.current.loading).toBe(true)

        await waitFor(() => expect(result.current.loading).toBe(false))
        expect(result.current.error).toBeNull()
        expect(fetcher).toHaveBeenCalledTimes(1)
        expect(fetcher.mock.calls[0]![0]).toEqual(sel)
    })

    it('surfaces a string error message on rejection', async () => {
        const fetcher = vi.fn().mockRejectedValue(new Error('upstream failed'))
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))

        await waitFor(() => expect(result.current.loading).toBe(false))
        expect(result.current.error).toBe('upstream failed')
    })

    it('swallows AbortError silently rather than surfacing it as an error', async () => {
        const abortError = new DOMException('aborted', 'AbortError')
        const fetcher = vi.fn().mockRejectedValue(abortError)
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))

        // Give the rejection a tick to be handled.
        await act(async () => {
            await new Promise((r) => setTimeout(r, 10))
        })

        expect(result.current.error).toBeNull()
    })

    it('refetch() does not flip loading back to true', async () => {
        const fetcher = vi.fn().mockResolvedValue([])
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.loading).toBe(false))

        act(() => {
            result.current.refetch()
        })
        // Loading should never flip back to true for a background refetch.
        expect(result.current.loading).toBe(false)

        await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
        expect(result.current.loading).toBe(false)
    })
})

describe('useMetric - abort on range change', () => {
    it('discards a stale response from a superseded range', async () => {
        let resolveFirst!: (v: Point[]) => void
        const firstPromise = new Promise<Point[]>((res) => { resolveFirst = res })

        const fetcher = vi
            .fn()
            .mockImplementationOnce(() => firstPromise)
            .mockImplementationOnce(() =>
                Promise.resolve([{ time: '2026-01-01T00:30:00.000Z', value: 2 }])
            )

        const rangeA = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')
        const rangeB = customRange('2026-01-02T00:00:00.000Z', '2026-01-02T01:00:00.000Z')

        const { result, rerender } = renderHook(
            ({ sel }: { sel: RangeSelection }) => useMetric<Point>(fetcher, sel),
            { initialProps: { sel: rangeA } }
        )

        // First fetch is still in flight. Switching ranges should abort it
        // and fire a second fetch.
        act(() => {
            rerender({ sel: rangeB })
        })

        await waitFor(() => expect(result.current.data).toHaveLength(1))
        expect(result.current.data[0]!.value).toBe(2)

        // Now let the FIRST (stale, aborted) call resolve late.
        await act(async () => {
            resolveFirst([{ time: '2026-01-01T00:00:00.000Z', value: 999 }])
            await new Promise((r) => setTimeout(r, 10))
        })

        // Still reflects the second (fresh) response - the stale one was discarded.
        expect(result.current.data).toHaveLength(1)
        expect(result.current.data[0]!.value).toBe(2)
    })
})

describe('useMetric - polling', () => {
    it('does not poll when pollMs is 0 (default)', async () => {
        vi.useFakeTimers()
        const fetcher = vi.fn().mockResolvedValue([])
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        renderHook(() => useMetric<Point>(fetcher, sel))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(fetcher).toHaveBeenCalledTimes(1)

        await act(async () => {
            await vi.advanceTimersByTimeAsync(60_000)
        })
        expect(fetcher).toHaveBeenCalledTimes(1) // still just the initial fetch
    })

    it('polls again after pollMs without flipping loading back to true', async () => {
        vi.useFakeTimers()
        const fetcher = vi.fn().mockResolvedValue([])
        const sel = customRange('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel, 5_000))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(fetcher).toHaveBeenCalledTimes(1)
        expect(result.current.loading).toBe(false)

        await act(async () => {
            await vi.advanceTimersByTimeAsync(5_000)
        })
        expect(fetcher).toHaveBeenCalledTimes(2)
        expect(result.current.loading).toBe(false)
    })
})

describe('useMetric - gap insertion and downsampling (observed through hook output)', () => {
    it('attaches a numeric _ts to every returned point', async () => {
        const t0 = '2026-01-01T00:00:00.000Z'
        const fetcher = vi.fn().mockResolvedValue([{ time: t0, value: 1 }])
        const sel = customRange(t0, '2026-01-01T00:10:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.data.length).toBeGreaterThan(0))

        for (const point of result.current.data) {
            expect(typeof (point as unknown as { _ts: number })._ts).toBe('number')
        }
    })

    it('does not insert gaps for closely-spaced points', async () => {
        const t0 = new Date('2026-01-01T00:00:00.000Z')
        const points: Point[] = Array.from({ length: 5 }, (_, i) => ({
            time: new Date(t0.getTime() + i * 60_000).toISOString(), // 1 min apart
            value: i,
        }))
        const fetcher = vi.fn().mockResolvedValue(points)
        // custom range gap threshold is a flat 5 minutes - 1 minute spacing
        // and a range start matching the first point avoid any gap insertion.
        const sel = customRange(points[0]!.time, new Date(t0.getTime() + 10 * 60_000).toISOString())

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.data.length).toBeGreaterThan(0))

        expect(result.current.data).toHaveLength(5)
        expect(result.current.data.some((p) => '_gap' in (p as object))).toBe(false)
    })

    it('inserts gap markers between points separated by more than the gap threshold', async () => {
        const t0 = '2026-01-01T00:00:00.000Z'
        const t1 = '2026-01-01T00:20:00.000Z' // 20 minutes later, well over the 5-minute custom-range threshold
        const points: Point[] = [
            { time: t0, value: 1 },
            { time: t1, value: 2 },
        ]
        const fetcher = vi.fn().mockResolvedValue(points)
        const sel = customRange(t0, '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.data.length).toBeGreaterThan(0))

        // Original 2 points plus 2 synthetic gap markers inserted between them.
        expect(result.current.data).toHaveLength(4)
        const gapStart = result.current.data[1] as unknown as { _gap?: unknown; value: number | null }
        const gapEnd = result.current.data[2] as unknown as { _gapEnd?: boolean; value: number | null }
        expect(gapStart._gap).toBeDefined()
        expect(gapStart.value).toBeNull()
        expect(gapEnd._gapEnd).toBe(true)
        expect(gapEnd.value).toBeNull()
    })

    it('prepends a leading gap when data starts well after the requested range start', async () => {
        const rangeStart = '2026-01-01T00:00:00.000Z'
        const dataStart = '2026-01-01T00:20:00.000Z' // 20 minutes into the range
        const points: Point[] = [{ time: dataStart, value: 5 }]
        const fetcher = vi.fn().mockResolvedValue(points)
        const sel = customRange(rangeStart, '2026-01-01T01:00:00.000Z')

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.data.length).toBeGreaterThan(0))

        // A synthetic leading gap (2 markers) is prepended before the real point.
        expect(result.current.data).toHaveLength(3)
        const leadIn = result.current.data[0] as unknown as { _gap?: unknown; time: string; value: number | null }
        expect(leadIn._gap).toBeDefined()
        expect(leadIn.time).toBe(rangeStart)
        expect(leadIn.value).toBeNull()
        expect(result.current.data[2]).toMatchObject({ time: dataStart, value: 5 })
    })

    it('downsamples a large closely-spaced dataset while preserving the first and last points', async () => {
        const t0 = new Date('2026-01-01T00:00:00.000Z')
        const points: Point[] = Array.from({ length: 1000 }, (_, i) => ({
            time: new Date(t0.getTime() + i * 1_000).toISOString(), // 1 second apart - well under the gap threshold
            value: i,
        }))
        const fetcher = vi.fn().mockResolvedValue(points)
        const sel = customRange(points[0]!.time, points[points.length - 1]!.time)

        const { result } = renderHook(() => useMetric<Point>(fetcher, sel))
        await waitFor(() => expect(result.current.data.length).toBeGreaterThan(0))

        expect(result.current.data.length).toBeLessThan(600) // meaningfully reduced from 1000
        expect(result.current.data[0]).toMatchObject({ value: 0 })
        expect(result.current.data[result.current.data.length - 1]).toMatchObject({ value: 999 })
    })
})