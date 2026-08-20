import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Settings } from '../pages/Settings'
import type { User } from '../types'

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'test-admin', role: 'admin', ...overrides } as User
}

describe('Settings', () => {
    it('shows the account username and role', () => {
        render(<Settings user={makeUser({ username: 'test-admin', role: 'admin' })} onLogout={vi.fn()} />)
        expect(screen.getByText('test-admin')).toBeInTheDocument()
        expect(screen.getByText('admin')).toBeInTheDocument()
    })

    it('calls onLogout when Logout is clicked', () => {
        const onLogout = vi.fn()
        render(<Settings user={makeUser()} onLogout={onLogout} />)

        fireEvent.click(screen.getByText('Logout'))
        expect(onLogout).toHaveBeenCalledTimes(1)
    })

    it('shows a button for every available theme', () => {
        render(<Settings user={makeUser()} onLogout={vi.fn()} />)
        expect(screen.getByText('MIDNIGHT')).toBeInTheDocument()
        expect(screen.getByText('SLATE')).toBeInTheDocument()
        expect(screen.getByText('NORD')).toBeInTheDocument()
    })

    it('does not crash when a theme is selected', () => {
        render(<Settings user={makeUser()} onLogout={vi.fn()} />)
        fireEvent.click(screen.getByText('NORD'))
        expect(screen.getByText('NORD')).toBeInTheDocument()
    })
})