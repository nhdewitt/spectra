import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { ServicesTab } from '../components/ServicesTab'
import type { Service } from '../types'

vi.mock('../api', () => ({
    api: { agentServices: vi.fn() },
}))

import { api } from '../api'
const mockAgentServices = api.agentServices as ReturnType<typeof vi.fn>

function makeService(overrides: Partial<Service> = {}): Service {
    return {
        agent_id: 'agent-1',
        name: 'sshd',
        status: 'active',
        sub_status: 'running',
        updated_at: '',
        ...overrides,
    }
}

beforeEach(() => {
    mockAgentServices.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('ServicesTab', () => {
    it('shows a loading spinner, then renders rows once loaded', async () => {
        mockAgentServices.mockResolvedValue([makeService({ name: 'sshd' })])

        const { container } = render(<ServicesTab agentId="agent-1" />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('sshd')).toBeInTheDocument())
        expect(mockAgentServices).toHaveBeenCalledWith('agent-1')
    })

    it('shows an error message when the fetch fails', async () => {
        mockAgentServices.mockRejectedValue(new Error('connection refused'))

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows "No services match" for an empty result', async () => {
        mockAgentServices.mockResolvedValue([])

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() =>
            expect(screen.getByText('No services match the current filter.')).toBeInTheDocument()
        )
    })

    it('shows per-status counts on the filter buttons', async () => {
        mockAgentServices.mockResolvedValue([
            makeService({ name: 'a', status: 'active' }),
            makeService({ name: 'b', status: 'active' }),
            makeService({ name: 'c', status: 'failed' }),
            makeService({ name: 'd', status: 'inactive' }),
        ])

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('All (4)')).toBeInTheDocument())

        expect(screen.getByText('Active (2)')).toBeInTheDocument()
        expect(screen.getByText('Failed (1)')).toBeInTheDocument()
        expect(screen.getByText('Inactive (1)')).toBeInTheDocument()
    })

    it('filters by status when a filter button is clicked', async () => {
        mockAgentServices.mockResolvedValue([
            makeService({ name: 'a', status: 'active' }),
            makeService({ name: 'b', status: 'failed' }),
        ])

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('a')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Failed (1)'))

        await waitFor(() => expect(screen.queryByText('a')).not.toBeInTheDocument())
        expect(screen.getByText('b')).toBeInTheDocument()
    })

    it('filters by search text, case-insensitively', async () => {
        mockAgentServices.mockResolvedValue([
            makeService({ name: 'sshd' }),
            makeService({ name: 'nginx' }),
        ])

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('sshd')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Filter services...'), { target: { value: 'NGI' } })

        await waitFor(() => expect(screen.queryByText('sshd')).not.toBeInTheDocument())
        expect(screen.getByText('nginx')).toBeInTheDocument()
    })

    it('shows the sub_status text alongside each row', async () => {
        mockAgentServices.mockResolvedValue([makeService({ name: 'sshd', sub_status: 'running' })])

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('running')).toBeInTheDocument())
    })

    it('paginates when there are more than 20 results', async () => {
        const services = Array.from({ length: 25 }, (_, i) => makeService({ name: `svc-${i}` }))
        mockAgentServices.mockResolvedValue(services)

        render(<ServicesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('svc-0')).toBeInTheDocument(), { timeout: 5000 })

        expect(screen.queryByText('svc-24')).not.toBeInTheDocument()
        fireEvent.click(screen.getByText('Next →'))
        await waitFor(() => expect(screen.getByText('svc-24')).toBeInTheDocument(), { timeout: 5000 })
    })

    it('polls for fresh data every 30 seconds', async () => {
        vi.useFakeTimers()
        mockAgentServices.mockResolvedValue([makeService({ name: 'sshd', sub_status: 'running' })])

        render(<ServicesTab agentId="agent-1" />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(mockAgentServices).toHaveBeenCalledTimes(1)

        mockAgentServices.mockResolvedValue([makeService({ name: 'sshd', sub_status: 'failed' })])
        await act(async () => {
            await vi.advanceTimersByTimeAsync(30_000)
        })
        expect(mockAgentServices).toHaveBeenCalledTimes(2)
    })
})