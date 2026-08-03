import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { DiagnosticsPanel } from '../components/DiagnosticsPanel'
import type { CommandEntry } from '../types'

vi.mock('../api', () => ({
    api: {
        triggerLogs: vi.fn(),
        triggerNetwork: vi.fn(),
        triggerDisk: vi.fn(),
        commandResult: vi.fn(),
    },
}))

import { api } from '../api'
const mockTriggerLogs = api.triggerLogs as ReturnType<typeof vi.fn>
const mockTriggerNetwork = api.triggerNetwork as ReturnType<typeof vi.fn>
const mockTriggerDisk = api.triggerDisk as ReturnType<typeof vi.fn>
const mockCommandResult = api.commandResult as ReturnType<typeof vi.fn>

function pendingEntry(type: string): CommandEntry {
    return { id: 'cmd-1', type, agent_id: 'agent-1', queued_at: '2026-01-01T00:00:00.000Z', done: false }
}

function doneEntry(type: string, payload: unknown, error?: string): CommandEntry {
    return {
        id: 'cmd-1',
        type,
        agent_id: 'agent-1',
        queued_at: '2026-01-01T00:00:00.000Z',
        done: true,
        result: { id: 'cmd-1', type, payload, ...(error ? { error } : {}) },
    }
}

// This component has a two-hop async chain: triggering a command resolves
// and sets activeCmd, which THEN causes useCommandPoller's own effect to
// fire and its own async poll to resolve. Empirically this needs about 4
// ticks to fully settle - see hooks.usePolling.test.tsx for why even a
// single hop sometimes needs more than one tick under fake timers.
async function flush() {
    for (let i = 0; i < 4; i++) {
        await vi.advanceTimersByTimeAsync(0)
    }
}

beforeEach(() => {
    mockTriggerLogs.mockReset()
    mockTriggerNetwork.mockReset()
    mockTriggerDisk.mockReset()
    mockCommandResult.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('DiagnosticsPanel - triggering commands', () => {
    it('triggers a logs fetch with the selected log level', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('FETCH_LOGS'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()

        expect(mockTriggerLogs).toHaveBeenCalledWith('agent-1', 'WARNING')
    })

    it('sends the changed log level', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('FETCH_LOGS'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.change(screen.getByDisplayValue('WARNING'), { target: { value: 'ERROR' } })
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()

        expect(mockTriggerLogs).toHaveBeenCalledWith('agent-1', 'ERROR')
    })

    it('triggers netstat', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('NETWORK_DIAG'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Netstat'))
        await flush()

        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'netstat')
    })

    it('disables Ping until a target is entered, then triggers it with the trimmed value', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('NETWORK_DIAG'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        expect(screen.getByText('Ping')).toBeDisabled()

        fireEvent.change(screen.getByPlaceholderText('Ping target (IP or hostname)'), {
            target: { value: '  10.0.0.1  ' },
        })
        expect(screen.getByText('Ping')).not.toBeDisabled()

        fireEvent.click(screen.getByText('Ping'))
        await flush()
        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'ping', '10.0.0.1')
    })

    it('triggers Ping on Enter in the target field', async () => {
        vi.useFakeTimers()
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('NETWORK_DIAG'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        const input = screen.getByPlaceholderText('Ping target (IP or hostname)')
        fireEvent.change(input, { target: { value: '10.0.0.1' } })
        fireEvent.keyDown(input, { key: 'Enter' })
        await flush()

        expect(mockTriggerNetwork).toHaveBeenCalledWith('agent-1', 'ping', '10.0.0.1')
    })

    it('disables Traceroute until a target is entered', () => {
        render(<DiagnosticsPanel agentId="agent-1" />)
        expect(screen.getByText('Traceroute')).toBeDisabled()
        fireEvent.change(screen.getByPlaceholderText('Traceroute target'), { target: { value: 'example.com' } })
        expect(screen.getByText('Traceroute')).not.toBeDisabled()
    })

    it('scans disk with the entered path, defaulting an empty path to "/"', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('DISK_USAGE'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(mockTriggerDisk).toHaveBeenCalledWith('agent-1', '/', 20)
    })

    it('sends a custom top-N value', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('DISK_USAGE'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.change(screen.getByDisplayValue('20'), { target: { value: '50' } })
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(mockTriggerDisk).toHaveBeenCalledWith('agent-1', '/', 50)
    })

    it('falls back to 20 when the top-N field is cleared', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('DISK_USAGE'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.change(screen.getByDisplayValue('20'), { target: { value: '' } })
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(mockTriggerDisk).toHaveBeenCalledWith('agent-1', '/', 20)
    })

    it('disables every trigger button while a command is running', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('FETCH_LOGS')) // never completes

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()

        expect(screen.getByText('Fetch Logs')).toBeDisabled()
        expect(screen.getByText('Netstat')).toBeDisabled()
        expect(screen.getByText('Scan Disk')).toBeDisabled()
    })

    it('shows an error and re-enables buttons when triggering fails', async () => {
        mockTriggerLogs.mockRejectedValue(new Error('agent offline'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))

        await waitFor(() => expect(screen.getByText('agent offline')).toBeInTheDocument())
        expect(screen.getByText('Fetch Logs')).not.toBeDisabled()
    })

    it('shows a polling error if commandResult fails, and stops polling', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockRejectedValue(new Error('lost connection'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()

        expect(screen.getByText('lost connection')).toBeInTheDocument()

        mockCommandResult.mockClear()
        await act(async () => {
            await vi.advanceTimersByTimeAsync(1000)
        })
        expect(mockCommandResult).not.toHaveBeenCalled()
    })
})

describe('DiagnosticsPanel - polling behavior', () => {
    it('polls every second until the command is done, then stops', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('FETCH_LOGS'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()
        expect(mockCommandResult).toHaveBeenCalledTimes(1) // immediate first poll

        await act(async () => {
            await vi.advanceTimersByTimeAsync(1000)
        })
        expect(mockCommandResult).toHaveBeenCalledTimes(2)

        mockCommandResult.mockResolvedValue(doneEntry('FETCH_LOGS', []))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(1000)
        })
        expect(mockCommandResult).toHaveBeenCalledTimes(3)

        mockCommandResult.mockClear()
        await act(async () => {
            await vi.advanceTimersByTimeAsync(3000)
        })
        expect(mockCommandResult).not.toHaveBeenCalled() // stopped after done
    })
})

describe('DiagnosticsPanel - result dispatch', () => {
    it('shows a fullscreen overlay spinner while logs are still running', async () => {
        vi.useFakeTimers()
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(pendingEntry('FETCH_LOGS'))

        const { container } = render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()

        expect(container.querySelector('[style*="position: fixed"]')).toBeInTheDocument()
    })

    it('shows the command error when the result contains one', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('DISK_USAGE', null, 'permission denied'))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(screen.getByText('Error: permission denied')).toBeInTheDocument()
    })

    it('shows "No data returned." for a done command with no payload', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('DISK_USAGE', null))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(screen.getByText('No data returned.')).toBeInTheDocument()
    })

    it('falls back to raw JSON for an unrecognized command type', async () => {
        vi.useFakeTimers()
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('SOME_OTHER_TYPE', { foo: 'bar' }))

        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()

        expect(screen.getByText(/"foo": "bar"/)).toBeInTheDocument()
    })
})

describe('DiagnosticsPanel - LogResults', () => {
    async function triggerLogsWithEntries(entries: unknown[]) {
        mockTriggerLogs.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('FETCH_LOGS', entries))
        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Fetch Logs'))
        await flush()
    }

    it('shows the empty-state modal for zero log entries', async () => {
        vi.useFakeTimers()
        await triggerLogsWithEntries([])
        expect(screen.getByText('No log entries found at this severity level.')).toBeInTheDocument()
    })

    it('shows a header count and filters by clicking a severity level', async () => {
        vi.useFakeTimers()
        await triggerLogsWithEntries([
            { timestamp: 1, source: 'sshd', level: 'ERROR', message: 'auth failure' },
            { timestamp: 2, source: 'sshd', level: 'WARNING', message: 'retry' },
            { timestamp: 3, source: 'kernel', level: 'WARNING', message: 'thermal' },
        ])

        expect(screen.getByText('System Logs (3 of 3)')).toBeInTheDocument()

        fireEvent.click(screen.getByText('WARNING (2)'))
        expect(screen.getByText('System Logs (2 WARNING of 3)')).toBeInTheDocument()
        expect(screen.queryByText('auth failure')).not.toBeInTheDocument()

        fireEvent.click(screen.getByText('All (3)'))
        expect(screen.getByText('System Logs (3 of 3)')).toBeInTheDocument()
    })

    it('closes the modal via the × button', async () => {
        vi.useFakeTimers()
        await triggerLogsWithEntries([
            { timestamp: 1, source: 'sshd', level: 'ERROR', message: 'auth failure' },
        ])
        expect(screen.getByText('auth failure')).toBeInTheDocument()

        fireEvent.click(screen.getByText('×'))
        expect(screen.queryByText('auth failure')).not.toBeInTheDocument()
    })
})

describe('DiagnosticsPanel - DiskResults', () => {
    async function triggerDiskWithReport(report: Record<string, unknown>) {
        mockTriggerDisk.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('DISK_USAGE', report))
        render(<DiagnosticsPanel agentId="agent-1" />)
        fireEvent.click(screen.getByText('Scan Disk'))
        await flush()
    }

    it('shows the scan summary', async () => {
        vi.useFakeTimers()
        await triggerDiskWithReport({
            root: '/', top_dirs: [], top_files: [], scanned_dirs: 100, scanned_files: 5000,
            error_count: 0, partial: false, duration_ms: 250,
        })
        expect(screen.getByText('Root: /')).toBeInTheDocument()
        expect(screen.getByText('Scanned: 100 dirs, 5000 files')).toBeInTheDocument()
        expect(screen.getByText('Duration: 250ms')).toBeInTheDocument()
    })

    it('pluralizes the error count correctly', async () => {
        vi.useFakeTimers()
        await triggerDiskWithReport({
            root: '/', top_dirs: [], top_files: [], scanned_dirs: 1, scanned_files: 1,
            error_count: 1, partial: false, duration_ms: 1,
        })
        expect(screen.getByText('1 error')).toBeInTheDocument()
    })

    it('shows a partial-scan flag', async () => {
        vi.useFakeTimers()
        await triggerDiskWithReport({
            root: '/', top_dirs: [], top_files: [], scanned_dirs: 1, scanned_files: 1,
            error_count: 0, partial: true, duration_ms: 1,
        })
        expect(screen.getByText('Partial scan')).toBeInTheDocument()
    })

    it('lists top directories and files with formatted sizes', async () => {
        vi.useFakeTimers()
        await triggerDiskWithReport({
            root: '/', scanned_dirs: 1, scanned_files: 1, error_count: 0, partial: false, duration_ms: 1,
            top_dirs: [{ path: '/var/log', size: 1024, count: 42 }],
            top_files: [{ path: '/var/log/big.log', size: 1024 }],
        })
        expect(screen.getByText('/var/log')).toBeInTheDocument()
        expect(screen.getAllByText('1.0 KB')).toHaveLength(2)
        expect(screen.getByText('42')).toBeInTheDocument()
    })
})

describe('DiagnosticsPanel - NetworkResults', () => {
    async function triggerNetworkWithReport(action: string, report: Record<string, unknown>) {
        mockTriggerNetwork.mockResolvedValue({ command_id: 'cmd-1', message: 'queued' })
        mockCommandResult.mockResolvedValue(doneEntry('NETWORK_DIAG', { action, ...report }))
        render(<DiagnosticsPanel agentId="agent-1" />)
        const button = action === 'netstat' ? 'Netstat' : action === 'ping' ? 'Ping' : 'Traceroute'
        if (button === 'Ping') {
            fireEvent.change(screen.getByPlaceholderText('Ping target (IP or hostname)'), { target: { value: 'x' } })
        }
        if (button === 'Traceroute') {
            fireEvent.change(screen.getByPlaceholderText('Traceroute target'), { target: { value: 'x' } })
        }
        fireEvent.click(screen.getByText(button))
        await flush()
    }

    it('shows ping results with rtt converted from nanoseconds to milliseconds', async () => {
        vi.useFakeTimers()
        await triggerNetworkWithReport('ping', {
            target: '10.0.0.1',
            ping_results: [{ seq: 1, success: true, rtt: 2_500_000, response: 'reply', peer: '10.0.0.1' }],
        })
        expect(screen.getByText(/rtt=2\.50ms/)).toBeInTheDocument()
    })

    it('shows traceroute raw output', async () => {
        vi.useFakeTimers()
        await triggerNetworkWithReport('traceroute', { raw_output: '1  10.0.0.1  1.2 ms' })
        expect(screen.getByText('1 10.0.0.1 1.2 ms')).toBeInTheDocument()
    })

    it('delegates netstat to the filterable modal', async () => {
        vi.useFakeTimers()
        await triggerNetworkWithReport('netstat', {
            netstat: [
                { proto: 'tcp', local_addr: '0.0.0.0', local_port: 22, remote_addr: '0.0.0.0', remote_port: 0, state: 'LISTEN' },
                { proto: 'tcp', local_addr: '10.0.0.5', local_port: 443, remote_addr: '1.2.3.4', remote_port: 51000, state: 'ESTABLISHED' },
            ],
        })

        expect(screen.getByText('Netstat (2 of 2)')).toBeInTheDocument()

        fireEvent.click(screen.getByText('LISTEN (1)'))
        expect(screen.getByText('Netstat (1 of 2)')).toBeInTheDocument()
        expect(screen.queryByText('ESTABLISHED')).not.toBeInTheDocument()

        fireEvent.click(screen.getByText('All States'))
        fireEvent.change(screen.getByPlaceholderText('Filter by address...'), { target: { value: '1.2.3.4' } })
        expect(screen.getByText('Netstat (1 of 2)')).toBeInTheDocument()
    })
})