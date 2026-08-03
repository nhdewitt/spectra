import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Alerts } from '../pages/Alerts'
import type { User } from '../types'

vi.mock('../pages/ActiveAlerts', () => ({
    ActiveAlerts: () => <div data-testid="panel-active" />,
}))
vi.mock('../pages/AlertRules', () => ({
    AlertRules: ({ user }: { user: User }) => <div data-testid="panel-rules" data-username={user.username} />,
}))
vi.mock('../pages/AlertChannels', () => ({
    AlertChannels: () => <div data-testid="panel-channels" />,
}))

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'admin', role: 'admin', ...overrides } as User
}

describe('Alerts', () => {
    it('shows the Active panel by default', () => {
        render(<Alerts user={makeUser()} />)
        expect(screen.getByTestId('panel-active')).toBeInTheDocument()
        expect(screen.queryByTestId('panel-rules')).not.toBeInTheDocument()
        expect(screen.queryByTestId('panel-channels')).not.toBeInTheDocument()
    })

    it('switches to the Rules panel and passes the user through', () => {
        render(<Alerts user={makeUser({ username: 'test-admin' })} />)
        fireEvent.click(screen.getByText('Rules'))

        expect(screen.getByTestId('panel-rules')).toBeInTheDocument()
        expect(screen.getByTestId('panel-rules')).toHaveAttribute('data-username', 'test-admin')
        expect(screen.queryByTestId('panel-active')).not.toBeInTheDocument()
    })

    it('switches to the Channels panel', () => {
        render(<Alerts user={makeUser()} />)
        fireEvent.click(screen.getByText('Channels'))

        expect(screen.getByTestId('panel-channels')).toBeInTheDocument()
        expect(screen.queryByTestId('panel-active')).not.toBeInTheDocument()
    })

    it('switches back to Active from another tab', () => {
        render(<Alerts user={makeUser()} />)
        fireEvent.click(screen.getByText('Channels'))
        fireEvent.click(screen.getByText('Active'))

        expect(screen.getByTestId('panel-active')).toBeInTheDocument()
        expect(screen.queryByTestId('panel-channels')).not.toBeInTheDocument()
    })
})