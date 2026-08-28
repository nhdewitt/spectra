import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Overview } from '../pages/Overview'
import type { OverviewAgent } from '../types'
import type { OverviewPageResponse, OverviewStats } from '../api'

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
        cpu_usage: 12.5,
        load_normalized: 0.3,
        ram_percent: 40,
        swap_percent: 0,
        disk_max_percent: 55,
        net_rx_bytes: 1024,
        net_tx_bytes: 2048,
        max_temp: 45,
        uptime: 3600,
        process_count: 120,
        reboot_required: false,
        updated_at: new Date().toISOString(),
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

function page(overrides: Partial<OverviewPageResponse> = {}): OverviewPageResponse {
    return { agents: [], page: 1, size: 25, ...overrides }
}

const emptyStats: OverviewStats = { total: 0, online: 0, warn: 0, crit: 0, stale: 0, offline: 0, reboot: 0 }

vi.mock('../api', () => ({
    api: {
        overviewPage: vi.fn(),
        labelValues: vi.fn(),
        labelKeys: vi.fn(),
        sparklines: vi.fn(),
    },
}))

import { api } from '../api'
const mockOverviewPage = api.overviewPage as ReturnType<typeof vi.fn>
const mockLabelValues = api.labelValues as ReturnType<typeof vi.fn>
const mockLabelKeys = api.labelKeys as ReturnType<typeof vi.fn>
const mockSparklines = api.sparklines as ReturnType<typeof vi.fn>

const noop = () => {}

async function flush() {
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
}

beforeEach(() => {
    mockOverviewPage.mockReset()
    mockLabelValues.mockReset().mockResolvedValue([])
    mockLabelKeys.mockReset().mockResolvedValue([])
    mockSparklines.mockReset().mockResolvedValue({})
})

afterEach(() => {
    vi.useRealTimers()
})

describe('Overview', () => {
    it('shows a loading spinner, then renders agent rows', async () => {
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent({ id: 'a1', hostname: 'test-host-1' })], total: 1, total_pages: 1 })
        )

        const { container } = render(
            <Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />
        )
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())
    })

    it('fetches the first page with count requested on mount', async () => {
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)

        await waitFor(() =>
            expect(mockOverviewPage).toHaveBeenCalledWith(
                expect.objectContaining({ page: 1, size: 25, count: true, sort: 'severity', order: 'desc' })
            )
        )
    })

    it('shows an error message when the fetch fails', async () => {
        mockOverviewPage.mockRejectedValue(new Error('connection refused'))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)

        await waitFor(() => expect(screen.getByText(/connection refused/)).toBeInTheDocument())
    })

    it('shows "no agents registered" when the fleet itself is empty', async () => {
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={emptyStats} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)

        await waitFor(() => expect(screen.getByText(/No agents registered/)).toBeInTheDocument())
    })

    it('shows "no agents match filters" when the fleet has agents but the page is empty', async () => {
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        const stats: OverviewStats = { ...emptyStats, total: 50, online: 45, crit: 2, stale: 3 }

        render(<Overview stats={stats} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)

        await waitFor(() => expect(screen.getByText(/No agents match the current filters/)).toBeInTheDocument())
    })

    it('debounces search input before fetching', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        fireEvent.change(screen.getByPlaceholderText('Filter by hostname...'), {
            target: { value: 'test-host' },
        })
        await flush() // let the debounce effect (re)schedule its timer

        await vi.advanceTimersByTimeAsync(200)
        expect(mockOverviewPage).not.toHaveBeenCalled()

        await vi.advanceTimersByTimeAsync(150) // past the 300ms debounce
        await flush()
        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ search: 'test-host', page: 1, count: true })
        )

        vi.useRealTimers()
    })

    it('paginates without recounting, then recounts and resets to page 1 on a filter change', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent()], total: 60, total_pages: 3 })
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        // Page nav: count should be false, page should advance, and the
        // cached total (60) should still be displayed even though this
        // response omits total/total_pages entirely.
        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()] }))

        fireEvent.click(screen.getByText('Next →'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ page: 2, count: false })
        )
        expect(screen.getByText(/of 60/)).toBeInTheDocument()

        // Filter change: should reset to page 1 and request a recount.
        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [], total: 3, total_pages: 1 }))

        fireEvent.change(screen.getByDisplayValue('All Status'), { target: { value: 'crit' } })
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ page: 1, status: 'crit', count: true })
        )

        vi.useRealTimers()
    })

    it('polls in the background without requesting a recount', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent({ hostname: 'test-host-1' })], total: 5, total_pages: 1 })
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent({ hostname: 'test-host-1' })] }))

        await vi.advanceTimersByTimeAsync(10_000)
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ page: 1, count: false })
        )

        vi.useRealTimers()
    })

    it('sends the hardware filter as a label param, not a top-level field', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        mockLabelValues.mockImplementation((key: string) =>
            Promise.resolve(key === 'hardware' ? ['raspberry-pi', 'virtual-machine'] : [])
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()
        expect(screen.getByDisplayValue('All Hardware')).toBeInTheDocument()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        fireEvent.change(screen.getByDisplayValue('All Hardware'), {
            target: { value: 'raspberry-pi' },
        })
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({
                page: 1,
                count: true,
                labels: [{ key: 'hardware', value: 'raspberry-pi' }],
            })
        )
        // hardware must not leak out as its own query param
        expect(mockOverviewPage).not.toHaveBeenCalledWith(
            expect.objectContaining({ hardware: expect.anything() })
        )

        vi.useRealTimers()
    })

    it('adds a user label filter via the picker, fetching values on demand', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        mockLabelKeys.mockResolvedValue([{ key: 'environment', source: 'user' }])
        mockLabelValues.mockImplementation((key: string) =>
            Promise.resolve(key === 'environment' ? ['prod', 'staging'] : [])
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()
        expect(screen.getByDisplayValue('+ Add filter')).toBeInTheDocument()

        fireEvent.change(screen.getByDisplayValue('+ Add filter'), {
            target: { value: 'environment' },
        })
        await flush()
        expect(screen.getByDisplayValue('choose value')).toBeInTheDocument()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        fireEvent.change(screen.getByDisplayValue('choose value'), { target: { value: 'prod' } })
        fireEvent.click(screen.getByText('Add'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({
                page: 1,
                count: true,
                labels: [{ key: 'environment', value: 'prod' }],
            })
        )

        vi.useRealTimers()
    })

    it('resets to page 1 and recounts on os/arch/pageSize changes', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent()], total: 60, total_pages: 3 })
        )
        mockLabelValues.mockImplementation((key: string) => {
            if (key === 'os') return Promise.resolve(['linux', 'windows'])
            if (key === 'arch') return Promise.resolve(['amd64', 'arm64'])
            return Promise.resolve([])
        })

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        async function goToPage2AndChange(getSelect: () => HTMLElement, value: string, extra: Record<string, unknown>) {
            mockOverviewPage.mockClear()
            mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 60, total_pages: 3 }))
            fireEvent.click(screen.getByText('Next →'))
            await flush()
            expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ page: 2, count: false }))

            mockOverviewPage.mockClear()
            mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 60, total_pages: 3 }))
            fireEvent.change(getSelect(), { target: { value } })
            await flush()
            expect(mockOverviewPage).toHaveBeenCalledWith(
                expect.objectContaining({ page: 1, count: true, ...extra })
            )
        }

        await goToPage2AndChange(() => screen.getByDisplayValue('All OS'), 'linux', { os: 'linux' })
        await goToPage2AndChange(() => screen.getByDisplayValue('All Arch'), 'arm64', { arch: 'arm64' })
        await goToPage2AndChange(() => screen.getByDisplayValue('25'), '50', { size: 50 })

        vi.useRealTimers()
    })

    // Column headers are the sorting mechanism; the sort dropdown is gone.
    // Anchored at BOTH ends: "CPU" is a prefix of "CPU Trend", so a leading-only
    // anchor matches two columns.
    function exactly(label: string) {
        return new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`, 'i')
    }

    function sortHeader(label: string) {
        return screen.getByRole('button', { name: exactly(label) })
    }

    it('sends each column its default direction on first click', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        // Names sort alphabetically; metrics put the worst case first.
        const cases: Array<[string, string, 'asc' | 'desc']> = [
            ['Status', 'status', 'asc'],
            ['Hostname', 'hostname', 'asc'],
            ['OS / Platform', 'os', 'asc'],
            ['CPU', 'cpu', 'desc'],
            ['Memory', 'memory', 'desc'],
            ['Disk', 'disk', 'desc'],
            ['Temp', 'temp', 'desc'],
            ['Uptime', 'uptime', 'desc'],
            ['Last Seen', 'last_seen', 'desc'],
            ['Procs', 'procs', 'desc'],
            ['Net RX/TX', 'net', 'desc'],
        ]

        for (const [label, sort, order] of cases) {
            mockOverviewPage.mockClear()
            mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

            fireEvent.click(sortHeader(label))
            await flush()

            expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort, order }))
        }

        vi.useRealTimers()
    })

    it('cycles a column default -> reversed -> back to severity', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        async function clickCPU() {
            mockOverviewPage.mockClear()
            mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
            fireEvent.click(sortHeader('CPU'))
            await flush()
        }

        await clickCPU()
        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort: 'cpu', order: 'desc' }))

        await clickCPU()
        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort: 'cpu', order: 'asc' }))

        // Third click clears back to the resting sort. Severity has no column
        // of its own, so without this it would be unreachable.
        await clickCPU()
        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort: 'severity', order: 'desc' }))

        vi.useRealTimers()
    })

    it('starts a new column at its own default rather than inheriting the current direction', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.click(sortHeader('CPU'))
        await flush()
        fireEvent.click(sortHeader('CPU')) // now cpu/asc
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        fireEvent.click(sortHeader('Hostname'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort: 'hostname', order: 'asc' }))

        vi.useRealTimers()
    })

    it('marks the active column with aria-sort and leaves the rest at none', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.click(sortHeader('CPU'))
        await flush()

        expect(screen.getByRole('columnheader', { name: exactly('CPU') })).toHaveAttribute('aria-sort', 'descending')
        expect(screen.getByRole('columnheader', { name: exactly('Disk') })).toHaveAttribute('aria-sort', 'none')

        // CPU Trend is a separate, unsortable column - it must not have picked
        // up aria-sort just by sharing a name prefix with CPU.
        expect(screen.getByRole('columnheader', { name: exactly('CPU Trend') })).not.toHaveAttribute('aria-sort')

        fireEvent.click(sortHeader('CPU'))
        await flush()
        expect(screen.getByRole('columnheader', { name: exactly('CPU') })).toHaveAttribute('aria-sort', 'ascending')

        vi.useRealTimers()
    })

    it('returns to the first page when the sort changes', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 60, total_pages: 3 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.click(screen.getByText('Next →'))
        await flush()
        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ page: 2 }))

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 60, total_pages: 3 }))
        fireEvent.click(sortHeader('CPU'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ page: 1, sort: 'cpu' }))

        vi.useRealTimers()
    })

    it('has no sort dropdown', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        expect(screen.queryByDisplayValue(/^Sort:/)).not.toBeInTheDocument()

        vi.useRealTimers()
    })

    it('pins cards to hostname ascending regardless of the table sort', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 1, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.click(sortHeader('CPU'))
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 1, total_pages: 1 }))
        fireEvent.click(screen.getByText('⊞ Cards'))
        await flush()

        // Sorting a grid by a metric makes tiles jump position on every poll,
        // so cards ignore the table's sort entirely.
        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ sort: 'hostname', order: 'asc' })
        )

        vi.useRealTimers()
    })

    it('restores the table sort when switching back from cards', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 1, total_pages: 1 }))

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.click(sortHeader('CPU'))
        await flush()
        fireEvent.click(screen.getByText('⊞ Cards'))
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ agents: [makeAgent()], total: 1, total_pages: 1 }))
        fireEvent.click(screen.getByText('☰ Table'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ sort: 'cpu', order: 'desc' }))

        vi.useRealTimers()
    })

    it('removes a label filter and clears all label filters', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        mockLabelKeys.mockResolvedValue([
            { key: 'environment', source: 'user' },
            { key: 'team', source: 'user' },
        ])
        mockLabelValues.mockImplementation((key: string) => {
            if (key === 'environment') return Promise.resolve(['prod', 'staging'])
            if (key === 'team') return Promise.resolve(['sre', 'infra'])
            return Promise.resolve([])
        })

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.change(screen.getByDisplayValue('+ Add filter'), { target: { value: 'environment' } })
        await flush()
        fireEvent.change(screen.getByDisplayValue('choose value'), { target: { value: 'prod' } })
        fireEvent.click(screen.getByText('Add'))
        await flush()

        fireEvent.change(screen.getByDisplayValue('+ Add filter'), { target: { value: 'team' } })
        await flush()
        fireEvent.change(screen.getByDisplayValue('choose value'), { target: { value: 'sre' } })
        fireEvent.click(screen.getByText('Add'))
        await flush()

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        fireEvent.click(screen.getByTitle('Remove environment'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ page: 1, count: true, labels: [{ key: 'team', value: 'sre' }] })
        )

        mockOverviewPage.mockClear()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))

        fireEvent.click(screen.getByText('Clear all'))
        await flush()

        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ page: 1, count: true, labels: [] })
        )

        vi.useRealTimers()
    })

    it('combines the hardware filter and a user label filter in the same request', async () => {
        vi.useFakeTimers()
        mockOverviewPage.mockResolvedValue(page({ total: 0, total_pages: 1 }))
        mockLabelKeys.mockResolvedValue([{ key: 'environment', source: 'user' }])
        mockLabelValues.mockImplementation((key: string) => {
            if (key === 'hardware') return Promise.resolve(['raspberry-pi'])
            if (key === 'environment') return Promise.resolve(['prod'])
            return Promise.resolve([])
        })

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await flush()

        fireEvent.change(screen.getByDisplayValue('All Hardware'), { target: { value: 'raspberry-pi' } })
        await flush()

        fireEvent.change(screen.getByDisplayValue('+ Add filter'), { target: { value: 'environment' } })
        await flush()
        fireEvent.change(screen.getByDisplayValue('choose value'), { target: { value: 'prod' } })
        fireEvent.click(screen.getByText('Add'))
        await flush()

        const lastCall = mockOverviewPage.mock.calls.at(-1)![0]
        expect(lastCall.labels).toEqual(
            expect.arrayContaining([
                { key: 'hardware', value: 'raspberry-pi' },
                { key: 'environment', value: 'prod' },
            ])
        )
        expect(lastCall.labels).toHaveLength(2)

        vi.useRealTimers()
    })

    it('switches to card view and still renders agent data', async () => {
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent({ hostname: 'test-host-2' })], total: 1, total_pages: 1 })
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)
        await waitFor(() => expect(screen.getByText('test-host-2')).toBeInTheDocument())
        expect(screen.getByRole('table')).toBeInTheDocument()

        fireEvent.click(screen.getByText('⊞ Cards'))

        expect(screen.getByText('test-host-2')).toBeInTheDocument()
        expect(screen.queryByRole('table')).not.toBeInTheDocument()
    })

    it('calls onSelectAgent on row click, and onToggleStar without triggering select when the star is clicked', async () => {
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent({ id: 'a1', hostname: 'test-host-3' })], total: 1, total_pages: 1 })
        )
        const onSelectAgent = vi.fn()
        const onToggleStar = vi.fn()

        render(
            <Overview stats={null} onSelectAgent={onSelectAgent} starredIds={['a1']} onToggleStar={onToggleStar} />
        )
        await waitFor(() => expect(screen.getByText('test-host-3')).toBeInTheDocument())
        expect(screen.getByTitle('Remove from quick access')).toBeInTheDocument()

        fireEvent.click(screen.getByTitle('Remove from quick access'))
        expect(onToggleStar).toHaveBeenCalledWith('a1')
        expect(onSelectAgent).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('test-host-3'))
        expect(onSelectAgent).toHaveBeenCalledWith(expect.objectContaining({ id: 'a1' }))
    })

    it('shows the reboot-required badge', async () => {
        mockOverviewPage.mockResolvedValue(
            page({ agents: [makeAgent({ reboot_required: true })], total: 1, total_pages: 1 })
        )

        render(<Overview stats={null} onSelectAgent={noop} starredIds={[]} onToggleStar={noop} />)

        await waitFor(() => expect(screen.getByText('REBOOT')).toBeInTheDocument())
    })
})