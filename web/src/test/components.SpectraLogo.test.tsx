import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { SpectraLogo } from '../components/SpectraLogo'

describe('SpectraLogo', () => {
    it('renders an svg sized to the size prop, defaulting to 40', () => {
        const { container } = render(<SpectraLogo />)
        const svg = container.querySelector('svg')!
        expect(svg.getAttribute('width')).toBe('40')
        expect(svg.getAttribute('height')).toBe('40')
    })

    it('renders at a custom size', () => {
        const { container } = render(<SpectraLogo size={24} />)
        const svg = container.querySelector('svg')!
        expect(svg.getAttribute('width')).toBe('24')
        expect(svg.getAttribute('height')).toBe('24')
    })

    it('uses the current theme colors for the two-tone circle', () => {
        const { container } = render(<SpectraLogo />)
        // The first two <circle> elements are clip-path definitions with no
        // fill attribute at all - select the actual filled circle directly.
        const filledCircle = container.querySelector('circle[fill]')!
        expect(filledCircle.getAttribute('fill')).toMatch(/^#[0-9a-f]{6}$/i)
    })

    it('renders no animation style or <style> tag by default', () => {
        const { container } = render(<SpectraLogo />)
        const svg = container.querySelector('svg')!
        expect(svg.getAttribute('style')).toBeNull()
        expect(container.querySelector('style')).not.toBeInTheDocument()
    })

    it('injects a keyframe animation when animate is true', () => {
        const { container } = render(<SpectraLogo animate />)
        const svg = container.querySelector('svg')!
        expect(svg.getAttribute('style')).toMatch(/animation:\s*spectra-spin-/)

        const styleTag = container.querySelector('style')
        expect(styleTag).toBeInTheDocument()
        expect(styleTag!.textContent).toMatch(/@keyframes spectra-spin-/)
    })

    it('generates non-colliding clip-path ids across separate instances', () => {
        const first = render(<SpectraLogo />)
        const second = render(<SpectraLogo />)

        const firstClip = first.container.querySelector('clipPath')!.id
        const secondClip = second.container.querySelector('clipPath')!.id

        expect(firstClip).not.toBe(secondClip)
    })
})