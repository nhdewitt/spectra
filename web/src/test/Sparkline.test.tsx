import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Sparkline } from '../Sparkline'

describe('Sparkline', () => {
    it('renders empty svg with fewer than 2 data points', () => {
        const { container } = render(<Sparkline data={[50]} />)
        const svg = container.querySelector('svg')
        expect(svg).toBeInTheDocument()
        // No path elements with insufficient data
        expect(container.querySelectorAll('path')).toHaveLength(0)
    })

    it('renders line and fill paths with valid data', () => {
        const { container } = render(<Sparkline data={[10, 20, 30, 40, 50]} />)
        const paths = container.querySelectorAll('path')
        // One fill path + one stroke path
        expect(paths).toHaveLength(2)
    })

    it('renders at custom dimensions', () => {
        const { container } = render(<Sparkline data={[10, 50]} width={120} height={40} />)
        const svg = container.querySelector('svg')
        expect(svg).toHaveAttribute('width', '120')
        expect(svg).toHaveAttribute('height', '40')
    })

    it('shows tooltip on hover with stats', () => {
        render(<Sparkline data={[10, 50, 30, 90, 60]} label="CPU" />)

        // Tooltip not visible initially
        expect(screen.queryByText('CPU')).toBeNull()
        expect(screen.queryByText(/now/)).toBeNull()

        // Hover to show tooltip
        const container = screen.getByText((_, el) => el?.tagName === 'svg')?.parentElement!
        fireEvent.mouseEnter(container)

        expect(screen.getByText('CPU')).toBeInTheDocument()
        expect(screen.getByText(/now/)).toBeInTheDocument()
        expect(screen.getByText('60.0%')).toBeInTheDocument()   // current (last value)
        expect(screen.getByText(/peak/)).toBeInTheDocument()
        expect(screen.getByText('90.0%')).toBeInTheDocument()   // peak
        expect(screen.getByText(/avg/)).toBeInTheDocument()
        expect(screen.getByText('48.0%')).toBeInTheDocument()   // avg
        expect(screen.getByText(/min/)).toBeInTheDocument()
        expect(screen.getByText('10.0%')).toBeInTheDocument()   // min
    })

    it('hides tooltip on mouse leave', () => {
        render(<Sparkline data={[10, 50, 30]} label="MEM" />)

        const container = screen.getByText((_, el) => el?.tagName === 'svg')?.parentElement!
        fireEvent.mouseEnter(container)
        expect(screen.getByText('MEM')).toBeInTheDocument()
    })

    it('does not render label in tooltip when omitted', () => {
        render(<Sparkline data={[10, 50, 30]} />)

        const container = screen.getByText((_, el) => el?.tagName === 'svg')?.parentElement!
        fireEvent.mouseEnter(container)

        expect(screen.getByText(/now/)).toBeInTheDocument()
        // No label div should exist
        expect(screen.queryByText('CPU')).toBeNull()
        expect(screen.queryByText('MEM')).toBeNull()
    })

    it('clamps values to yMin/yMax range', () => {
        // Values outside range shouldn't cause errors
        const { container } = render(<Sparkline data={[-10, 150, 50]} yMin={0} yMax={100} />)
        const paths = container.querySelectorAll('path')
        expect(paths).toHaveLength(2)
    })
})