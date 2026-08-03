import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { Tags } from '../pages/Tags'
import type { OverviewAgent, AgentLabel, LabelKey, User } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: {
            overview: vi.fn(),
            agentLabels: vi.fn(),
            labelKeys: vi.fn(),
            labelValues: vi.fn(),
            setAgentLabel: vi.fn(),
            deleteAgentLabel: vi.fn(),
        },
    }
})

import { api } from '../api'
const mockOverview = api.overview as ReturnType<typeof vi.fn>
const mockAgentLabels = api.agentLabels as ReturnType<typeof vi.fn>
const mockLabelKeys = api.labelKeys as ReturnType<typeof vi.fn>
const mockLabelValues = api.labelValues as ReturnType<typeof vi.fn>
const mockSetAgentLabel = api.setAgentLabel as ReturnType<typeof vi.fn>
const mockDeleteAgentLabel = api.deleteAgentLabel as ReturnType<typeof vi.fn>

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
        updated_at: '',
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

function makeLabel(overrides: Partial<AgentLabel> = {}): AgentLabel {
    return { key: 'os', value: 'linux', source: 'auto', updated_at: '', ...overrides }
}

function makeUser(role: string): User {
    return { id: 'u1', username: 'test-admin', role } as User
}

// Sets up a fleet of agents each with their own label set, matching how the
// component actually builds its internal labelsByAgent map (one
// api.agentLabels(id) call per agent from the overview list).
function setupFleet(agentLabels: Record<string, AgentLabel[]>) {
    const agents = Object.keys(agentLabels).map((id) => makeAgent({ id, hostname: `host-${id}` }))
    mockOverview.mockResolvedValue(agents)
    mockAgentLabels.mockImplementation((id: string) => Promise.resolve(agentLabels[id] ?? []))
    return agents
}

beforeEach(() => {
    mockOverview.mockReset().mockResolvedValue([])
    mockAgentLabels.mockReset().mockResolvedValue([])
    mockLabelKeys.mockReset().mockResolvedValue([])
    mockLabelValues.mockReset().mockResolvedValue([])
    mockSetAgentLabel.mockReset()
    mockDeleteAgentLabel.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('Tags - loading, error, admin gating', () => {
    it('shows a loading spinner, then renders', async () => {
        mockOverview.mockResolvedValue([])
        const { container } = render(<Tags user={makeUser('admin')} />)
        expect(container.querySelector('svg')).toBeInTheDocument()
        await waitFor(() => expect(screen.getByText('Tags')).toBeInTheDocument())
    })

    it('shows an error message on load failure', async () => {
        mockOverview.mockRejectedValue(new Error('connection refused'))
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows the Apply Tag panel for an admin', async () => {
        mockOverview.mockResolvedValue([])
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Apply a tag')).toBeInTheDocument())
    })

    it('hides the Apply Tag panel for a viewer', async () => {
        mockOverview.mockResolvedValue([])
        render(<Tags user={makeUser('viewer')} />)
        await waitFor(() => expect(screen.getByText('All tags')).toBeInTheDocument())
        expect(screen.queryByText('Apply a tag')).not.toBeInTheDocument()
    })

    it('regression: a viewer never sees a Remove control on user tags (previously rendered but silently no-opped)', async () => {
        setupFleet({ a1: [makeLabel({ key: 'team', value: 'sre', source: 'user' })] })
        render(<Tags user={makeUser('viewer')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.click(screen.getByText('team')) // expand
        expect(screen.queryByText('Remove')).not.toBeInTheDocument()
    })

    it('an admin sees Remove on a user tag but not on an auto tag', async () => {
        setupFleet({
            a1: [
                makeLabel({ key: 'team', value: 'sre', source: 'user' }),
                makeLabel({ key: 'os', value: 'linux', source: 'auto' }),
            ],
        })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.click(screen.getByText('team'))
        expect(screen.getByText('Remove')).toBeInTheDocument()

        fireEvent.click(screen.getByText('os'))
        expect(screen.getByText('read-only')).toBeInTheDocument()
    })
})

describe('Tags - summary aggregation', () => {
    it('sorts user keys before auto keys, alphabetically within each group', async () => {
        setupFleet({
            a1: [
                makeLabel({ key: 'zeta-auto', source: 'auto' }),
                makeLabel({ key: 'alpha-auto', source: 'auto' }),
                makeLabel({ key: 'zeta-user', value: 'x', source: 'user' }),
                makeLabel({ key: 'alpha-user', value: 'x', source: 'user' }),
            ],
        })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('alpha-user')).toBeInTheDocument())

        const keys = ['alpha-user', 'zeta-user', 'alpha-auto', 'zeta-auto']
        const allKeyEls = keys.map((k) => screen.getByText(k))
        for (let i = 1; i < allKeyEls.length; i++) {
            const rel = allKeyEls[i - 1]!.compareDocumentPosition(allKeyEls[i]!)
            expect(rel & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
        }
    })

    it('counts distinct agents across all values of a key, not total label rows', async () => {
        setupFleet({
            a1: [makeLabel({ key: 'team', value: 'sre', source: 'user' })],
            a2: [makeLabel({ key: 'team', value: 'infra', source: 'user' })],
        })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText(/2 agents/)).toBeInTheDocument())
    })

    it('lists agent hostnames per value, truncated past 6', async () => {
        const labels: Record<string, AgentLabel[]> = {}
        for (let i = 1; i <= 8; i++) {
            labels[`a${i}`] = [makeLabel({ key: 'team', value: 'sre', source: 'user' })]
        }
        setupFleet(labels)
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.click(screen.getByText('team'))
        await waitFor(() => expect(screen.getByText(/\+2 more/)).toBeInTheDocument())
    })
})

describe('Tags - AllTagsPanel filtering and empty states', () => {
    it('shows "No tags yet" when there are no labels at all', async () => {
        setupFleet({ a1: [] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() =>
            expect(screen.getByText('No tags yet. Apply one above to get started.')).toBeInTheDocument()
        )
    })

    it('filters keys by the filter text, matching key or value', async () => {
        setupFleet({
            a1: [
                makeLabel({ key: 'team', value: 'sre', source: 'user' }),
                makeLabel({ key: 'env', value: 'prod', source: 'user' }),
            ],
        })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Filter keys...'), { target: { value: 'env' } })
        expect(screen.queryByText('team')).not.toBeInTheDocument()
        expect(screen.getByText('env')).toBeInTheDocument()
    })

    it('shows "No keys match the filter" distinctly from the truly-empty state', async () => {
        setupFleet({ a1: [makeLabel({ key: 'team', value: 'sre', source: 'user' })] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Filter keys...'), { target: { value: 'nonexistent' } })
        expect(screen.getByText('No keys match the filter.')).toBeInTheDocument()
    })

    it('deletes a value after confirming, across all its agents', async () => {
        setupFleet({
            a1: [makeLabel({ key: 'team', value: 'sre', source: 'user' })],
            a2: [makeLabel({ key: 'team', value: 'sre', source: 'user' })],
        })
        mockDeleteAgentLabel.mockResolvedValue(undefined)

        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())
        fireEvent.click(screen.getByText('team'))

        fireEvent.click(screen.getByText('Remove'))
        expect(screen.getByText('Remove from 2 agents?')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Confirm'))
        await waitFor(() => expect(mockDeleteAgentLabel).toHaveBeenCalledWith('a1', 'team'))
        expect(mockDeleteAgentLabel).toHaveBeenCalledWith('a2', 'team')
    })

    it('cancels a delete confirmation without deleting', async () => {
        setupFleet({ a1: [makeLabel({ key: 'team', value: 'sre', source: 'user' })] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('team')).toBeInTheDocument())
        fireEvent.click(screen.getByText('team'))

        fireEvent.click(screen.getByText('Remove'))
        fireEvent.click(screen.getByText('Cancel'))

        expect(screen.queryByText('Remove from')).not.toBeInTheDocument()
        expect(mockDeleteAgentLabel).not.toHaveBeenCalled()
    })
})

describe('Tags - ApplyTagPanel: agent picker filters', () => {
    it('filters the agent picker by hostname search', async () => {
        setupFleet({ a1: [], a2: [] }) // hostnames become host-a1 / host-a2
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('host-a1')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Search agents...'), { target: { value: 'a2' } })
        expect(screen.queryByText('host-a1')).not.toBeInTheDocument()
        expect(screen.getByText('host-a2')).toBeInTheDocument()
    })

    it('filters by OS', async () => {
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', hostname: 'linux-host', os: 'linux' }),
            makeAgent({ id: 'a2', hostname: 'bsd-host', os: 'freebsd' }),
        ])
        mockAgentLabels.mockResolvedValue([])
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('linux-host')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('All OS'), { target: { value: 'freebsd' } })
        expect(screen.queryByText('linux-host')).not.toBeInTheDocument()
        expect(screen.getByText('bsd-host')).toBeInTheDocument()
    })

    it('filters by an existing user label key/value', async () => {
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', hostname: 'prod-host' }),
            makeAgent({ id: 'a2', hostname: 'staging-host' }),
        ])
        mockAgentLabels.mockImplementation((id: string) =>
            Promise.resolve(id === 'a1' ? [makeLabel({ key: 'env', value: 'prod', source: 'user' })] : [makeLabel({ key: 'env', value: 'staging', source: 'user' })])
        )
        mockLabelKeys.mockResolvedValue([{ key: 'env', source: 'user' } as LabelKey])

        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('prod-host')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('(any)'), { target: { value: 'env' } })
        fireEvent.change(screen.getByDisplayValue('(any value)'), { target: { value: 'prod' } })

        expect(screen.getByText('prod-host')).toBeInTheDocument()
        expect(screen.queryByText('staging-host')).not.toBeInTheDocument()
    })

    it('toggles Select All Visible / Deselect All Visible', async () => {
        setupFleet({ a1: [], a2: [] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Select All Visible')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Select All Visible'))
        expect(screen.getByText('2 selected')).toBeInTheDocument()
        expect(screen.getByText('Deselect All Visible')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Deselect All Visible'))
        expect(screen.getByText('0 selected')).toBeInTheDocument()
    })
})

describe('Tags - ApplyTagPanel: bulk apply', () => {
    it('disables Apply until key, value, and at least one agent are set', async () => {
        setupFleet({ a1: [] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Apply')).toBeInTheDocument())
        expect(screen.getByText('Apply')).toBeDisabled()

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('e.g. prod'), { target: { value: 'sre' } })
        expect(screen.getByText('Apply')).toBeDisabled() // still no agent selected

        fireEvent.click(screen.getByText('Select All Visible'))
        expect(screen.getByText('Apply to 1 Agent')).not.toBeDisabled()
    })

    it('applies successfully to all selected agents, clears the form, and shows a confirmation', async () => {
        vi.useFakeTimers()
        setupFleet({ a1: [], a2: [] })
        mockSetAgentLabel.mockResolvedValue(makeLabel({ key: 'team', value: 'sre', source: 'user' }))

        render(<Tags user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('e.g. prod'), { target: { value: 'sre' } })
        fireEvent.click(screen.getByText('Select All Visible'))
        fireEvent.click(screen.getByText('Apply to 2 Agents'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        expect(mockSetAgentLabel).toHaveBeenCalledWith('a1', 'team', 'sre')
        expect(mockSetAgentLabel).toHaveBeenCalledWith('a2', 'team', 'sre')
        expect(screen.getByText('2 agents tagged with team=sre')).toBeInTheDocument()
        expect(screen.getByPlaceholderText('e.g. env')).toHaveValue('')

        await act(async () => { await vi.advanceTimersByTimeAsync(4000) })
        expect(screen.queryByText('2 agents tagged with team=sre')).not.toBeInTheDocument()
    })

    it('reports a partial failure without clearing the form', async () => {
        setupFleet({ a1: [], a2: [] })
        mockSetAgentLabel.mockImplementation((id: string) =>
            id === 'a1' ? Promise.resolve(makeLabel()) : Promise.reject(new Error('validation failed'))
        )

        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('host-a1')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('e.g. prod'), { target: { value: 'sre' } })
        fireEvent.click(screen.getByText('Select All Visible'))
        fireEvent.click(screen.getByText('Apply to 2 Agents'))

        await waitFor(() => expect(screen.getByText('1 tagged, 1 failed')).toBeInTheDocument())
        expect(screen.getByPlaceholderText('e.g. env')).toHaveValue('team') // not cleared
    })

    it('shows the server error when every apply fails', async () => {
        setupFleet({ a1: [] })
        mockSetAgentLabel.mockRejectedValue(new Error('reserved key'))

        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('host-a1')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'os' } })
        fireEvent.change(screen.getByPlaceholderText('e.g. prod'), { target: { value: 'linux' } })
        fireEvent.click(screen.getByText('Select All Visible'))
        fireEvent.click(screen.getByText('Apply to 1 Agent'))

        await waitFor(() => expect(screen.getByText('reserved key')).toBeInTheDocument())
    })

    it('fetches value autocomplete only for a key that already exists as a user key', async () => {
        vi.useFakeTimers()
        setupFleet({ a1: [] })
        mockLabelKeys.mockResolvedValue([{ key: 'team', source: 'user' } as LabelKey])
        mockLabelValues.mockResolvedValue(['sre', 'infra'])

        render(<Tags user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'unknown-key' } })
        await act(async () => { await vi.advanceTimersByTimeAsync(150) })
        expect(mockLabelValues).not.toHaveBeenCalled()

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'team' } })
        await act(async () => { await vi.advanceTimersByTimeAsync(150) })
        expect(mockLabelValues).toHaveBeenCalledWith('team')
    })

    it('Clear resets the key, value, and selection', async () => {
        setupFleet({ a1: [] })
        render(<Tags user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('host-a1')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('e.g. env'), { target: { value: 'team' } })
        fireEvent.change(screen.getByPlaceholderText('e.g. prod'), { target: { value: 'sre' } })
        fireEvent.click(screen.getByText('Select All Visible'))

        fireEvent.click(screen.getByText('Clear'))
        expect(screen.getByPlaceholderText('e.g. env')).toHaveValue('')
        expect(screen.getByPlaceholderText('e.g. prod')).toHaveValue('')
        expect(screen.getByText('0 selected')).toBeInTheDocument()
    })
})