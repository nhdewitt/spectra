import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { ContainersTab } from '../components/ContainersTab'
import type { ContainerMetric, RangeSelection } from '../types'

vi.mock('../api', () => ({
    api: { agentContainers: vi.fn() },
}))

vi.mock('../components/MetricChart', () => ({
    MetricChart: (props: { title: string; data: unknown[] }) => (
        <div data-testid={`chart-${props.title}`} data-points={props.data.length} />
    ),
}))

import { api } from '../api'
const mockAgentContainers = api.agentContainers as ReturnType<typeof vi.fn>

function makeContainer(overrides: Partial<ContainerMetric> = {}): ContainerMetric {
    return {
        time: '2026-01-01T00:00:00.000Z',
        agent_id: 'agent-1',
        container_id: 'c1',
        name: 'web',
        image: 'nginx:latest',
        state: 'running',
        source: 'docker',
        kind: 'docker',
        cpu_percent: 5,
        cpu_cores: 1,
        memory_bytes: 1024,
        memory_limit: 4096,
        net_rx_bytes: 100,
        net_tx_bytes: 200,
        ...overrides,
    }
}

const oneHour: RangeSelection = { type: 'quick', range: '1h' }

beforeEach(() => {
    mockAgentContainers.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('ContainersTab', () => {
    it('shows a loading spinner, then renders the container list', async () => {
        mockAgentContainers.mockResolvedValue([makeContainer({ name: 'web' })])

        const { container } = render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('web')).toBeInTheDocument())
        expect(mockAgentContainers).toHaveBeenCalledWith('agent-1', { type: 'quick', range: '1h' })
    })

    it('shows an error message when the fetch fails', async () => {
        mockAgentContainers.mockRejectedValue(new Error('connection refused'))

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows "No containers found." for an empty result', async () => {
        mockAgentContainers.mockResolvedValue([])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('No containers found.')).toBeInTheDocument())
    })

    it('collapses to the latest reading per container_id', async () => {
        mockAgentContainers.mockResolvedValue([
            makeContainer({ container_id: 'c1', name: 'web', time: '2026-01-01T00:00:00.000Z', cpu_percent: 1 }),
            makeContainer({ container_id: 'c1', name: 'web', time: '2026-01-01T00:05:00.000Z', cpu_percent: 9 }),
        ])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('web')).toBeInTheDocument())

        // Only the later reading's CPU value should show, and only one row.
        expect(screen.getByText('9.0%')).toBeInTheDocument()
        expect(screen.queryByText('1.0%')).not.toBeInTheDocument()
    })

    it('sorts running containers before non-running, then alphabetically', async () => {
        mockAgentContainers.mockResolvedValue([
            makeContainer({ container_id: 'c1', name: 'zeta', state: 'running' }),
            makeContainer({ container_id: 'c2', name: 'alpha', state: 'exited' }),
            makeContainer({ container_id: 'c3', name: 'beta', state: 'running' }),
        ])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('zeta')).toBeInTheDocument())

        const rows = screen.getAllByRole('row').slice(1) // skip header row
        const names = rows.map((r) => r.textContent)
        // beta and zeta (running, alphabetical) before alpha (exited)
        expect(names[0]).toContain('beta')
        expect(names[1]).toContain('zeta')
        expect(names[2]).toContain('alpha')
    })

    it('shows the running/total summary count', async () => {
        mockAgentContainers.mockResolvedValue([
            makeContainer({ container_id: 'c1', state: 'running' }),
            makeContainer({ container_id: 'c2', state: 'exited' }),
        ])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText(/1 of\s*2 running/)).toBeInTheDocument())
    })

    it('shows placeholder dashes for stats on non-running containers', async () => {
        mockAgentContainers.mockResolvedValue([
            makeContainer({ container_id: 'c1', name: 'stopped-one', state: 'exited', cpu_percent: 42 }),
        ])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('stopped-one')).toBeInTheDocument())

        expect(screen.queryByText('42.0%')).not.toBeInTheDocument()
        // Four stat columns (CPU, Memory, Net RX, Net TX) all show em dashes.
        expect(screen.getAllByText('—')).toHaveLength(4)
    })

    it('truncates long image names with a leading ellipsis', async () => {
        const longImage = 'registry.example.com/some/very/long/nested/path/image:v1.2.3-with-a-long-tag'
        mockAgentContainers.mockResolvedValue([makeContainer({ image: longImage })])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText(/^…/)).toBeInTheDocument())

        const shown = screen.getByText(/^…/).textContent!
        expect(shown.startsWith('…')).toBe(true)
        expect(longImage.endsWith(shown.slice(1))).toBe(true)
    })

    it('expands container detail charts on row click, and collapses on a second click', async () => {
        mockAgentContainers.mockResolvedValue([makeContainer({ container_id: 'c1', name: 'web' })])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await waitFor(() => expect(screen.getByText('web')).toBeInTheDocument())

        expect(screen.queryByTestId('chart-CPU')).not.toBeInTheDocument()

        fireEvent.click(screen.getAllByText('web')[0]!) // the table row's instance
        await waitFor(() => expect(screen.getByTestId('chart-CPU')).toBeInTheDocument())
        expect(screen.getByTestId('chart-Memory')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Network')).toBeInTheDocument()

        fireEvent.click(screen.getAllByText('web')[0]!) // still the table row, not the now-visible detail heading
        await waitFor(() => expect(screen.queryByTestId('chart-CPU')).not.toBeInTheDocument())
    })

    it('polls for fresh data every 15 seconds', async () => {
        vi.useFakeTimers()
        mockAgentContainers.mockResolvedValue([makeContainer({ name: 'web', cpu_percent: 1 })])

        render(<ContainersTab agentId="agent-1" rangeSel={oneHour} />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(mockAgentContainers).toHaveBeenCalledTimes(1)

        mockAgentContainers.mockResolvedValue([makeContainer({ name: 'web', cpu_percent: 2 })])
        await act(async () => {
            await vi.advanceTimersByTimeAsync(15_000)
        })
        expect(mockAgentContainers).toHaveBeenCalledTimes(2)
    })
})