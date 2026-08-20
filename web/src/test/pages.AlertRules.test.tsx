import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { AlertRules } from '../pages/AlertRules'
import type { AlertRule, Agent, AlertChannel, User } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: {
            listAlertRules: vi.fn(),
            agents: vi.fn(),
            listAlertChannels: vi.fn(),
            createAlertRule: vi.fn(),
            updateAlertRule: vi.fn(),
            setAlertRuleEnabled: vi.fn(),
            deleteAlertRule: vi.fn(),
            getAlertRule: vi.fn(),
            agentDisk: vi.fn(),
            agentServices: vi.fn(),
        },
    }
})

import { api, HttpError } from '../api'
const mockListRules = api.listAlertRules as ReturnType<typeof vi.fn>
const mockAgents = api.agents as ReturnType<typeof vi.fn>
const mockListChannels = api.listAlertChannels as ReturnType<typeof vi.fn>
const mockCreateRule = api.createAlertRule as ReturnType<typeof vi.fn>
const mockUpdateRule = api.updateAlertRule as ReturnType<typeof vi.fn>
const mockSetEnabled = api.setAlertRuleEnabled as ReturnType<typeof vi.fn>
const mockDeleteRule = api.deleteAlertRule as ReturnType<typeof vi.fn>
const mockGetRule = api.getAlertRule as ReturnType<typeof vi.fn>
const mockAgentDisk = api.agentDisk as ReturnType<typeof vi.fn>
const mockAgentServices = api.agentServices as ReturnType<typeof vi.fn>

function makeRule(overrides: Partial<AlertRule> = {}): AlertRule {
    return {
        id: 'rule-1',
        name: 'Agent offline',
        enabled: true,
        scope: 'global',
        agent_id: null,
        condition_type: 'agent_offline',
        condition_params: { timeout_seconds: 300 },
        cooldown_seconds: 0,
        created_at: '',
        updated_at: '',
        ...overrides,
    }
}

function makeAgent(overrides: Partial<Agent> = {}): Agent {
    return {
        id: 'agent-1', hostname: 'test-host-1', os: 'linux', platform: 'proxmox', arch: 'amd64',
        cpu_model: 'AMD EPYC', cpu_cores: 8, ram_total: 0, registered_at: '', last_seen: null,
        ip_address: null, ...overrides,
    }
}

function makeChannel(overrides: Partial<AlertChannel> = {}): AlertChannel {
    return { id: 'ch-1', name: 'ops-webhook', type: 'webhook', config: { url: 'https://x.test' }, created_at: '', ...overrides }
}

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'test-admin', role: 'admin', ...overrides } as User
}

beforeEach(() => {
    mockListRules.mockReset().mockResolvedValue([])
    mockAgents.mockReset().mockResolvedValue([])
    mockListChannels.mockReset().mockResolvedValue([])
    mockCreateRule.mockReset()
    mockUpdateRule.mockReset()
    mockSetEnabled.mockReset()
    mockDeleteRule.mockReset()
    mockGetRule.mockReset().mockResolvedValue({ rule: makeRule(), channels: [] })
    mockAgentDisk.mockReset().mockResolvedValue([])
    mockAgentServices.mockReset().mockResolvedValue([])
})

afterEach(() => {
    vi.useRealTimers()
})

describe('AlertRules - listing', () => {
    it('shows a loading spinner, then renders rules', async () => {
        mockListRules.mockResolvedValue([makeRule({ name: 'Agent offline' })])

        const { container } = render(<AlertRules user={makeUser()} />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('Agent offline')).toBeInTheDocument())
        expect(screen.getByText('Agent Offline')).toBeInTheDocument() // condition label
        expect(screen.getByText('All agents')).toBeInTheDocument()
    })

    it('shows an error message on load failure', async () => {
        mockListRules.mockRejectedValue(new Error('connection refused'))
        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows the empty state with no rules', async () => {
        mockListRules.mockResolvedValue([])
        render(<AlertRules user={makeUser()} />)
        await waitFor(() =>
            expect(screen.getByText('No rules configured. Create one to start evaluating alert conditions.')).toBeInTheDocument()
        )
    })

    it('resolves an agent-scoped rule\'s target to the agent hostname', async () => {
        mockListRules.mockResolvedValue([makeRule({ scope: 'agent', agent_id: 'agent-1' })])
        mockAgents.mockResolvedValue([makeAgent({ id: 'agent-1', hostname: 'test-host-1' })])

        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())
    })

    it('toggles enabled/disabled and reloads the list', async () => {
        mockListRules.mockResolvedValue([makeRule({ enabled: true })])
        mockSetEnabled.mockResolvedValue(makeRule({ enabled: false }))

        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('Enabled', { selector: 'button' })).toBeInTheDocument())

        fireEvent.click(screen.getByText('Enabled', { selector: 'button' }))
        await waitFor(() => expect(mockSetEnabled).toHaveBeenCalledWith('rule-1', false))
    })

    it('shows a toggle error that clears itself after 3 seconds', async () => {
        vi.useFakeTimers()
        mockListRules.mockResolvedValue([makeRule({ enabled: true })])
        mockSetEnabled.mockRejectedValue(new Error('toggle failed'))

        render(<AlertRules user={makeUser()} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Enabled', { selector: 'button' }))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        expect(screen.getByText('toggle failed')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
        expect(screen.queryByText('toggle failed')).not.toBeInTheDocument()
    })
})

describe('AlertRules - delete flow', () => {
    it('requires a confirm click before deleting', async () => {
        mockListRules.mockResolvedValue([makeRule({ name: 'Agent offline' })])
        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete'))
        expect(screen.getByText('Delete Agent offline?')).toBeInTheDocument()
        expect(mockDeleteRule).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Confirm'))
        await waitFor(() => expect(mockDeleteRule).toHaveBeenCalledWith('rule-1'))
    })
})

describe('AlertRules - RuleModal validation and scope/condition rules', () => {
    async function openCreateModal() {
        mockListRules.mockResolvedValue([])
        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('+ Create Rule')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Rule'))
        await waitFor(() => expect(screen.getByText('Create Rule', { selector: 'div' })).toBeInTheDocument())
    }

    it('requires a name', async () => {
        await openCreateModal()
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))
        expect(screen.getByText('Name is required.')).toBeInTheDocument()
        expect(mockCreateRule).not.toHaveBeenCalled()
    })

    it('requires an agent for agent-scoped rules', async () => {
        await openCreateModal()
        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'my rule' } })
        fireEvent.change(screen.getByDisplayValue('Global (all agents)'), { target: { value: 'agent' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        expect(screen.getByText('An agent must be selected for agent-scoped rules.')).toBeInTheDocument()
    })

    it('forces agent scope when Service Down is selected, and disables the global option', async () => {
        await openCreateModal()
        fireEvent.change(screen.getByDisplayValue('Agent Offline'), { target: { value: 'service_down' } })

        expect(screen.getByDisplayValue('Single agent')).toBeInTheDocument()
        const globalOption = screen.getByText('Global — unavailable for Service Down') as HTMLOptionElement
        expect(globalOption.disabled).toBe(true)
    })

    it('validates a positive timeout for agent_offline', async () => {
        mockAgents.mockResolvedValue([makeAgent()])
        await openCreateModal()
        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'r' } })
        fireEvent.change(screen.getByDisplayValue('300'), { target: { value: '0' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        expect(screen.getByText('Timeout must be a positive number of seconds.')).toBeInTheDocument()
    })

    it('requires a mount path for disk_prediction', async () => {
        await openCreateModal()
        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'r' } })
        fireEvent.change(screen.getByDisplayValue('Agent Offline'), { target: { value: 'disk_prediction' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        expect(screen.getByText('Mount path is required for disk prediction.')).toBeInTheDocument()
    })

    it('requires a service name for service_down', async () => {
        mockAgents.mockResolvedValue([makeAgent({ id: 'agent-1', hostname: 'test-host-1' })])
        await openCreateModal()
        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'r' } })
        fireEvent.change(screen.getByDisplayValue('Agent Offline'), { target: { value: 'service_down' } })
        fireEvent.change(screen.getByDisplayValue('Select an agent…'), { target: { value: 'agent-1' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        expect(screen.getByText('Service name is required for service down.')).toBeInTheDocument()
    })

    it('creates an agent_offline rule with the built params', async () => {
        mockCreateRule.mockResolvedValue({ rule: makeRule() })
        await openCreateModal()

        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'my rule' } })
        fireEvent.change(screen.getByDisplayValue('300'), { target: { value: '600' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        await waitFor(() =>
            expect(mockCreateRule).toHaveBeenCalledWith(
                expect.objectContaining({
                    name: 'my rule',
                    scope: 'global',
                    condition_type: 'agent_offline',
                    condition_params: { timeout_seconds: 600 },
                    channel_ids: [],
                })
            )
        )
    })

    it('shows the server error message on create failure', async () => {
        mockCreateRule.mockRejectedValue(new HttpError(400, 'duplicate rule name'))
        await openCreateModal()
        fireEvent.change(screen.getByPlaceholderText('Production web server offline'), { target: { value: 'r' } })
        fireEvent.click(screen.getByText('Create Rule', { selector: 'button' }))

        await waitFor(() => expect(screen.getByText('duplicate rule name')).toBeInTheDocument())
    })

    it('fetches mount autocomplete options for an agent-scoped disk_prediction rule', async () => {
        mockAgents.mockResolvedValue([makeAgent({ id: 'agent-1', hostname: 'test-host-1' })])
        mockAgentDisk.mockResolvedValue([
            { mountpoint: '/data' },
            { mountpoint: '/' },
        ])
        await openCreateModal()

        fireEvent.change(screen.getByDisplayValue('Agent Offline'), { target: { value: 'disk_prediction' } })
        fireEvent.change(screen.getByDisplayValue('Global (all agents)'), { target: { value: 'agent' } })
        fireEvent.change(screen.getByDisplayValue('Select an agent…'), { target: { value: 'agent-1' } })

        await waitFor(() => expect(mockAgentDisk).toHaveBeenCalledWith('agent-1', { type: 'quick', range: '1h' }))
    })

    it('fetches service-name autocomplete options once an agent is selected for service_down', async () => {
        mockAgents.mockResolvedValue([makeAgent({ id: 'agent-1', hostname: 'test-host-1' })])
        mockAgentServices.mockResolvedValue([{ name: 'nginx' }, { name: 'sshd' }])
        await openCreateModal()

        fireEvent.change(screen.getByDisplayValue('Agent Offline'), { target: { value: 'service_down' } })
        fireEvent.change(screen.getByDisplayValue('Select an agent…'), { target: { value: 'agent-1' } })

        await waitFor(() => expect(mockAgentServices).toHaveBeenCalledWith('agent-1'))
    })

    it('closes the modal via Cancel', async () => {
        await openCreateModal()
        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.queryByText('Create Rule', { selector: 'div' })).not.toBeInTheDocument()
    })
})

describe('AlertRules - editing', () => {
    async function openEditModal(rule: AlertRule) {
        mockListRules.mockResolvedValue([rule])
        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('Edit')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Edit'))
        await waitFor(() => expect(screen.getByText('Edit Rule')).toBeInTheDocument())
    }

    it('pre-fills the form and marks condition/scope as fixed', async () => {
        await openEditModal(makeRule({ name: 'Existing rule', condition_params: { timeout_seconds: 900 } }))

        expect(screen.getByDisplayValue('Existing rule')).toBeInTheDocument()
        expect(screen.getByDisplayValue('900')).toBeInTheDocument()
        expect(screen.getByDisplayValue('Agent Offline')).toBeDisabled()
        expect(screen.getByDisplayValue('Global (all agents)')).toBeDisabled()
    })

    it('pre-selects the rule\'s currently attached channels once loaded', async () => {
        mockGetRule.mockResolvedValue({
            rule: makeRule(),
            channels: [makeChannel({ id: 'ch-1' })],
        })
        mockListChannels.mockResolvedValue([makeChannel({ id: 'ch-1', name: 'ops-webhook' }), makeChannel({ id: 'ch-2', name: 'pager' })])

        await openEditModal(makeRule())
        await waitFor(() => expect(screen.getByText('ops-webhook')).toBeInTheDocument())

        const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
        const opsCheckbox = checkboxes.find((c) => c.closest('label')?.textContent?.includes('ops-webhook'))
        const pagerCheckbox = checkboxes.find((c) => c.closest('label')?.textContent?.includes('pager'))
        expect(opsCheckbox!.checked).toBe(true)
        expect(pagerCheckbox!.checked).toBe(false)
    })

    it('disables Save Changes until the existing rule\'s channels have finished loading', async () => {
        mockGetRule.mockReturnValue(new Promise(() => {})) // never resolves
        await openEditModal(makeRule())

        expect(screen.getByText('Save Changes')).toBeDisabled()
        expect(mockUpdateRule).not.toHaveBeenCalled()
    })

    it('sends only mutable fields on update', async () => {
        mockGetRule.mockResolvedValue({ rule: makeRule(), channels: [] })
        mockUpdateRule.mockResolvedValue({ rule: makeRule() })

        await openEditModal(makeRule({ id: 'rule-1', name: 'Existing rule' }))
        await waitFor(() => expect(mockGetRule).toHaveBeenCalled())

        fireEvent.change(screen.getByDisplayValue('Existing rule'), { target: { value: 'Renamed rule' } })
        fireEvent.click(screen.getByText('Save Changes'))

        await waitFor(() =>
            expect(mockUpdateRule).toHaveBeenCalledWith('rule-1', {
                name: 'Renamed rule',
                enabled: true,
                condition_params: { timeout_seconds: 300 },
                cooldown_seconds: 0,
                channel_ids: [],
            })
        )
    })
})

describe('AlertRules - viewer permissions', () => {
    it('hides create, edit, delete and the Actions column from a viewer', async () => {
        mockListRules.mockResolvedValue([makeRule({ name: 'Agent offline' })])

        render(<AlertRules user={makeUser({ username: 'test-viewer', role: 'viewer' })} />)
        await waitFor(() => expect(screen.getByText('Agent offline')).toBeInTheDocument())

        expect(screen.queryByText('+ Create Rule')).not.toBeInTheDocument()
        expect(screen.queryByText('Actions')).not.toBeInTheDocument()
        expect(screen.queryByText('Edit')).not.toBeInTheDocument()
        expect(screen.queryByText('Delete')).not.toBeInTheDocument()
    })

    it('renders the enabled state as a non-interactive badge for a viewer', async () => {
        mockListRules.mockResolvedValue([makeRule({ name: 'Agent offline', enabled: true })])

        render(<AlertRules user={makeUser({ username: 'test-viewer', role: 'viewer' })} />)
        await waitFor(() => expect(screen.getByText('Agent offline')).toBeInTheDocument())

        // 'Enabled' is also the column header, so match on the badge element itself.
        expect(screen.queryByRole('button', { name: 'Enabled' })).not.toBeInTheDocument()
        const badge = screen.getAllByText('Enabled').find((el) => el.tagName === 'SPAN')
        expect(badge).toBeDefined()

        fireEvent.click(badge!)
        expect(mockSetEnabled).not.toHaveBeenCalled()
    })

    it('still shows create, edit and delete to an admin', async () => {
        mockListRules.mockResolvedValue([makeRule({ name: 'Agent offline' })])

        render(<AlertRules user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('Agent offline')).toBeInTheDocument())

        expect(screen.getByText('+ Create Rule')).toBeInTheDocument()
        expect(screen.getByText('Actions')).toBeInTheDocument()
        expect(screen.getByText('Edit')).toBeInTheDocument()
        expect(screen.getByText('Delete')).toBeInTheDocument()
    })
})