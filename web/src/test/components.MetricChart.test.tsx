import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MetricChart } from '../components/MetricChart'

interface Point {
    time: string
    _ts: number
    value: number
}

function makePoint(overrides: Partial<Point> = {}): Point {
    const time = overrides.time ?? '2026-01-01T00:00:00.000Z'
    return { time, _ts: Date.parse(time), value: 1, ...overrides }
}

const series = [{ key: 'value', label: 'Value' }]

describe('MetricChart', () => {
    it('renders the title', () => {
        render(<MetricChart title="CPU" data={[]} series={series} />)
        expect(screen.getByText('CPU')).toBeInTheDocument()
    })

    it('shows a loading spinner while loading, and nothing else', () => {
        const { container } = render(
            <MetricChart title="CPU" data={[makePoint()]} series={series} loading error="ignored while loading" />
        )
        expect(container.querySelector('svg')).toBeInTheDocument() // LoadingSpinner
        expect(screen.queryByText('ignored while loading')).not.toBeInTheDocument()
        expect(container.querySelector('.recharts-responsive-container')).not.toBeInTheDocument()
    })

    it('shows the error message and no chart when not loading', () => {
        const { container } = render(
            <MetricChart title="CPU" data={[makePoint()]} series={series} error="upstream failed" />
        )
        expect(screen.getByText('upstream failed')).toBeInTheDocument()
        expect(container.querySelector('.recharts-responsive-container')).not.toBeInTheDocument()
    })

    it('prioritizes the error message over the empty-data message', () => {
        render(<MetricChart title="CPU" data={[]} series={series} error="upstream failed" />)
        expect(screen.getByText('upstream failed')).toBeInTheDocument()
        expect(screen.queryByText('No data for this range.')).not.toBeInTheDocument()
    })

    it('shows "No data for this range." when not loading, no error, and data is empty', () => {
        const { container } = render(<MetricChart title="CPU" data={[]} series={series} />)
        expect(screen.getByText('No data for this range.')).toBeInTheDocument()
        expect(container.querySelector('.recharts-responsive-container')).not.toBeInTheDocument()
    })

    it('renders the chart container once data is present and there is no loading/error', () => {
        const { container } = render(
            <MetricChart title="CPU" data={[makePoint(), makePoint({ time: '2026-01-01T00:01:00.000Z' })]} series={series} />
        )
        expect(container.querySelector('.recharts-responsive-container')).toBeInTheDocument()
        expect(screen.queryByText('No data for this range.')).not.toBeInTheDocument()
    })

    it('respects a custom height', () => {
        const { container } = render(
            <MetricChart title="CPU" data={[makePoint()]} series={series} height={160} />
        )
        const wrapper = container.querySelector('.recharts-responsive-container') as HTMLElement
        expect(wrapper.style.height).toBe('160px')
    })
})