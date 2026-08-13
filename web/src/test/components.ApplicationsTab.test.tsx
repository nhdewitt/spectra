import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ApplicationsTab } from '../components/ApplicationsTab'
import type { Application } from '../types'

vi.mock('../api', () => ({
    api: { agentApplications: vi.fn() },
}))

import { api } from '../api'
const mockAgentApplications = api.agentApplications as ReturnType<typeof vi.fn>

function makeApp(overrides: Partial<Application> = {}): Application {
    return { agent_id: 'agent-1', name: 'curl', version: '8.5.0', updated_at: '', ...overrides }
}

beforeEach(() => {
    mockAgentApplications.mockReset()
})

describe('ApplicationsTab', () => {
    it('shows a loading spinner, then renders rows once loaded', async () => {
        mockAgentApplications.mockResolvedValue([makeApp({ name: 'curl' })])

        const { container } = render(<ApplicationsTab agentId="agent-1" />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('curl')).toBeInTheDocument())
        expect(mockAgentApplications).toHaveBeenCalledWith('agent-1')
    })

    it('shows an error message when the fetch fails', async () => {
        mockAgentApplications.mockRejectedValue(new Error('connection refused'))

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows "No applications found." for an empty result', async () => {
        mockAgentApplications.mockResolvedValue([])

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('No applications found.')).toBeInTheDocument())
    })

    it('shows the package count', async () => {
        mockAgentApplications.mockResolvedValue([makeApp({ name: 'curl' }), makeApp({ name: 'git' })])

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('2 of 2 packages')).toBeInTheDocument())
    })

    it('filters by name or version, case-insensitively', async () => {
        mockAgentApplications.mockResolvedValue([
            makeApp({ name: 'curl', version: '8.5.0' }),
            makeApp({ name: 'git', version: '2.40.0' }),
        ])

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('curl')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Search applications...'), { target: { value: 'GIT' } })

        await waitFor(() => expect(screen.queryByText('curl')).not.toBeInTheDocument())
        expect(screen.getByText('git')).toBeInTheDocument()
        expect(screen.getByText('1 of 2 packages')).toBeInTheDocument()
    })

    it('shows a distinct empty-search message when the search excludes everything', async () => {
        mockAgentApplications.mockResolvedValue([makeApp({ name: 'curl' })])

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('curl')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Search applications...'), { target: { value: 'nonexistent' } })

        await waitFor(() =>
            expect(screen.getByText('No applications match your search.')).toBeInTheDocument()
        )
    })

    it('paginates when there are more than 20 results', async () => {
        const apps = Array.from({ length: 25 }, (_, i) => makeApp({ name: `pkg-${i}`, version: '1.0' }))
        mockAgentApplications.mockResolvedValue(apps)

        render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('pkg-0')).toBeInTheDocument(), { timeout: 5000 })

        expect(screen.getByText('Next →')).toBeInTheDocument()
        expect(screen.queryByText('pkg-24')).not.toBeInTheDocument() // on page 2

        fireEvent.click(screen.getByText('Next →'))
        await waitFor(() => expect(screen.getByText('pkg-24')).toBeInTheDocument(), { timeout: 5000 })
    }, 15000)

    it('refetches when agentId changes', async () => {
        mockAgentApplications.mockImplementation((id: string) =>
            Promise.resolve([makeApp({ name: `app-for-${id}` })])
        )

        const { rerender } = render(<ApplicationsTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('app-for-agent-1')).toBeInTheDocument())

        rerender(<ApplicationsTab agentId="agent-2" />)
        await waitFor(() => expect(screen.getByText('app-for-agent-2')).toBeInTheDocument())

        expect(mockAgentApplications).toHaveBeenCalledWith('agent-2')
    })
})