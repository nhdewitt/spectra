import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PiPanels } from '../components/PiPanels'
import type { PiMetric, RangeSelection } from '../types'

let mockUseMetricReturn: { data: PiMetric[]; loading: boolean; error: string | null } = {
    data: [],
    loading: false,
    error: null,
}

vi.mock('../hooks/useMetric', () => ({
    useMetric: () => mockUseMetricReturn,
}))

vi.mock('../components/MetricChart', () => ({
    MetricChart: (props: { title: string }) => <div data-testid={`chart-${props.title}`} />,
}))

function makePiMetric(overrides: Partial<PiMetric> = {}): PiMetric {
    return {
        time: '2026-01-01T00:00:00.000Z',
        agent_id: 'agent-1',
        metric_type: 'pi',
        arm_freq_hz: 1_500_000_000,
        core_freq_hz: 500_000_000,
        gpu_freq_hz: 500_000_000,
        core_volts: 0.85,
        sdram_c_volts: 1.1,
        sdram_i_volts: 1.2,
        sdram_p_volts: 1.3,
        soft_temp_limit: 0,
        throttled: false,
        under_voltage: false,
        freq_capped: false,
        undervoltage_occurred: false,
        freq_cap_occurred: false,
        throttled_occurred: false,
        soft_temp_limit_occurred: false,
        gpu_mem_total: 76,
        gpu_mem_used: 32,
        gpu_temp: 45,
        ...overrides,
    }
}

const oneHour: RangeSelection = { type: 'quick', range: '1h' }

describe('PiPanels', () => {
    it('renders nothing when not loading, no error, and no data', () => {
        mockUseMetricReturn = { data: [], loading: false, error: null }
        const { container } = render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(container.firstChild).toBeNull()
    })

    it('renders the charts once data is present', () => {
        mockUseMetricReturn = { data: [makePiMetric()], loading: false, error: null }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)

        expect(screen.getByTestId('chart-GPU Temperature')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Frequencies')).toBeInTheDocument()
        expect(screen.getByTestId('chart-Voltages')).toBeInTheDocument()
        expect(screen.getByTestId('chart-GPU Memory')).toBeInTheDocument()
    })

    it('hides the throttle summary while still loading, even if data is present', () => {
        mockUseMetricReturn = { data: [makePiMetric()], loading: true, error: null }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.queryByText('Throttle Status')).not.toBeInTheDocument()
    })

    it('shows the throttle summary once loaded with data', () => {
        mockUseMetricReturn = { data: [makePiMetric()], loading: false, error: null }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText('Throttle Status')).toBeInTheDocument()
    })

    it('reflects the latest reading\'s throttled state', () => {
        mockUseMetricReturn = {
            data: [makePiMetric({ throttled: false }), makePiMetric({ throttled: true })],
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText('Throttled: Yes')).toBeInTheDocument()
    })

    it('reflects the latest reading\'s under_voltage state independently of throttled (regression)', () => {
        // throttled=true but under_voltage=false on the latest reading -
        // Undervoltage must read No, not mirror Throttled's Yes.
        mockUseMetricReturn = {
            data: [makePiMetric({ throttled: true, under_voltage: false })],
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText('Throttled: Yes')).toBeInTheDocument()
        expect(screen.getByText('Undervoltage: No')).toBeInTheDocument()
    })

    it('shows Undervoltage: Yes when under_voltage is true, even if throttled is false (regression)', () => {
        mockUseMetricReturn = {
            data: [makePiMetric({ throttled: false, under_voltage: true })],
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText('Throttled: No')).toBeInTheDocument()
        expect(screen.getByText('Undervoltage: Yes')).toBeInTheDocument()
    })

    it('shows "No throttle events" when data exists but nothing actually happened (regression)', () => {
        mockUseMetricReturn = {
            data: [makePiMetric(), makePiMetric(), makePiMetric()], // all-clear readings
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText('No throttle events in last 1h.')).toBeInTheDocument()
    })

    it('summarizes actual throttle events by type, correctly pluralized', () => {
        mockUseMetricReturn = {
            data: [
                makePiMetric({ throttled: true }),
                makePiMetric({ under_voltage: true }),
            ],
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText(/1 throttle, 1 undervoltage/)).toBeInTheDocument()
        expect(screen.getByText(/events$/)).toBeInTheDocument() // 2 total events - plural
    })

    it('does not pluralize "event" for a single event', () => {
        mockUseMetricReturn = {
            data: [makePiMetric({ throttled: true })],
            loading: false,
            error: null,
        }
        render(<PiPanels agentId="agent-1" rangeSel={oneHour} />)
        expect(screen.getByText(/1 throttle event$/)).toBeInTheDocument()
    })
})