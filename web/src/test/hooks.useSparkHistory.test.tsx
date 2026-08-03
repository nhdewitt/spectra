import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useSparkHistory } from '../hooks/useSparkHistory'
import type { OverviewAgent } from '../types'

function makeAgent(overrides: Partial<OverviewAgent> = {}): OverviewAgent {
    return {
        id: 'a1',
        hostname: 'test-host-1',
        os: 'linux',
        platform: 'proxmox',
        arch: 'amd64',
        cpu_cores: 8,
        last_seen: new Date().toISOString(),
        version: '1.0.0',
        cpu_usage: 10,
        load_normalized: 0.1,
        ram_percent: 20,
        swap_percent: 0,
        disk_max_percent: 30,
        net_rx_bytes: 0,
        net_tx_bytes: 0,
        max_temp: 40,
        uptime: 100,
        process_count: 10,
        reboot_required: false,
        updated_at: new Date().toISOString(),
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

vi.mock('../api', () => ({
    api: {
        sparklines: vi.fn(),
    },
}))

import { api } from '../api'
const mockSparklines = api.sparklines as ReturnType<typeof vi.fn>

beforeEach(() => {
    mockSparklines.mockReset().mockResolvedValue({})
})

describe('useSparkHistory', () => {
    it('returns an empty map and makes no request when there are no agents', () => {
        const { result } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [] } }
        )

        expect(result.current.size).toBe(0)
        expect(mockSparklines).not.toHaveBeenCalled()
    })

    it('seeds history from api.sparklines() on first load', async () => {
        mockSparklines.mockResolvedValue({
            a1: { cpu: [1, 2, 3], mem: [4, 5, 6], disk: [7, 8, 9] },
        })

        const { result } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1', cpu_usage: 999 })] } }
        )

        // The mount-time append effect also runs and creates an entry first,
        // but the seed's async response resolves after and overwrites it via
        // Map.set (a full replace, not a merge) - the seeded values win.
        await waitFor(() =>
            expect(result.current.get('a1')).toEqual({ cpu: [1, 2, 3], mem: [4, 5, 6], disk: [7, 8, 9] })
        )
    })

    it('trims seeded history to maxPoints', async () => {
        mockSparklines.mockResolvedValue({
            a1: { cpu: [1, 2, 3, 4, 5], mem: [1, 2, 3, 4, 5], disk: [1, 2, 3, 4, 5] },
        })

        const { result } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents, 2),
            { initialProps: { agents: [makeAgent({ id: 'a1', last_seen: null })] } } // last_seen null: append effect skips, isolating the seed
        )

        await waitFor(() => expect(result.current.get('a1')).toBeDefined())
        expect(result.current.get('a1')).toEqual({ cpu: [4, 5], mem: [4, 5], disk: [4, 5] })
    })

    it('seeds only once, even as the agents array reference changes', async () => {
        mockSparklines.mockResolvedValue({ a1: { cpu: [1], mem: [1], disk: [1] } })

        const { rerender } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1' })] } }
        )

        await waitFor(() => expect(mockSparklines).toHaveBeenCalledTimes(1))

        act(() => { rerender({ agents: [makeAgent({ id: 'a1' })] }) }) // new array reference, same shape
        act(() => { rerender({ agents: [makeAgent({ id: 'a1' })] }) })

        expect(mockSparklines).toHaveBeenCalledTimes(1)
    })

    it('does not throw when the seed request fails', async () => {
        mockSparklines.mockRejectedValue(new Error('network down'))

        renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1' })] } }
        )

        await waitFor(() => expect(mockSparklines).toHaveBeenCalledTimes(1))
        // No assertion beyond "didn't throw" - a rejected seed should fall
        // back silently to accumulating from polling instead.
    })

    it('appends a data point on mount and on each subsequent poll', () => {
        const { result, rerender } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1', cpu_usage: 10, ram_percent: 20, disk_max_percent: 30 })] } }
        )

        // Mount itself already appended one point.
        expect(result.current.get('a1')).toEqual({ cpu: [10], mem: [20], disk: [30] })

        act(() => {
            rerender({ agents: [makeAgent({ id: 'a1', cpu_usage: 15, ram_percent: 25, disk_max_percent: 35 })] })
        })

        expect(result.current.get('a1')).toEqual({ cpu: [10, 15], mem: [20, 25], disk: [30, 35] })
    })

    it('caps accumulated history at maxPoints, dropping the oldest reading', () => {
        const { result, rerender } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents, 2),
            { initialProps: { agents: [makeAgent({ id: 'a1', cpu_usage: 1 })] } }
        )

        act(() => { rerender({ agents: [makeAgent({ id: 'a1', cpu_usage: 2 })] }) })
        act(() => { rerender({ agents: [makeAgent({ id: 'a1', cpu_usage: 3 })] }) })

        expect(result.current.get('a1')!.cpu).toEqual([2, 3])
    })

    it('never appends for an agent with no last_seen', () => {
        const { result, rerender } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1', last_seen: null })] } }
        )

        expect(result.current.has('a1')).toBe(false)

        act(() => { rerender({ agents: [makeAgent({ id: 'a1', last_seen: null })] }) })
        expect(result.current.has('a1')).toBe(false)
    })

    it('freezes the sparkline once an agent goes offline (last_seen >= 10m ago)', () => {
        const staleTimestamp = new Date(Date.now() - 700_000).toISOString() // ~11.6 minutes ago

        const { result, rerender } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            { initialProps: { agents: [makeAgent({ id: 'a1', last_seen: new Date().toISOString(), cpu_usage: 1 })] } }
        )

        act(() => {
            rerender({ agents: [makeAgent({ id: 'a1', last_seen: new Date().toISOString(), cpu_usage: 2 })] })
        })
        expect(result.current.get('a1')!.cpu).toEqual([1, 2])

        act(() => {
            rerender({ agents: [makeAgent({ id: 'a1', last_seen: staleTimestamp, cpu_usage: 99 })] })
        })
        // Still [1, 2] - the offline reading (99) never gets appended.
        expect(result.current.get('a1')!.cpu).toEqual([1, 2])
    })

    it('treats a null metric value as 0', () => {
        const { result } = renderHook(
            ({ agents }: { agents: OverviewAgent[] }) => useSparkHistory(agents),
            {
                initialProps: {
                    agents: [makeAgent({ id: 'a1', cpu_usage: null, ram_percent: null, disk_max_percent: null })],
                },
            }
        )

        expect(result.current.get('a1')).toEqual({ cpu: [0], mem: [0], disk: [0] })
    })
})