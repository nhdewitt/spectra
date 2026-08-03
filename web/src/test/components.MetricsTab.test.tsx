import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MetricsTab } from '../components/MetricsTab'
import type { DiskMetric, NetworkMetric, TemperatureMetric, RangeSelection } from '../types'

vi.mock('../api', () => ({
    api: {
        agentCPU: vi.fn().mockResolvedValue([]),
        agentMemory: vi.fn().mockResolvedValue([]),
        agentDisk: vi.fn().mockResolvedValue([]),
        agentDiskIO: vi.fn().mockResolvedValue([]),
        agentNetwork: vi.fn().mockResolvedValue([]),
        agentTemperature: vi.fn().mockResolvedValue([]),
        agentWifi: vi.fn().mockResolvedValue([]),
    },
}))

vi.mock('../components/PiPanels', () => ({
    PiPanels: () => <div data-testid="pi-panels" />,
}))

interface ChartCall {
    title: string
    data: unknown[]
    formatter?: (v: number, key: string) => string
    refLines?: { y: number; label?: string }[]
    series: { key: string; label: string }[]
}

let chartCalls: ChartCall[] = []

vi.mock('../components/MetricChart', () => ({
    MetricChart: (props: ChartCall) => {
        chartCalls.push(props)
        return <div data-testid={`chart-${props.title}`} data-points={props.data.length} />
    },
}))

import { api } from '../api'
const mocks = {
    cpu: api.agentCPU as ReturnType<typeof vi.fn>,
    memory: api.agentMemory as ReturnType<typeof vi.fn>,
    disk: api.agentDisk as ReturnType<typeof vi.fn>,
    diskIO: api.agentDiskIO as ReturnType<typeof vi.fn>,
    network: api.agentNetwork as ReturnType<typeof vi.fn>,
    temperature: api.agentTemperature as ReturnType<typeof vi.fn>,
    wifi: api.agentWifi as ReturnType<typeof vi.fn>,
}

const oneHour: RangeSelection = { type: 'quick', range: '1h' }

function chartFor(title: string): ChartCall {
    // Most recent call for that title reflects current state after any interaction.
    const calls = chartCalls.filter((c) => c.title === title)
    return calls[calls.length - 1]!
}

function makeDisk(overrides: Partial<DiskMetric> = {}): DiskMetric {
    return {
        time: '2026-01-01T00:00:00.000Z',
        agent_id: 'agent-1',
        device: 'sda1',
        mountpoint: '/',
        filesystem: 'ext4',
        disk_type: 'ssd',
        total_bytes: 1000,
        used_bytes: 500,
        free_bytes: 500,
        used_percent: 50,
        inodes_total: 100,
        inodes_used: 10,
        inodes_percent: 10,
        ...overrides,
    }
}

function makeNetwork(overrides: Partial<NetworkMetric> = {}): NetworkMetric {
    return {
        time: '2026-01-01T00:00:00.000Z',
        agent_id: 'agent-1',
        interface: 'eth0',
        mac: '',
        mtu: 1500,
        speed: 1000,
        rx_bytes: 100,
        rx_packets: 1,
        rx_errors: 0,
        rx_drops: 0,
        tx_bytes: 100,
        tx_packets: 1,
        tx_errors: 0,
        tx_drops: 0,
        ...overrides,
    }
}

function makeTemp(overrides: Partial<TemperatureMetric> = {}): TemperatureMetric {
    return {
        time: '2026-01-01T00:00:00.000Z',
        agent_id: 'agent-1',
        sensor: 'cpu',
        temperature: 40,
        max_temp: 90,
        ...overrides,
    }
}

beforeEach(() => {
    chartCalls = []
    Object.values(mocks).forEach((m) => {
        m.mockReset()
        m.mockResolvedValue([])
    })
})

describe('MetricsTab', () => {
    it('renders every panel, including the mocked PiPanels', async () => {
        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)

        await waitFor(() => expect(screen.getByTestId('chart-CPU')).toBeInTheDocument())
        expect(screen.getByTestId('chart-Load Average')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Memory')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Memory (Absolute)')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Disk Usage')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Disk I/O')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Network')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Temperature')).toBeInTheDocument()
        expect(screen.getByTestId('pi-panels')).toBeInTheDocument()
    })

    it('passes a reference line at the core count to the Load Average chart', async () => {
        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={8} />)
        await waitFor(() => expect(chartFor('Load Average')).toBeDefined())

        expect(chartFor('Load Average').refLines).toEqual([
            { y: 8, label: '8 cores', color: expect.anything() },
        ])
    })

    it('DiskPanel: auto-selects the first mount and filters data to it', async () => {
        mocks.disk.mockResolvedValue([
            makeDisk({ mountpoint: '/', used_percent: 50 }),
            makeDisk({ mountpoint: '/data', used_percent: 80 }),
        ])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Disk Usage').data.length).toBeGreaterThan(0))

        expect(chartFor('Disk Usage').data).toHaveLength(1)
        expect((chartFor('Disk Usage').data[0] as DiskMetric).mountpoint).toBe('/')
    })

    it('DiskPanel: switching mounts via the selector refilters the chart data', async () => {
        mocks.disk.mockResolvedValue([
            makeDisk({ mountpoint: '/', used_percent: 50 }),
            makeDisk({ mountpoint: '/data', used_percent: 80 }),
        ])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(screen.getByText('Mount:')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('/'), { target: { value: '/data' } })

        await waitFor(() =>
            expect((chartFor('Disk Usage').data[0] as DiskMetric).mountpoint).toBe('/data')
        )
    })

    it('DiskPanel: formatter shows free/total bytes for the active mount, and a bare percentage otherwise', async () => {
        mocks.disk.mockResolvedValue([makeDisk({ mountpoint: '/', used_percent: 50, free_bytes: 512, total_bytes: 1024 })])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Disk Usage').data.length).toBeGreaterThan(0))

        const formatter = chartFor('Disk Usage').formatter!
        expect(formatter(50, '/')).toBe('50.0% (512.0 B free of 1.0 KB)')
        expect(formatter(50, 'unrelated-key')).toBe('50.0')
    })

    it('does not show the mount selector when there is only one mount', async () => {
        mocks.disk.mockResolvedValue([makeDisk({ mountpoint: '/' })])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Disk Usage').data.length).toBeGreaterThan(0))

        expect(screen.queryByText('Mount:')).not.toBeInTheDocument()
    })

    it('NetworkPanel: auto-selects the interface with the most total traffic', async () => {
        mocks.network.mockResolvedValue([
            makeNetwork({ interface: 'eth0', rx_bytes: 10, tx_bytes: 10 }), // 20 total
            makeNetwork({ interface: 'wlan0', rx_bytes: 500, tx_bytes: 500 }), // 1000 total
        ])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Network').data.length).toBeGreaterThan(0))

        expect((chartFor('Network').data[0] as NetworkMetric).interface).toBe('wlan0')
    })

    it('WifiPanel: renders nothing when there is no wifi data', async () => {
        mocks.wifi.mockResolvedValue([])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(screen.getByTestId('chart-CPU')).toBeInTheDocument())

        expect(screen.queryByTestId('chart-WiFi Signal')).not.toBeInTheDocument()
    })

    it('WifiPanel: renders the chart when wifi data is present', async () => {
        mocks.wifi.mockResolvedValue([{ time: '2026-01-01T00:00:00.000Z', signal_dbm: -50, noise_dbm: -90 }])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(screen.getByTestId('chart-WiFi Signal')).toBeInTheDocument())
    })

    it('TemperaturePanel: pivots readings into one row per sensor per timestamp', async () => {
        mocks.temperature.mockResolvedValue([
            makeTemp({ sensor: 'cpu', temperature: 40, time: '2026-01-01T00:00:00.000Z' }),
            makeTemp({ sensor: 'gpu', temperature: 35, time: '2026-01-01T00:00:00.000Z' }),
        ])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Temperature').data.length).toBeGreaterThan(0))

        const row = chartFor('Temperature').data[0] as Record<string, unknown>
        expect(row.cpu).toBe(40)
        expect(row.gpu).toBe(35)
        expect(chartFor('Temperature').series.map((s) => s.key).sort()).toEqual(['cpu', 'gpu'])
    })

    it('TemperaturePanel: forward-fills a sensor reading that is missing at a later timestamp', async () => {
        mocks.temperature.mockResolvedValue([
            makeTemp({ sensor: 'cpu', temperature: 40, time: '2026-01-01T00:00:00.000Z' }),
            makeTemp({ sensor: 'gpu', temperature: 35, time: '2026-01-01T00:00:00.000Z' }),
            // Only cpu reports at the next timestamp - gpu should carry forward at 35.
            makeTemp({ sensor: 'cpu', temperature: 45, time: '2026-01-01T00:00:05.000Z' }),
        ])

        render(<MetricsTab agentId="agent-1" rangeSel={oneHour} cores={4} />)
        await waitFor(() => expect(chartFor('Temperature').data.length).toBeGreaterThanOrEqual(2))

        const rows = chartFor('Temperature').data as Record<string, unknown>[]
        const secondRow = rows.find((r) => r.cpu === 45)!
        expect(secondRow.gpu).toBe(35) // forward-filled, not missing
    })
})