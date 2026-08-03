import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { AgentLabelChips } from '../components/AgentLabelChips'
import type { AgentLabel, LabelKey } from '../types'

vi.mock('../api', () => ({
    api: {
        agentLabels: vi.fn(),
        labelKeys: vi.fn(),
        labelValues: vi.fn(),
        setAgentLabel: vi.fn(),
        deleteAgentLabel: vi.fn(),
    },
}))

import { api } from '../api'
const mockAgentLabels = api.agentLabels as ReturnType<typeof vi.fn>
const mockLabelKeys = api.labelKeys as ReturnType<typeof vi.fn>
const mockLabelValues = api.labelValues as ReturnType<typeof vi.fn>
const mockSetAgentLabel = api.setAgentLabel as ReturnType<typeof vi.fn>
const mockDeleteAgentLabel = api.deleteAgentLabel as ReturnType<typeof vi.fn>

function makeLabel(overrides: Partial<AgentLabel> = {}): AgentLabel {
    return { key: 'os', value: 'linux', source: 'auto', updated_at: '', ...overrides }
}

beforeEach(() => {
    mockAgentLabels.mockReset().mockResolvedValue([])
    mockLabelKeys.mockReset().mockResolvedValue([])
    mockLabelValues.mockReset().mockResolvedValue([])
    mockSetAgentLabel.mockReset()
    mockDeleteAgentLabel.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('AgentLabelChips', () => {
    it('renders nothing while loading', () => {
        mockAgentLabels.mockReturnValue(new Promise(() => {})) // never resolves
        const { container } = render(<AgentLabelChips agentId="agent-1" isAdmin={false} />)
        expect(container.firstChild).toBeNull()
    })

    it('renders nothing for a non-admin when there are no labels', async () => {
        mockAgentLabels.mockResolvedValue([])
        const { container } = render(<AgentLabelChips agentId="agent-1" isAdmin={false} />)
        await waitFor(() => expect(mockAgentLabels).toHaveBeenCalled())
        expect(container.firstChild).toBeNull()
    })

    it('renders labels sorted with auto labels first, then alphabetically by key', async () => {
        mockAgentLabels.mockResolvedValue([
            makeLabel({ key: 'zeta', source: 'user' }),
            makeLabel({ key: 'os', source: 'auto' }),
            makeLabel({ key: 'arch', source: 'auto' }),
            makeLabel({ key: 'alpha', source: 'user' }),
        ])

        render(<AgentLabelChips agentId="agent-1" isAdmin={false} />)
        await waitFor(() => expect(screen.getByText('os')).toBeInTheDocument())

        const keys = screen.getAllByText(/^(zeta|os|arch|alpha)$/).map((el) => el.textContent)
        expect(keys).toEqual(['arch', 'os', 'alpha', 'zeta'])
    })

    it('does not show the Add button or delete controls for a non-admin', async () => {
        mockAgentLabels.mockResolvedValue([makeLabel({ key: 'team', source: 'user' })])

        render(<AgentLabelChips agentId="agent-1" isAdmin={false} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        expect(screen.queryByText('+ Add')).not.toBeInTheDocument()
        expect(screen.queryByTitle('Remove team')).not.toBeInTheDocument()
    })

    it('lets an admin delete a user label but not an auto label', async () => {
        mockAgentLabels.mockResolvedValue([
            makeLabel({ key: 'os', source: 'auto' }),
            makeLabel({ key: 'team', source: 'user' }),
        ])

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        expect(screen.getByTitle('Remove team')).toBeInTheDocument()
        expect(screen.queryByTitle('Remove os')).not.toBeInTheDocument()
    })

    it('removes a label on successful delete', async () => {
        mockAgentLabels.mockResolvedValue([makeLabel({ key: 'team', source: 'user' })])
        mockDeleteAgentLabel.mockResolvedValue(undefined)

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.click(screen.getByTitle('Remove team'))

        await waitFor(() => expect(screen.queryByText('team')).not.toBeInTheDocument())
        expect(mockDeleteAgentLabel).toHaveBeenCalledWith('agent-1', 'team')
    })

    it('leaves the label in place if the delete request fails', async () => {
        mockAgentLabels.mockResolvedValue([makeLabel({ key: 'team', source: 'user' })])
        mockDeleteAgentLabel.mockRejectedValue(new Error('forbidden'))

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.click(screen.getByTitle('Remove team'))

        await waitFor(() => expect(mockDeleteAgentLabel).toHaveBeenCalled())
        expect(screen.getByText('team')).toBeInTheDocument() // still there
    })

    it('shows the Add form when an admin clicks + Add, and hides it on Cancel', async () => {
        mockAgentLabels.mockResolvedValue([])
        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Add'))
        expect(screen.getByPlaceholderText('key (e.g. env)')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.queryByPlaceholderText('key (e.g. env)')).not.toBeInTheDocument()
    })

    it('keeps the Add submit button disabled until both key and value are filled', async () => {
        mockAgentLabels.mockResolvedValue([])
        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Add'))

        const addButton = screen.getByRole('button', { name: 'Add' })
        expect(addButton).toBeDisabled()

        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'team' } })
        expect(addButton).toBeDisabled()

        fireEvent.change(screen.getByPlaceholderText('value (e.g. prod)'), { target: { value: 'sre' } })
        expect(addButton).not.toBeDisabled()
    })

    it('adds a new label, closes the form, and shows the new chip', async () => {
        mockAgentLabels.mockResolvedValue([])
        mockSetAgentLabel.mockResolvedValue(makeLabel({ key: 'team', value: 'sre', source: 'user' }))

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Add'))

        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('value (e.g. prod)'), { target: { value: 'sre' } })
        fireEvent.click(screen.getByRole('button', { name: 'Add' }))

        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())
        expect(mockSetAgentLabel).toHaveBeenCalledWith('agent-1', 'team', 'sre')
        expect(screen.queryByPlaceholderText('key (e.g. env)')).not.toBeInTheDocument()
    })

    it('replaces an existing label in place rather than duplicating it', async () => {
        mockAgentLabels.mockResolvedValue([makeLabel({ key: 'team', value: 'sre', source: 'user' })])
        mockSetAgentLabel.mockResolvedValue(makeLabel({ key: 'team', value: 'infra', source: 'user' }))

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('sre')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Add'))
        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('value (e.g. prod)'), { target: { value: 'infra' } })
        fireEvent.click(screen.getByRole('button', { name: 'Add' }))

        await waitFor(() => expect(screen.getByText('infra')).toBeInTheDocument())
        expect(screen.queryByText('sre')).not.toBeInTheDocument()
        expect(screen.getAllByText('team')).toHaveLength(1) // not duplicated
    })

    it('shows an error and keeps the form open when adding fails', async () => {
        mockAgentLabels.mockResolvedValue([])
        mockSetAgentLabel.mockRejectedValue(new Error('reserved key'))

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Add'))

        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'os' } })
        fireEvent.change(screen.getByPlaceholderText('value (e.g. prod)'), { target: { value: 'linux' } })
        fireEvent.click(screen.getByRole('button', { name: 'Add' }))

        await waitFor(() => expect(screen.getByText('reserved key')).toBeInTheDocument())
        expect(screen.getByPlaceholderText('key (e.g. env)')).toBeInTheDocument() // form still open
    })

    it('submits on Enter in the value field once the form is valid', async () => {
        mockAgentLabels.mockResolvedValue([])
        mockSetAgentLabel.mockResolvedValue(makeLabel({ key: 'team', value: 'sre', source: 'user' }))

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Add'))

        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('value (e.g. prod)'), { target: { value: 'sre' } })
        fireEvent.keyDown(screen.getByPlaceholderText('value (e.g. prod)'), { key: 'Enter' })

        await waitFor(() => expect(mockSetAgentLabel).toHaveBeenCalledWith('agent-1', 'team', 'sre'))
    })

    it('cancels on Escape from either field', async () => {
        mockAgentLabels.mockResolvedValue([])
        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await waitFor(() => expect(screen.getByText('+ Add')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Add'))

        fireEvent.keyDown(screen.getByPlaceholderText('key (e.g. env)'), { key: 'Escape' })
        expect(screen.queryByPlaceholderText('key (e.g. env)')).not.toBeInTheDocument()
    })

    it('fetches autocomplete values only when the key matches an existing user key', async () => {
        vi.useFakeTimers()
        const keys: LabelKey[] = [{ key: 'team', source: 'user' }]
        mockAgentLabels.mockResolvedValue([])
        mockLabelKeys.mockResolvedValue(keys)
        mockLabelValues.mockResolvedValue(['sre', 'infra'])

        render(<AgentLabelChips agentId="agent-1" isAdmin={true} />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        fireEvent.click(screen.getByText('+ Add'))

        // A key that doesn't match any existing user key - no autocomplete fetch.
        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'unknown-key' } })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(150)
        })
        expect(mockLabelValues).not.toHaveBeenCalled()

        // A key that does match - triggers the debounced fetch.
        fireEvent.change(screen.getByPlaceholderText('key (e.g. env)'), { target: { value: 'team' } })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(150)
        })
        expect(mockLabelValues).toHaveBeenCalledWith('team')
    })
})