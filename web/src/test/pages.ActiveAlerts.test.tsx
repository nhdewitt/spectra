import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { ActiveAlerts } from '../pages/ActiveAlerts'
import type { AlertEvent } from '../types'

vi.mock('../api', () => ({
    api: { activeAlerts: vi.fn(), alertHistory: vi.fn() },
}))

import { api } from '../api'
const mockActiveAlerts = api.activeAlerts as ReturnType<typeof vi.fn>
const mockAlertHistory = api.alertHistory as ReturnType<typeof vi.fn>

const NOW = '2026-01-01T12:00:00.000Z'

function makeEvent(overrides: Partial<AlertEvent> = {}): AlertEvent {
    return {
        id: 'evt-1',
        rule_id: 'rule-1',
        agent_id: 'agent-1',
        fired_at: '2026-01-01T11:55:00.000Z', // 5 minutes before NOW
        resolved_at: null,
        last_notified_at: null,
        condition_snapshot: null,
        rule_name: 'High CPU',
        condition_type: 'agent_offline',
        hostname: 'test-host-1',
        ...overrides,
    }
}

beforeEach(() => {
    mockActiveAlerts.mockReset().mockResolvedValue([])
    mockAlertHistory.mockReset().mockResolvedValue([])
})

afterEach(() => {
    vi.useRealTimers()
})

describe('ActiveAlerts - Active section', () => {
    it('shows a loading spinner, then renders active alerts', async () => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date(NOW))
        mockActiveAlerts.mockResolvedValue([makeEvent({ rule_name: 'High CPU', hostname: 'test-host-1' })])

        const { container } = render(<ActiveAlerts />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('High CPU')).toBeInTheDocument()
        expect(screen.getByText('test-host-1')).toBeInTheDocument()
    })

    it('shows an error message', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockRejectedValue(new Error('connection refused'))

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('connection refused')).toBeInTheDocument()
    })

    it('shows the all-clear message when there are no active alerts', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('No active alerts. All monitored conditions are healthy.')).toBeInTheDocument()
    })

    it('shows a dash for a missing hostname', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([makeEvent({ hostname: undefined })])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    })

    it('polls for fresh active alerts every 10 seconds', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(mockActiveAlerts).toHaveBeenCalledTimes(1)

        await act(async () => {
            await vi.advanceTimersByTimeAsync(10_000)
        })
        expect(mockActiveAlerts).toHaveBeenCalledTimes(2)
    })

    it('shows the condition label and a relative fired time', async () => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date(NOW))
        mockActiveAlerts.mockResolvedValue([makeEvent({ condition_type: 'service_down', fired_at: '2026-01-01T11:55:00.000Z' })])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('Service Down')).toBeInTheDocument()
        expect(screen.getByText('5m ago')).toBeInTheDocument()
    })
})

describe('ActiveAlerts - formatSnapshot per condition type', () => {
    it('formats an agent_offline snapshot', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({
                condition_type: 'agent_offline',
                condition_snapshot: { last_seen: '2026-01-01T00:00:00.000Z', seconds_silent: 305 },
            }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText(/Silent 5m/)).toBeInTheDocument()
    })

    it('formats a disk_prediction snapshot with a projection', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({
                condition_type: 'disk_prediction',
                condition_snapshot: { mount: '/data', used_pct: 91.234, hours_remaining: 2 },
            }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('/data at 91.2% — full in ~2h')).toBeInTheDocument()
    })

    it('formats a disk_prediction snapshot with no projection available', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({
                condition_type: 'disk_prediction',
                condition_snapshot: { mount: '/data', used_pct: 91 },
            }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('/data at 91.0% — projection unavailable')).toBeInTheDocument()
    })

    it('formats a service_down snapshot with a status', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({
                condition_type: 'service_down',
                condition_snapshot: { service_name: 'nginx', last_status: 'failed' },
            }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('nginx not healthy (failed)')).toBeInTheDocument()
    })

    it('formats a service_down snapshot with no status', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({
                condition_type: 'service_down',
                condition_snapshot: { service_name: 'nginx' },
            }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getByText('nginx not healthy')).toBeInTheDocument()
    })

    it('falls back to a dash for a missing or malformed snapshot', async () => {
        vi.useFakeTimers()
        mockActiveAlerts.mockResolvedValue([
            makeEvent({ condition_type: 'agent_offline', condition_snapshot: null }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    })
})

describe('ActiveAlerts - History section', () => {
    it('shows a loading spinner, then renders history rows', async () => {
        mockAlertHistory.mockResolvedValue([makeEvent({ rule_name: 'High CPU' })])

        render(<ActiveAlerts />)
        await waitFor(() => expect(screen.getAllByText('High CPU').length).toBeGreaterThan(0))
        expect(mockAlertHistory).toHaveBeenCalledWith(200, 0)
    })

    it('shows an error message', async () => {
        mockAlertHistory.mockRejectedValue(new Error('history unavailable'))

        render(<ActiveAlerts />)
        await waitFor(() => expect(screen.getByText('history unavailable')).toBeInTheDocument())
    })

    it('shows "No alert history yet." for an empty result', async () => {
        mockAlertHistory.mockResolvedValue([])

        render(<ActiveAlerts />)
        await waitFor(() => expect(screen.getByText('No alert history yet.')).toBeInTheDocument())
    })

    it('shows Resolved status and a resolved time for a resolved event', async () => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date(NOW))
        mockAlertHistory.mockResolvedValue([
            makeEvent({ resolved_at: '2026-01-01T11:58:00.000Z' }),
        ])

        render(<ActiveAlerts />)
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(screen.getAllByText('Resolved').length).toBeGreaterThan(1) // header + badge
        expect(screen.getByText('2m ago')).toBeInTheDocument() // the Resolved column's relative time
    })

    it('shows Firing status and a dash in the Resolved column for an unresolved event', async () => {
        mockAlertHistory.mockResolvedValue([makeEvent({ resolved_at: null })])

        render(<ActiveAlerts />)
        await waitFor(() => expect(screen.getByText('Firing')).toBeInTheDocument())
        expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    })

    it('paginates when there are more than 20 history entries', async () => {
        const events = Array.from({ length: 25 }, (_, i) => makeEvent({ id: `evt-${i}`, rule_name: `Rule ${i}` }))
        mockAlertHistory.mockResolvedValue(events)

        render(<ActiveAlerts />)
        await waitFor(() => expect(screen.getByText('Rule 0')).toBeInTheDocument(), { timeout: 5000 })

        expect(screen.queryByText('Rule 24')).not.toBeInTheDocument()
        fireEvent.click(screen.getByText('Next →'))
        await waitFor(() => expect(screen.getByText('Rule 24')).toBeInTheDocument(), { timeout: 5000 })
    }, 15000)
})