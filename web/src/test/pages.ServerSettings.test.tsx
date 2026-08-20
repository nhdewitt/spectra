import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ServerSettings } from '../pages/ServerSettings'
import type { User } from '../types'

vi.mock('../pages/SMTPSettings', () => ({
    SMTPSettings: () => <div data-testid="smtp-settings" />,
}))

vi.mock('../pages/ThresholdsSettings', () => ({
    ThresholdsSettings: () => <div data-testid="thresholds-settings" />,
}))

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'test-admin', role: 'superadmin', ...overrides } as User
}

describe('ServerSettings', () => {
    it('opens on the thresholds tab for a superadmin', () => {
        render(<ServerSettings user={makeUser()} />)
        expect(screen.getByTestId('thresholds-settings')).toBeInTheDocument()
        expect(screen.queryByTestId('smtp-settings')).not.toBeInTheDocument()
    })

    it('switches to the email tab and renders SMTP settings', () => {
        render(<ServerSettings user={makeUser()} />)

        fireEvent.click(screen.getByText('email'))

        expect(screen.getByTestId('smtp-settings')).toBeInTheDocument()
        expect(screen.queryByTestId('thresholds-settings')).not.toBeInTheDocument()
    })

    it('refuses an admin', () => {
        render(<ServerSettings user={makeUser({ role: 'admin' })} />)
        expect(screen.getByText('Server settings require the superadmin role.')).toBeInTheDocument()
        expect(screen.queryByTestId('thresholds-settings')).not.toBeInTheDocument()
        expect(screen.queryByTestId('smtp-settings')).not.toBeInTheDocument()
    })

    it('refuses a viewer', () => {
        render(<ServerSettings user={makeUser({ role: 'viewer' })} />)
        expect(screen.getByText('Server settings require the superadmin role.')).toBeInTheDocument()
    })
})