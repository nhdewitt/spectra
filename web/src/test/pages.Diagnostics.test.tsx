import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Diagnostics } from '../pages/Diagnostics'
import type { OverviewAgent, CommandEntry } from '../types'

vi.mock('../api', () => ({
    api: {
        triggerNetwork: vi.fn(),
        triggerDisk: vi.fn(),
        triggerLogs: vi.fn(),
        commandResult: vi.fn(),
        overviewPage: vi.fn(),
    },
}))

import { api } from '../api'
const mockTriggerNetwork = api.triggerNetwork as ReturnType<typeof vi.fn>
const mockTriggerDisk = api.triggerDisk as ReturnType<typeof vi.fn>
const mockTriggerLogs = api.triggerLogs as ReturnType<typeof vi.fn>
const mockCommandResult = api.commandResult as ReturnType<typeof vi.fn>
const mockOverviewPage = api.overviewPage as ReturnType<typeof vi.fn>

function makeAgent(overrides: Partial<OverviewAgent> = {}): OverviewAgent {
    return {
        id: 'agent-1',
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

function doneEntry(payload: unknown, error?: string): CommandEntry {
    return {
        id: 'cmd-1',
        type: 'x',
        agent_id: 'agent-1',
        queued_at: '',
        done: true,
        result: { id: 'cmd-1', type: 'x', payload, ...(error ? { error } : {}) },
    }
}

// This component has a three-hop async chain: triggering resolves and sets
// activeCmd, which causes the poller's own effect to fire and resolve,
// which THEN triggers a separate effect that converts entry.done into
// results/cardStatus updates. Empirically needs about 6 ticks to fully
// settle - see hooks.usePolling.test.tsx for why even a single hop
// sometimes needs more than one tick under fake timers.
async function flush() {
    for (let i = 0; i < 6; i++) {
        await vi.advanceTimersByTimeAsync(0)
    }
}

const noop = () => {}

beforeEach(() => {
    mockTriggerNetwork.mockReset()
    mockTriggerDisk.mockReset()
    mockTriggerLogs.mockReset()
    mockCommandResult.mockReset()
    mockOverviewPage.mockReset().mockResolvedValue({ agents: [], page: 1, size: 20 })
})

afterEach(() => {
    vi.useRealTimers()
})

describe('Diagnostics - agent picker', () => {
    it('shows a prompt and no cards when no agent is selected', () => {
        render(<Diagnostics selectedAgent={null} onSelectAgent={noop} />)
        expect(screen.getByText('Select an agent to run diagnostics.')).toBeInTheDocument()
        expect(screen.queryByText('Ping')).not.toBeInTheDocument()
    })

    it('selecting an agent from the picker calls onSelectAgent', async () => {
        const onSelectAgent = vi.fn()
        const agent = makeAgent({ id: 'agent-1', hostname: 'test-host-1' })
        mockOverviewPage.mockResolvedValue({ agents: [agent], page: 1, size: 20 })

        render(<Diagnostics selectedAgent={null} onSelectAgent={onSelectAgent} />)

        fireEvent.click(screen.getByText('Select an agent...'))
        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())

        fireEvent.click(screen.getByText('test-host-1'))
        expect(onSelectAgent).toHaveBeenCalledWith(agent)
    })

    it('shows the diagnostic cards once an agent is selected', () => {
        const agent = makeAgent({ hostname: 'test-host-1' })
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        expect(screen.getByText('Ping')).toBeInTheDocument()
        expect(screen.getByText('Traceroute')).toBeInTheDocument()
        expect(screen.getByText('Netstat')).toBeInTheDocument()
        expect(screen.getByText('Disk Scan')).toBeInTheDocument()
        expect(screen.getByText('Fetch Logs')).toBeInTheDocument()
        expect(screen.getAllByText('test-host-1').length).toBeGreaterThan(0) // title + status pill
    })
})

describe('Diagnostics - ping/traceroute confirm-then-run', () => {
    it('flashes the target field instead of running when the target is empty', () => {
        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.click(screen.getByText('Ping'))
        expect(mockTriggerNetwork).not.toHaveBeenCalled()
    })

    it('requires a second click to actually run once the target is filled', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.change(screen.getByPlaceholderText('IP or hostname'), { target: { value: '10.0.0.1' } })
        fireEvent.click(screen.getByText('Ping'))
        expect(mockTriggerNetwork).not.toHaveBeenCalled() // first click only confirms

        fireEvent.click(screen.getByText('Ping'))
        await flush()
        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'ping', '10.0.0.1')
    })

    it('resets the confirmation if the target is edited between clicks', () => {
        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.change(screen.getByPlaceholderText('IP or hostname'), { target: { value: '10.0.0.1' } })
        fireEvent.click(screen.getByText('Ping')) // confirms

        fireEvent.change(screen.getByPlaceholderText('IP or hostname'), { target: { value: '10.0.0.2' } }) // resets
        fireEvent.click(screen.getByText('Ping')) // should just re-confirm, not run

        expect(mockTriggerNetwork).not.toHaveBeenCalled()
    })

    it('runs ping immediately on Enter in the target field, bypassing the confirm click', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        const input = screen.getByPlaceholderText('IP or hostname')
        fireEvent.change(input, { target: { value: '10.0.0.1' } })
        fireEvent.keyDown(input, { key: 'Enter' })
        await flush()

        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'ping', '10.0.0.1')
    })
})

describe('Diagnostics - netstat (no target/confirm needed)', () => {
    it('runs immediately on a single click', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.click(screen.getByText('Netstat'))
        await flush()
        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'netstat')
    })
})

describe('Diagnostics - disk/logs options panels', () => {
    it('toggles the disk options panel open and closed without running anything', () => {
        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.click(screen.getByText('Disk Scan'))
        expect(screen.getByText('Path to scan')).toBeInTheDocument()
        expect(mockTriggerDisk).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Disk Scan'))
        expect(screen.queryByText('Path to scan')).not.toBeInTheDocument()
    })

    it('runs a disk scan with the entered path and top-N from the options panel', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Disk Scan'))

        fireEvent.change(screen.getByPlaceholderText('/'), { target: { value: '/data' } })
        fireEvent.change(screen.getByDisplayValue('20'), { target: { value: '50' } })
        fireEvent.click(screen.getByText('Run Disk Scan'))
        await flush()

        expect(mockTriggerDisk).toHaveBeenCalledWith('agent-1', '/data', 50)
    })

    it('opening the log options panel closes the disk panel, and vice versa', () => {
        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.click(screen.getByText('Disk Scan'))
        expect(screen.getByText('Path to scan')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Fetch Logs'))
        expect(screen.queryByText('Path to scan')).not.toBeInTheDocument()
        expect(screen.getByText('Minimum Severity')).toBeInTheDocument()
    })

    it('runs a log fetch with the selected severity level', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Fetch Logs'))

        fireEvent.change(screen.getByDisplayValue('WARNING'), { target: { value: 'ERROR' } })
        fireEvent.click(screen.getByText('Fetch Logs', { selector: 'button' }))
        await flush()

        expect(mockTriggerLogs).toHaveBeenCalledWith('agent-1', 'ERROR')
    })
})

describe('Diagnostics - results and card status', () => {
    it('shows the RUNNING badge on the active card and disables the others', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue({ id: 'cmd-1', type: 'x', agent_id: 'a', queued_at: '', done: false })

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Netstat'))
        await flush()

        expect(screen.getByText('⟳ RUNNING')).toBeInTheDocument()
        expect(screen.getByText('Ping').closest('button')).toBeDisabled()
    })

    it('shows COMPLETE and the parsed result once the command finishes', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(
            doneEntry({ action: 'ping', target: '10.0.0.1', ping_results: [{ seq: 1, success: true, rtt: 1_500_000, response: 'reply', peer: '10.0.0.1' }] })
        )

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.change(screen.getByPlaceholderText('IP or hostname'), { target: { value: '10.0.0.1' } })
        fireEvent.click(screen.getByText('Ping'))
        fireEvent.click(screen.getByText('Ping'))
        await flush()

        expect(screen.getByText('✓ COMPLETE')).toBeInTheDocument()
        expect(screen.getByText('ping output')).toBeInTheDocument()
        expect(screen.getByText(/rtt=1\.50ms/)).toBeInTheDocument()
    })

    it('shows an ERROR badge and message when the command result contains an error', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry(null, 'permission denied'))

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Netstat'))
        await flush()

        expect(screen.getByText('✗ ERROR')).toBeInTheDocument()
        expect(screen.getByText('Error: permission denied')).toBeInTheDocument()
    })

    it('shows a send error when triggering fails outright', async () => {
        mockTriggerNetwork.mockRejectedValue(new Error('agent unreachable'))
        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)

        fireEvent.click(screen.getByText('Netstat'))
        await waitFor(() => expect(screen.getByText('agent unreachable')).toBeInTheDocument())
    })

    it('retains a completed tool\'s card status after switching to a different tool', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry({ action: 'netstat', netstat: [] }))

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Netstat'))
        await flush()
        expect(screen.getByText('✓ COMPLETE')).toBeInTheDocument()

        // Now trigger a different tool - netstat's card should stay marked complete.
        mockCommandResult.mockResolvedValue({ id: 'cmd-2', type: 'x', agent_id: 'a', queued_at: '', done: false })
        fireEvent.click(screen.getByText('Disk Scan'))
        fireEvent.click(screen.getByText('Run Disk Scan'))
        await flush()

        const netstatCard = screen.getByText('Netstat').closest('button')!
        expect(netstatCard.textContent).toContain('COMPLETE')
    })
})

describe('Diagnostics - LogResultsInline (fetched logs result)', () => {
    async function triggerLogsWithEntries(entries: Array<Record<string, unknown>>) {
        // Real timers throughout: this block never manipulates elapsed
        // time, unlike the update-flow tests elsewhere in this file, so
        // fake timers bought nothing here and proved unreliable for the
        // plain synchronous pagination clicks in the tests below.
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry(entries))

        const agent = makeAgent()
        render(<Diagnostics selectedAgent={agent} onSelectAgent={noop} />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        fireEvent.click(screen.getByText('Fetch Logs', { selector: 'button' }))
        await waitFor(() => {
            const hasEmptyState = screen.queryByText('No log entries found at or below this severity level.')
            const hasEntries = screen.queryByText(/\d+ entries$/)
            if (!hasEmptyState && !hasEntries) throw new Error('log result not rendered yet')
        })
    }

    it('shows the empty state for zero log entries', async () => {
        await triggerLogsWithEntries([])
        expect(screen.getByText('No log entries found at or below this severity level.')).toBeInTheDocument()
    })

    it('renders entries with a level filter button per distinct level, counted and sorted by severity', async () => {
        await triggerLogsWithEntries([
            { timestamp: 1, source: 'sshd', level: 'ERROR', message: 'auth failure' },
            { timestamp: 2, source: 'sshd', level: 'WARNING', message: 'retry' },
            { timestamp: 3, source: 'kernel', level: 'WARNING', message: 'thermal' },
        ])

        expect(screen.getByText('All (3)')).toBeInTheDocument()
        expect(screen.getByText('ERROR (1)')).toBeInTheDocument()
        expect(screen.getByText('WARNING (2)')).toBeInTheDocument()
        expect(screen.getByText('auth failure')).toBeInTheDocument()
        expect(screen.getByText('3 entries')).toBeInTheDocument()
    })

    it('filters cumulatively: selecting a level shows entries at or more severe than it', async () => {
        await triggerLogsWithEntries([
            { timestamp: 1, source: 'sshd', level: 'CRITICAL', message: 'disk full' },
            { timestamp: 2, source: 'sshd', level: 'WARNING', message: 'retry' },
            { timestamp: 3, source: 'kernel', level: 'DEBUG', message: 'noise' },
        ])

        fireEvent.click(screen.getByText('WARNING (1)'))

        expect(screen.getByText('disk full')).toBeInTheDocument() // CRITICAL is more severe than WARNING
        expect(screen.getByText('retry')).toBeInTheDocument()
        expect(screen.queryByText('noise')).not.toBeInTheDocument() // DEBUG is less severe
        expect(screen.getByText('2 entries')).toBeInTheDocument()
    })

    it('deselects the active level filter on a second click, returning to all entries', async () => {
        await triggerLogsWithEntries([
            { timestamp: 1, source: 'sshd', level: 'ERROR', message: 'auth failure' },
            { timestamp: 2, source: 'kernel', level: 'DEBUG', message: 'noise' },
        ])

        fireEvent.click(screen.getByText('ERROR (1)'))
        expect(screen.queryByText('noise')).not.toBeInTheDocument()

        fireEvent.click(screen.getByText('ERROR (1)')) // toggle off
        expect(screen.getByText('noise')).toBeInTheDocument()
        expect(screen.getByText('All (2)')).toBeInTheDocument()
    })

    it('paginates when there are more than 50 log entries', async () => {
        const entries = Array.from({ length: 60 }, (_, i) => ({
            timestamp: i, source: 'sshd', level: 'INFO', message: `entry-${i}`,
        }))
        await triggerLogsWithEntries(entries)

        expect(screen.getByText('entry-0')).toBeInTheDocument()
        expect(screen.queryByText('entry-59')).not.toBeInTheDocument()

        fireEvent.click(screen.getByText('Next →'))
        await waitFor(() => expect(screen.getByText('entry-59')).toBeInTheDocument(), { timeout: 5000 })
    }, 15000)

    it('resets to page 1 when the level filter changes', async () => {
        const entries = Array.from({ length: 60 }, (_, i) => ({
            timestamp: i, source: 'sshd', level: 'INFO', message: `entry-${i}`,
        }))
        await triggerLogsWithEntries(entries)

        fireEvent.click(screen.getByText('Next →'))
        await waitFor(() => expect(screen.getByText('entry-59')).toBeInTheDocument(), { timeout: 5000 })

        fireEvent.click(screen.getByText('INFO (60)'))
        await waitFor(() => expect(screen.getByText('entry-0')).toBeInTheDocument(), { timeout: 5000 })
        expect(screen.queryByText('entry-59')).not.toBeInTheDocument()
    }, 20000)
})