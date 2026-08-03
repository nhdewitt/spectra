import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { UpdatesTab } from '../components/UpdatesTab'
import type { Updates } from '../types'

vi.mock('../api', () => ({
    api: { agentUpdates: vi.fn() },
}))

import { api } from '../api'
const mockAgentUpdates = api.agentUpdates as ReturnType<typeof vi.fn>

function makeUpdates(overrides: Partial<Updates> = {}): Updates {
    return {
        agent_id: 'agent-1',
        pending_count: 0,
        security_count: 0,
        reboot_required: false,
        package_manager: 'apt',
        updated_at: '2026-01-01T12:00:00.000Z',
        ...overrides,
    }
}

beforeEach(() => {
    mockAgentUpdates.mockReset()
})

describe('UpdatesTab', () => {
    it('shows a loading spinner, then renders once loaded', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates())

        const { container } = render(<UpdatesTab agentId="agent-1" />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('System is up to date.')).toBeInTheDocument())
        expect(mockAgentUpdates).toHaveBeenCalledWith('agent-1')
    })

    it('shows an error message when the fetch fails', async () => {
        mockAgentUpdates.mockRejectedValue(new Error('connection refused'))

        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows "up to date" when there are no pending updates', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ pending_count: 0 }))

        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('System is up to date.')).toBeInTheDocument())
    })

    it('shows the pending count, pluralized correctly', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ pending_count: 1 }))
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText(/^1 update available/)).toBeInTheDocument())
    })

    it('pluralizes "updates" for more than one pending', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ pending_count: 5 }))
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText(/^5 updates available/)).toBeInTheDocument())
    })

    it('mentions security updates when present', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ pending_count: 5, security_count: 2 }))
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText(/2 security updates/)).toBeInTheDocument())
    })

    it('shows the reboot-required stat as Yes/No', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ reboot_required: true }))
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('Yes')).toBeInTheDocument())
    })

    it('shows the package manager', async () => {
        mockAgentUpdates.mockResolvedValue(makeUpdates({ package_manager: 'dnf' }))
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() => expect(screen.getByText('dnf')).toBeInTheDocument())
    })

    it('shows "No update information available." when data is null-ish after load', async () => {
        // The component only reaches this branch if data stays null post-load,
        // which happens if the fetch never resolves with a value - simulate via
        // a resolved-but-never-called path isn't reachable from the public API,
        // so this exercises the loading->error path being absent and data absent.
        mockAgentUpdates.mockResolvedValue(undefined as unknown as Updates)
        render(<UpdatesTab agentId="agent-1" />)
        await waitFor(() =>
            expect(screen.getByText('No update information available.')).toBeInTheDocument()
        )
    })
})