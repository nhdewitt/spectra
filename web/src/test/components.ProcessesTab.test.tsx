import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ProcessesTab } from '../components/ProcessesTab'
import type { Process } from '../types'

const mockProcesses: Process[] = [
  { agent_id: 'a1', pid: 1, name: 'systemd', cpu_percent: 0.5, mem_percent: 1.2, mem_rss: 10485760, status: 'running', threads: 1, updated_at: '2026-01-01T00:00:00Z' },
  { agent_id: 'a1', pid: 101, name: 'nginx', cpu_percent: 25.3, mem_percent: 4.8, mem_rss: 52428800, status: 'running', threads: 4, updated_at: '2026-01-01T00:00:00Z' },
  { agent_id: 'a1', pid: 200, name: 'postgres', cpu_percent: 55.0, mem_percent: 60.0, mem_rss: 1073741824, status: 'waiting', threads: 12, updated_at: '2026-01-01T00:00:00Z' },
  { agent_id: 'a1', pid: 300, name: 'stress', cpu_percent: 95.0, mem_percent: 85.0, mem_rss: 2147483648, status: 'runnable', threads: 8, updated_at: '2026-01-01T00:00:00Z' },
]

vi.mock('../api', () => ({
    api: {
        agentProcesses: vi.fn(),
    },
}))

import { api } from '../api'
const mockAgentProcesses = api.agentProcesses as ReturnType<typeof vi.fn>

beforeEach(() => {
    mockAgentProcesses.mockReset()
})

describe('ProcessesTab', () => {
    it('shows loading spinner initially', () => {
        mockAgentProcesses.mockReturnValue(new Promise(() => {}))
        const { container } = render(<ProcessesTab agentId="a1" />)
        expect(container.querySelector('svg')).toBeInTheDocument()
    })

    it('renders process table after data loads', async () => {
        mockAgentProcesses.mockResolvedValue(mockProcesses)
        render(<ProcessesTab agentId="a1" />)

        await waitFor(() => {
            expect(screen.getByText('systemd')).toBeInTheDocument()
        })

        expect(screen.getByText('nginx')).toBeInTheDocument()
        expect(screen.getByText('postgres')).toBeInTheDocument()
        expect(screen.getByText('stress')).toBeInTheDocument()
    })

    it('shows PID, CPU%, Mem%, status, and threads', async () => {
        mockAgentProcesses.mockResolvedValue(mockProcesses)
        render(<ProcessesTab agentId="a1" />)

        await waitFor(() => {
            expect(screen.getByText('nginx')).toBeInTheDocument()
        })

        expect(screen.getByText('101')).toBeInTheDocument()         // PID
        expect(screen.getByText('25.3')).toBeInTheDocument()        // CPU%
        expect(screen.getAllByText('running')).toHaveLength(2)      // systemd + nginx
        expect(screen.getByText('waiting')).toBeInTheDocument()     // postgres
        expect(screen.getByText('runnable')).toBeInTheDocument()    // stress
    })

    it('formats memory with bytes', async () => {
        mockAgentProcesses.mockResolvedValue(mockProcesses)
        render(<ProcessesTab agentId="a1" />)

        await waitFor(() => {
            expect(screen.getByText('nginx')).toBeInTheDocument()
        })

        expect(screen.getByText(/50\.0 MB/)).toBeInTheDocument()    // mem_rss 52428800
    })

 it('shows empty message when no processes', async () => {
    mockAgentProcesses.mockResolvedValue([])
    render(<ProcessesTab agentId="a1" />)

    await waitFor(() => {
      expect(screen.getByText('No processes found.')).toBeInTheDocument()
    })
  })

  it('shows error message on fetch failure', async () => {
    mockAgentProcesses.mockRejectedValue(new Error('connection refused'))
    render(<ProcessesTab agentId="a1" />)

    await waitFor(() => {
      expect(screen.getByText(/connection refused/)).toBeInTheDocument()
    })
  })

  it('calls API with default sort and limit', async () => {
    mockAgentProcesses.mockResolvedValue(mockProcesses)
    render(<ProcessesTab agentId="a1" />)

    await waitFor(() => {
      expect(mockAgentProcesses).toHaveBeenCalledWith('a1', 'cpu', 20)
    })
  })

  it('changes sort option', async () => {
    vi.useFakeTimers()
    mockAgentProcesses.mockResolvedValue(mockProcesses)
    render(<ProcessesTab agentId="a1" />)

    await vi.advanceTimersByTimeAsync(0)

    mockAgentProcesses.mockClear()
    mockAgentProcesses.mockResolvedValue(mockProcesses)

    const sortSelect = screen.getAllByRole('combobox')[0]!
    fireEvent.change(sortSelect, { target: { value: 'memory' } })

    await vi.advanceTimersByTimeAsync(10_000)

    expect(mockAgentProcesses).toHaveBeenCalledWith('a1', 'memory', 20)

    vi.useRealTimers()
  })

  it('changes limit option', async () => {
    vi.useFakeTimers()
    mockAgentProcesses.mockResolvedValue(mockProcesses)
    render(<ProcessesTab agentId="a1" />)

    await vi.advanceTimersByTimeAsync(0) // resolve initial fetch

    mockAgentProcesses.mockClear()
    mockAgentProcesses.mockResolvedValue(mockProcesses)

    const limitSelect = screen.getAllByRole('combobox')[1]!
    fireEvent.change(limitSelect, { target: { value: '50' } })

    await vi.advanceTimersByTimeAsync(10_000) // advance past polling interval

    expect(mockAgentProcesses).toHaveBeenCalledWith('a1', 'cpu', 50)

    vi.useRealTimers()
  })
})
