import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LabelChip } from '../components/LabelChip'
import type { AgentLabel } from '../types'

function makeLabel(overrides: Partial<AgentLabel> = {}): AgentLabel {
    return { key: 'environment', value: 'prod', source: 'user', updated_at: '', ...overrides }
}

describe('LabelChip', () => {
    it('renders the key and value', () => {
        render(<LabelChip label={makeLabel({ key: 'team', value: 'sre' })} />)
        expect(screen.getByText('team')).toBeInTheDocument()
        expect(screen.getByText('=')).toBeInTheDocument()
        expect(screen.getByText('sre')).toBeInTheDocument()
    })

    it('shows the source in the title attribute', () => {
        const { container } = render(<LabelChip label={makeLabel({ source: 'auto' })} />)
        expect(container.querySelector('span[title="auto label"]')).toBeInTheDocument()
    })

    it('does not render a delete button when onDelete is not provided', () => {
        render(<LabelChip label={makeLabel()} />)
        expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })

    it('renders a delete button titled "Remove {key}" and calls onDelete when clicked', () => {
        const onDelete = vi.fn()
        render(<LabelChip label={makeLabel({ key: 'team' })} onDelete={onDelete} />)

        const btn = screen.getByTitle('Remove team')
        fireEvent.click(btn)
        expect(onDelete).toHaveBeenCalledTimes(1)
    })

    it('dims auto-sourced labels relative to user labels', () => {
        const { container: autoContainer } = render(<LabelChip label={makeLabel({ source: 'auto' })} />)
        const { container: userContainer } = render(<LabelChip label={makeLabel({ source: 'user' })} />)

        const autoSpan = autoContainer.querySelector('span') as HTMLElement
        const userSpan = userContainer.querySelector('span') as HTMLElement

        expect(autoSpan.style.opacity).toBe('0.85')
        expect(userSpan.style.opacity).toBe('1')
    })

    it('renders a deterministic color for the same key/value pair', () => {
        const { container: c1 } = render(<LabelChip label={makeLabel({ key: 'team', value: 'sre' })} />)
        const { container: c2 } = render(<LabelChip label={makeLabel({ key: 'team', value: 'sre' })} />)

        const span1 = c1.querySelector('span') as HTMLElement
        const span2 = c2.querySelector('span') as HTMLElement

        expect(span1.style.borderColor).toBe(span2.style.borderColor)
        expect(span1.style.backgroundColor).toBe(span2.style.backgroundColor)
    })
})