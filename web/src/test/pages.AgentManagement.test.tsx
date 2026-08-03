import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { AgentManagement } from '../pages/AgentManagement'
import type { OverviewAgent, Agent, User, PlatformInfo, ProvisionResponse } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: {
            overview: vi.fn(),
            agent: vi.fn(),
            agentConfig: vi.fn(),
            agentDisk: vi.fn(),
            agentNetwork: vi.fn(),
            setAgentConfig: vi.fn(),
            deleteAgentConfig: vi.fn(),
            upgradeInstructions: vi.fn(),
            uninstallInstructions: vi.fn(),
            deleteAgent: vi.fn(),
            pushUpdate: vi.fn(),
            purgeOfflineAgents: vi.fn(),
            revokeAllTokens: vi.fn(),
            platforms: vi.fn(),
            provision: vi.fn(),
        },
    }
})

import { api } from '../api'
const mockOverview = api.overview as ReturnType<typeof vi.fn>
const mockAgent = api.agent as ReturnType<typeof vi.fn>
const mockAgentConfig = api.agentConfig as ReturnType<typeof vi.fn>
const mockAgentDisk = api.agentDisk as ReturnType<typeof vi.fn>
const mockAgentNetwork = api.agentNetwork as ReturnType<typeof vi.fn>
const mockSetAgentConfig = api.setAgentConfig as ReturnType<typeof vi.fn>
const mockDeleteAgentConfig = api.deleteAgentConfig as ReturnType<typeof vi.fn>
const mockUpgradeInstructions = api.upgradeInstructions as ReturnType<typeof vi.fn>
const mockUninstallInstructions = api.uninstallInstructions as ReturnType<typeof vi.fn>
const mockDeleteAgent = api.deleteAgent as ReturnType<typeof vi.fn>
const mockPushUpdate = api.pushUpdate as ReturnType<typeof vi.fn>
const mockPurgeOffline = api.purgeOfflineAgents as ReturnType<typeof vi.fn>
const mockRevokeTokens = api.revokeAllTokens as ReturnType<typeof vi.fn>
const mockPlatforms = api.platforms as ReturnType<typeof vi.fn>
const mockProvision = api.provision as ReturnType<typeof vi.fn>

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
        update_available: false,
        ...overrides,
    }
}

function makeFullAgent(overrides: Partial<Agent> = {}): Agent {
    return {
        id: 'a1', hostname: 'test-host-1', os: 'linux', platform: 'proxmox', arch: 'amd64',
        cpu_model: 'AMD EPYC', cpu_cores: 8, ram_total: 0, registered_at: '', last_seen: null,
        ip_address: '198.51.100.10', ...overrides,
    }
}

function makeUser(role: string): User {
    return { id: 'u1', username: 'test-admin', role } as User
}

function makePlatform(overrides: Partial<PlatformInfo> = {}): PlatformInfo {
    return { os: 'linux', arch: 'amd64', label: 'Linux (x86_64)', filename: 'spectra-agent-linux-amd64', ...overrides }
}

function makeProvisionResponse(overrides: Partial<ProvisionResponse> = {}): ProvisionResponse {
    return {
        token: 'tok_abc123',
        expires_at: '2026-01-02T00:00:00.000Z',
        platform: 'spectra-agent-linux-amd64',
        download_url: '',
        config: { server: 'https://spectra.example.com', token: 'tok_abc123' },
        install: { type: 'systemd', content: '', steps: '1. Copy the binary\ncurl -o agent https://x\n2. Run it\n./agent' },
        ...overrides,
    }
}

// Default happy-path resolves for AgentConfigPanel's admin-only fetches, so
// opening the modal doesn't hang in tests that don't care about its details.
function setAgentConfigPanelDefaults() {
    mockAgent.mockResolvedValue(makeFullAgent())
    mockAgentConfig.mockResolvedValue({})
    mockAgentDisk.mockResolvedValue([])
    mockAgentNetwork.mockResolvedValue([])
}

beforeEach(() => {
    mockOverview.mockReset().mockResolvedValue([])
    mockAgent.mockReset()
    mockAgentConfig.mockReset()
    mockAgentDisk.mockReset()
    mockAgentNetwork.mockReset()
    mockSetAgentConfig.mockReset()
    mockDeleteAgentConfig.mockReset()
    mockUpgradeInstructions.mockReset()
    mockUninstallInstructions.mockReset()
    mockDeleteAgent.mockReset()
    mockPushUpdate.mockReset()
    mockPurgeOffline.mockReset()
    mockRevokeTokens.mockReset()
    mockPlatforms.mockReset()
    mockProvision.mockReset()
    setAgentConfigPanelDefaults()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('AgentManagement - listing', () => {
    it('shows a loading spinner, then renders agents', async () => {
        mockOverview.mockResolvedValue([makeAgent({ hostname: 'test-host-1' })])
        const { container } = render(<AgentManagement user={makeUser('admin')} />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())
        expect(screen.getByText('1 agent registered')).toBeInTheDocument()
    })

    it('shows an error message on load failure', async () => {
        mockOverview.mockRejectedValue(new Error('connection refused'))
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('filters by hostname, os, or platform', async () => {
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', hostname: 'web-1', os: 'linux', platform: 'proxmox' }),
            makeAgent({ id: 'a2', hostname: 'db-1', os: 'freebsd', platform: 'bare-metal' }),
        ])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('web-1')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('Search agents...'), { target: { value: 'freebsd' } })
        expect(screen.queryByText('web-1')).not.toBeInTheDocument()
        expect(screen.getByText('db-1')).toBeInTheDocument()
    })

    it('pluralizes the agent count', async () => {
        mockOverview.mockResolvedValue([makeAgent({ id: 'a1' }), makeAgent({ id: 'a2' })])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('2 agents registered')).toBeInTheDocument())
    })
})

describe('AgentManagement - config modal open/close', () => {
    it('opens the config modal on row click and closes via the × button', async () => {
        mockOverview.mockResolvedValue([makeAgent({ hostname: 'test-host-1' })])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())

        fireEvent.click(screen.getByText('test-host-1'))
        await waitFor(() => expect(screen.getAllByText('test-host-1').length).toBeGreaterThan(1)) // row + modal title

        fireEvent.click(screen.getByText('×'))
        await waitFor(() => expect(screen.getAllByText('test-host-1')).toHaveLength(1))
    })

    it('closes the config modal on Escape', async () => {
        mockOverview.mockResolvedValue([makeAgent({ hostname: 'test-host-1' })])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())

        fireEvent.click(screen.getByText('test-host-1'))
        await waitFor(() => expect(screen.getAllByText('test-host-1').length).toBeGreaterThan(1))

        fireEvent.keyDown(window, { key: 'Escape' })
        await waitFor(() => expect(screen.getAllByText('test-host-1')).toHaveLength(1))
    })
})

describe('AgentManagement - update flow', () => {
    it('shows the outdated badge and a selectable checkbox for an outdated agent', async () => {
        mockOverview.mockResolvedValue([makeAgent({ update_available: true })])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('outdated')).toBeInTheDocument())
        expect(screen.getByRole('checkbox')).toBeInTheDocument()
    })

    it('toggles Select All Outdated / Deselect All', async () => {
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true }),
            makeAgent({ id: 'a2', update_available: true }),
        ])
        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Select All Outdated')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Select All Outdated'))
        expect(screen.getByText('Update 2 Agents')).toBeInTheDocument()
        expect(screen.getByText('Deselect All')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Deselect All'))
        expect(screen.getByText('Update 0 Agents')).toBeInTheDocument()
    })

    it('pushes an update and shows the queued count', async () => {
        mockOverview.mockResolvedValue([makeAgent({ id: 'a1', update_available: true })])
        mockPushUpdate.mockResolvedValue({ queued: 1, skipped: 0, failed: 0 })

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Select All Outdated')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))

        await waitFor(() => expect(mockPushUpdate).toHaveBeenCalledWith(['a1']))
        // The top-level "N queued" summary is masked while hasPendingUpdates
        // is true (both states update in the same batch on a successful
        // push) - the per-row QUEUED badge is the visible confirmation here.
        await waitFor(() => expect(screen.getByText('QUEUED')).toBeInTheDocument())
    })

    it('hides the row checkbox once an update has been queued for that agent', async () => {
        mockOverview.mockResolvedValue([makeAgent({ id: 'a1', update_available: true })])
        mockPushUpdate.mockResolvedValue({ queued: 1, skipped: 0, failed: 0 })

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByRole('checkbox')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))

        await waitFor(() => expect(screen.getByText('QUEUED')).toBeInTheDocument())
        expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    })

    it('shows updating once the agent has update_available but has not gone offline, then restarting once it does', async () => {
        vi.useFakeTimers()
        const base = new Date('2026-01-01T00:00:00.000Z')
        vi.setSystemTime(base)

        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true, last_seen: base.toISOString() }),
        ])
        mockPushUpdate.mockResolvedValue({ queued: 1, skipped: 0, failed: 0 })

        render(<AgentManagement user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
            await vi.advanceTimersByTimeAsync(0)
        })
        // The watch-progress effect depends on [agents, updateStatuses], so
        // it reconciles again the instant the push sets "queued" - against
        // the same still-online agent snapshot, immediately becoming
        // UPDATING. QUEUED is a one-render flash, not a stable state here.
        expect(screen.getByText('UPDATING')).toBeInTheDocument()

        // Still recently seen (not offline yet) - stays UPDATING.
        vi.setSystemTime(new Date(base.getTime() + 1_000))
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true, last_seen: base.toISOString() }),
        ])
        await act(async () => { await vi.advanceTimersByTimeAsync(3_000) }) // fast poll while updates pending
        expect(screen.getByText('UPDATING')).toBeInTheDocument()

        // Now more than 15s since last_seen with no newer heartbeat - offline/restarting.
        vi.setSystemTime(new Date(base.getTime() + 20_000))
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true, last_seen: base.toISOString() }),
        ])
        await act(async () => { await vi.advanceTimersByTimeAsync(3_000) })
        expect(screen.getByText('RESTARTING')).toBeInTheDocument()
    })

    it('shows updated once update_available flips false, then clears after 10 seconds', async () => {
        vi.useFakeTimers()
        const base = new Date('2026-01-01T00:00:00.000Z')
        vi.setSystemTime(base)

        mockOverview.mockResolvedValue([makeAgent({ id: 'a1', update_available: true })])
        mockPushUpdate.mockResolvedValue({ queued: 1, skipped: 0, failed: 0 })

        render(<AgentManagement user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        // Next poll reports the agent as fully updated.
        mockOverview.mockResolvedValue([makeAgent({ id: 'a1', update_available: false })])
        vi.setSystemTime(new Date(base.getTime() + 1_000))
        await act(async () => { await vi.advanceTimersByTimeAsync(3_000) })
        expect(screen.getByText('UPDATED')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(10_000) })
        expect(screen.queryByText('UPDATED')).not.toBeInTheDocument()
    })

    it('marks an update as failed after the 60-second timeout', async () => {
        vi.useFakeTimers()
        const base = new Date('2026-01-01T00:00:00.000Z')
        vi.setSystemTime(base)

        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true, last_seen: base.toISOString() }),
        ])
        mockPushUpdate.mockResolvedValue({ queued: 1, skipped: 0, failed: 0 })

        render(<AgentManagement user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        vi.setSystemTime(new Date(base.getTime() + 61_000))
        mockOverview.mockResolvedValue([
            makeAgent({ id: 'a1', update_available: true, last_seen: base.toISOString() }),
        ])
        await act(async () => { await vi.advanceTimersByTimeAsync(3_000) })
        expect(screen.getByText('FAILED')).toBeInTheDocument()
    })

    it('shows a failed push result when pushUpdate rejects', async () => {
        mockOverview.mockResolvedValue([makeAgent({ id: 'a1', update_available: true })])
        mockPushUpdate.mockRejectedValue(new Error('network error'))

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Select All Outdated')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Select All Outdated'))
        fireEvent.click(screen.getByText('Update 1 Agent'))

        await waitFor(() => expect(screen.getByText(/1 failed/)).toBeInTheDocument())
    })
})

describe('AgentManagement - AgentConfigPanel', () => {
    async function openConfigModal(agent: OverviewAgent, role = 'admin') {
        mockOverview.mockResolvedValue([agent])
        render(<AgentManagement user={makeUser(role)} />)
        await waitFor(() => expect(screen.getByText(agent.hostname)).toBeInTheDocument())
        fireEvent.click(screen.getByText(agent.hostname))
        await waitFor(() => expect(mockAgent).toHaveBeenCalledWith(agent.id))
    }

    it('shows read-only stats for a non-admin without fetching admin-only config', async () => {
        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }), 'viewer')
        expect(mockAgentConfig).not.toHaveBeenCalled()
        expect(screen.queryByText('Filesystems')).not.toBeInTheDocument()
    })

    it('shows ignore checklists for an admin and toggles an item', async () => {
        mockAgentDisk.mockResolvedValue([{ filesystem: 'ext4' }, { filesystem: 'ext4' }])
        mockAgentNetwork.mockResolvedValue([{ interface: 'eth0' }])
        mockAgentConfig.mockResolvedValue({ ignored_filesystems: [] })

        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('ext4')).toBeInTheDocument())

        fireEvent.click(screen.getByText('ext4'))
        await waitFor(() => expect(mockSetAgentConfig).toHaveBeenCalledWith('a1', 'ignored_filesystems', ['ext4']))
    })

    it('clears the config key instead of setting an empty list', async () => {
        mockAgentDisk.mockResolvedValue([{ filesystem: 'ext4' }])
        mockAgentConfig.mockResolvedValue({ ignored_filesystems: ['ext4'] })

        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('ext4')).toBeInTheDocument())

        fireEvent.click(screen.getByText('ext4')) // un-ignore it -> list becomes empty
        await waitFor(() => expect(mockDeleteAgentConfig).toHaveBeenCalledWith('a1', 'ignored_filesystems'))
    })

    it('changes the log level', async () => {
        mockAgentConfig.mockResolvedValue({ log_level: 'info' })
        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByDisplayValue('Info')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('Info'), { target: { value: 'debug' } })
        await waitFor(() => expect(mockSetAgentConfig).toHaveBeenCalledWith('a1', 'log_level', 'debug'))
    })

    it('shows upgrade instructions', async () => {
        mockUpgradeInstructions.mockResolvedValue({ type: 'systemd', steps: 'do the upgrade' })
        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('Upgrade instructions')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Upgrade instructions'))
        await waitFor(() => expect(screen.getByText('do the upgrade')).toBeInTheDocument())
    })

    it('goes straight to delete confirmation when there are no uninstall steps (unknown platform)', async () => {
        mockUninstallInstructions.mockResolvedValue({ type: '', steps: '' })
        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('Delete agent')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete agent'))
        await waitFor(() => expect(screen.getByText('Delete test-host-1 and all its data?')).toBeInTheDocument())
    })

    it('shows uninstall steps first when available, deferring the delete confirmation', async () => {
        mockUninstallInstructions.mockResolvedValue({ type: 'systemd', steps: 'uninstall steps here' })
        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('Delete agent')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete agent'))
        await waitFor(() => expect(screen.getByText('uninstall steps here')).toBeInTheDocument())
        expect(screen.queryByText('Delete test-host-1 and all its data?')).not.toBeInTheDocument()
    })

    it('deletes the agent and closes the modal on confirm', async () => {
        mockUninstallInstructions.mockResolvedValue({ type: '', steps: '' })
        mockDeleteAgent.mockResolvedValue(undefined)

        await openConfigModal(makeAgent({ id: 'a1', hostname: 'test-host-1' }))
        await waitFor(() => expect(screen.getByText('Delete agent')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Delete agent'))
        await waitFor(() => expect(screen.getByText('Confirm delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Confirm delete'))
        await waitFor(() => expect(mockDeleteAgent).toHaveBeenCalledWith('a1'))
        await waitFor(() => expect(screen.queryByText('Delete test-host-1 and all its data?')).not.toBeInTheDocument())
    })
})

describe('AgentManagement - ProvisionModal', () => {
    it('shows the provision modal, generates a token, and copies the non-numbered command lines', async () => {
        mockPlatforms.mockResolvedValue([makePlatform()])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('+ Provision Agent')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Provision Agent'))

        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Generate Token & Instructions'))

        await waitFor(() => expect(mockProvision).toHaveBeenCalledWith('spectra-agent-linux-amd64'))
        expect(screen.getByText('tok_abc123')).toBeInTheDocument()
    })

    it('resets to the platform picker via Provision Another', async () => {
        mockPlatforms.mockResolvedValue([makePlatform()])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('+ Provision Agent')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Provision Agent'))
        await waitFor(() => expect(screen.getByText('Generate Token & Instructions')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Generate Token & Instructions'))
        await waitFor(() => expect(screen.getByText('tok_abc123')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Provision Another'))
        expect(screen.getByText('Generate Token & Instructions')).toBeInTheDocument()
    })

    it('closes via Done', async () => {
        mockPlatforms.mockResolvedValue([makePlatform()])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('+ Provision Agent')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Provision Agent'))
        await waitFor(() => expect(screen.getByText('Generate Token & Instructions')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Generate Token & Instructions'))
        await waitFor(() => expect(screen.getByText('Done')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Done'))
        expect(screen.queryByText('Provision Agent')).not.toBeInTheDocument()
    })

    it('shows the error message on provisioning failure', async () => {
        mockPlatforms.mockResolvedValue([makePlatform()])
        mockProvision.mockRejectedValue(new Error('token generation failed'))

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('+ Provision Agent')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Provision Agent'))
        await waitFor(() => expect(screen.getByText('Generate Token & Instructions')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Generate Token & Instructions'))

        await waitFor(() => expect(screen.getByText('token generation failed')).toBeInTheDocument())
    })
})

describe('AgentManagement - Danger Zone', () => {
    it('is hidden for a non-admin', async () => {
        mockOverview.mockResolvedValue([])
        render(<AgentManagement user={makeUser('viewer')} />)
        await waitFor(() => expect(screen.getByText('Agent Management')).toBeInTheDocument())
        expect(screen.queryByText('Danger Zone')).not.toBeInTheDocument()
    })

    it('purges offline agents after confirmation and shows a result message that clears itself', async () => {
        vi.useFakeTimers()
        mockOverview.mockResolvedValue([])
        mockPurgeOffline.mockResolvedValue({ purged: 3 })

        render(<AgentManagement user={makeUser('admin')} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Purge Offline'))
        fireEvent.click(screen.getByText('Confirm'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        expect(screen.getByText('3 agents purged.')).toBeInTheDocument()
        expect(mockPurgeOffline).toHaveBeenCalledTimes(1)

        await act(async () => { await vi.advanceTimersByTimeAsync(3_000) })
        expect(screen.queryByText('3 agents purged.')).not.toBeInTheDocument()
    })

    it('revokes all tokens after confirmation', async () => {
        mockOverview.mockResolvedValue([])
        mockRevokeTokens.mockResolvedValue(undefined)

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Revoke Tokens')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Revoke Tokens'))
        fireEvent.click(screen.getByText('Confirm'))

        await waitFor(() => expect(screen.getByText('All tokens revoked.')).toBeInTheDocument())
    })

    it('shows a failure message when a danger action rejects', async () => {
        mockOverview.mockResolvedValue([])
        mockRevokeTokens.mockRejectedValue(new Error('forbidden'))

        render(<AgentManagement user={makeUser('admin')} />)
        await waitFor(() => expect(screen.getByText('Revoke Tokens')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Revoke Tokens'))
        fireEvent.click(screen.getByText('Confirm'))

        await waitFor(() => expect(screen.getByText('Action failed.')).toBeInTheDocument())
    })
})