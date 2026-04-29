import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { TimeRangePicker } from '../components/TimeRangePicker'
import type { RangeSelection } from '../types'

function lastCallArg<T>(fn: ReturnType<typeof vi.fn>): T {
    const calls = fn.mock.calls
    if (calls.length === 0) throw new Error('mock was never called')
    return calls[calls.length - 1]![0] as T
}

describe('TimeRangePicker', () => {
    const quickValue: RangeSelection = { type: 'quick', range: '1h' }

    it('renders all quick range buttons', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        for (const range of ['5m', '15m', '1h', '6h', '24h', '7d', '30d']) {
            expect(screen.getByText(range)).toBeInTheDocument()
        }
    })

    it('renders CUSTOM button', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        expect(screen.getByText('CUSTOM')).toBeInTheDocument()
    })

    it('calls onChange with quick range on click', () => {
        const onChange = vi.fn()
        render(<TimeRangePicker value={quickValue} onChange={onChange} />)
        fireEvent.click(screen.getByText('24h'))
        expect(onChange).toHaveBeenCalledWith({ type: 'quick', range: '24h' })
    })

    it('does not show custom inputs initially', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        expect(screen.queryByText('Start')).toBeNull()
        expect(screen.queryByText('End')).toBeNull()
    })

    it('shows custom inputs when CUSTOM is clicked', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        fireEvent.click(screen.getByText('CUSTOM'))
        expect(screen.getByText('Start')).toBeInTheDocument()
        expect(screen.getByText('End')).toBeInTheDocument()
        expect(screen.getByText('APPLY')).toBeInTheDocument()
    })

    it('hides custom inputs when CUSTOM is clicked again', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        fireEvent.click(screen.getByText('CUSTOM'))
        expect(screen.getByText('Start')).toBeInTheDocument()

        fireEvent.click(screen.getByText('CUSTOM'))
        expect(screen.queryByText('Start')).toBeNull()
    })

    it('hides custom inputs when a quick range is selected', () => {
        render(<TimeRangePicker value={quickValue} onChange={() => {}} />)
        fireEvent.click(screen.getByText('CUSTOM'))
        expect(screen.getByText('Start')).toBeInTheDocument()

        fireEvent.click(screen.getByText('6h'))
        expect(screen.queryByText('Start')).toBeNull()
    })

    it('shows validation error when start >= end', () => {
        const customValue: RangeSelection = {
        type: 'custom',
        start: '2026-06-01T14:00:00Z',
        end: '2026-06-01T10:00:00Z',
        }
        render(<TimeRangePicker value={customValue} onChange={() => {}} />)
        expect(screen.getByText('Start date must fall before end date')).toBeInTheDocument()
    })

    it('calls onChange with custom range on APPLY', () => {
        const customValue: RangeSelection = {
        type: 'custom',
        start: '2026-06-01T08:00:00Z',
        end: '2026-06-01T12:00:00Z',
        }
        const onChange = vi.fn()
        render(<TimeRangePicker value={customValue} onChange={onChange} />)

        fireEvent.click(screen.getByText('APPLY'))

        expect(onChange).toHaveBeenCalledTimes(1)
        const call = lastCallArg(onChange) as RangeSelection
        expect(call.type).toBe('custom')
        if (call.type === 'custom') {
        expect(new Date(call.start).getTime()).toBeLessThan(new Date(call.end).getTime())
        }
    })

    it('opens with custom inputs when value is custom type', () => {
        const customValue: RangeSelection = {
            type: 'custom',
            start: '2026-06-01T08:00:00Z',
            end: '2026-06-01T12:00:00Z',
        }
        render(<TimeRangePicker value={customValue} onChange={() => {}} />)
        expect(screen.getByText('Start')).toBeInTheDocument()
        expect(screen.getByText('End')).toBeInTheDocument()
    })
})