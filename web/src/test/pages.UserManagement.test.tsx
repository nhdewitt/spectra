import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { UserManagement } from '../pages/UserManagement'
import type { ManagedUser, User } from '../types'

vi.mock('../api', () => ({
    api: {
        listUsers: vi.fn(),
        createUser: vi.fn(),
        deleteUser: vi.fn(),
        updateUserRole: vi.fn(),
    },
}))

import { api } from '../api'
const mockListUsers = api.listUsers as ReturnType<typeof vi.fn>
const mockCreateUser = api.createUser as ReturnType<typeof vi.fn>
const mockDeleteUser = api.deleteUser as ReturnType<typeof vi.fn>
const mockUpdateUserRole = api.updateUserRole as ReturnType<typeof vi.fn>

function makeManagedUser(overrides: Partial<ManagedUser> = {}): ManagedUser {
    return {
        id: 'u1',
        username: 'test-viewer',
        role: 'viewer',
        created_at: '2026-01-01T00:00:00.000Z',
        updated_at: '',
        ...overrides,
    }
}

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'test-admin', role: 'admin', ...overrides } as User
}

beforeEach(() => {
    mockListUsers.mockReset().mockResolvedValue([])
    mockCreateUser.mockReset()
    mockDeleteUser.mockReset()
    mockUpdateUserRole.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('UserManagement - listing and role gating', () => {
    it('shows a loading spinner, then renders users', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ username: 'test-viewer' })])

        const { container } = render(<UserManagement user={makeUser()} />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('test-viewer')).toBeInTheDocument())
    })

    it('shows an error message on load failure', async () => {
        mockListUsers.mockRejectedValue(new Error('connection refused'))
        render(<UserManagement user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('marks the current user with "(you)"', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u1', username: 'test-admin' })])
        render(<UserManagement user={makeUser({ id: 'u1' })} />)
        await waitFor(() => expect(screen.getByText('(you)')).toBeInTheDocument())
    })

    it('shows Never for a user with no last_login', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ last_login: undefined })])
        render(<UserManagement user={makeUser()} />)
        await waitFor(() => expect(screen.getByText('Never')).toBeInTheDocument())
    })

    it('hides Create User and the Actions column for a viewer', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser()])
        render(<UserManagement user={makeUser({ role: 'viewer' })} />)
        await waitFor(() => expect(screen.getByText('test-viewer')).toBeInTheDocument())

        expect(screen.queryByText('+ Create User')).not.toBeInTheDocument()
        expect(screen.queryByText('Actions')).not.toBeInTheDocument()
        expect(screen.queryByText('Delete')).not.toBeInTheDocument()
    })

    it('shows Create User and Delete for an admin, but never Delete on their own row', async () => {
        mockListUsers.mockResolvedValue([
            makeManagedUser({ id: 'u1', username: 'self' }),
            makeManagedUser({ id: 'u2', username: 'other' }),
        ])
        render(<UserManagement user={makeUser({ id: 'u1', role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('other')).toBeInTheDocument())

        expect(screen.getByText('+ Create User')).toBeInTheDocument()
        expect(screen.getAllByText('Delete')).toHaveLength(1) // only for "other", not "self"
    })
})

describe('UserManagement - role editing (superadmin only)', () => {
    it('does not let a plain admin click a role badge to edit it', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u2', username: 'other', role: 'viewer' })])
        render(<UserManagement user={makeUser({ id: 'u1', role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('viewer')).toBeInTheDocument())

        fireEvent.click(screen.getByText('viewer'))
        expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    })

    it('lets a superadmin click another user\'s role to change it', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u2', username: 'other', role: 'viewer' })])
        mockUpdateUserRole.mockResolvedValue(undefined)

        render(<UserManagement user={makeUser({ id: 'u1', role: 'superadmin' })} />)
        await waitFor(() => expect(screen.getByText('viewer')).toBeInTheDocument())

        fireEvent.click(screen.getByText('viewer'))
        fireEvent.change(screen.getByRole('combobox'), { target: { value: 'admin' } })

        await waitFor(() => expect(mockUpdateUserRole).toHaveBeenCalledWith('u2', 'admin'))
    })

    it('does not let a superadmin change their own role inline', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u1', username: 'self', role: 'superadmin' })])
        render(<UserManagement user={makeUser({ id: 'u1', role: 'superadmin' })} />)
        await waitFor(() => expect(screen.getByText('superadmin')).toBeInTheDocument())

        fireEvent.click(screen.getByText('superadmin'))
        expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    })
})

describe('UserManagement - delete flow', () => {
    it('requires a confirm click before deleting', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u2', username: 'other' })])
        render(<UserManagement user={makeUser({ id: 'u1', role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete'))
        expect(screen.getByText('Delete other?')).toBeInTheDocument()
        expect(mockDeleteUser).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Confirm'))
        await waitFor(() => expect(mockDeleteUser).toHaveBeenCalledWith('u2'))
    })

    it('cancels the delete confirmation without deleting', async () => {
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u2', username: 'other' })])
        render(<UserManagement user={makeUser({ id: 'u1', role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete'))
        fireEvent.click(screen.getByText('Cancel'))

        expect(screen.queryByText('Delete other?')).not.toBeInTheDocument()
        expect(mockDeleteUser).not.toHaveBeenCalled()
    })

    it('shows a delete error that clears itself after 3 seconds', async () => {
        vi.useFakeTimers()
        mockListUsers.mockResolvedValue([makeManagedUser({ id: 'u2', username: 'other' })])
        mockDeleteUser.mockRejectedValue(new Error('cannot delete last admin'))

        render(<UserManagement user={makeUser({ id: 'u1', role: 'admin' })} />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Delete'))
        fireEvent.click(screen.getByText('Confirm'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        expect(screen.getByText('cannot delete last admin')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
        expect(screen.queryByText('cannot delete last admin')).not.toBeInTheDocument()
    })
})

describe('UserManagement - create modal', () => {
    it('offers only "viewer" as a role for a plain admin', async () => {
        mockListUsers.mockResolvedValue([])
        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Create User'))
        const roleSelect = screen.getByRole('combobox') as HTMLSelectElement
        const options = Array.from(roleSelect.options).map((o) => o.value)
        expect(options).toEqual(['viewer'])
    })

    it('offers all three roles for a superadmin', async () => {
        mockListUsers.mockResolvedValue([])
        render(<UserManagement user={makeUser({ role: 'superadmin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Create User'))
        const roleSelect = screen.getByRole('combobox') as HTMLSelectElement
        const options = Array.from(roleSelect.options).map((o) => o.value)
        expect(options).toEqual(['viewer', 'admin', 'superadmin'])
    })

    it('requires a username', async () => {
        mockListUsers.mockResolvedValue([])
        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create User'))

        fireEvent.click(screen.getByText('Create User', { selector: 'button' }))
        expect(screen.getByText('Username is required.')).toBeInTheDocument()
        expect(mockCreateUser).not.toHaveBeenCalled()
    })

    it('requires a password of at least 8 characters', async () => {
        mockListUsers.mockResolvedValue([])
        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create User'))

        const inputs = screen.getAllByRole('textbox')
        fireEvent.change(inputs[0]!, { target: { value: 'newuser' } })
        const passwordInput = document.querySelector('input[type="password"]')!
        fireEvent.change(passwordInput, { target: { value: 'short' } })
        fireEvent.click(screen.getByText('Create User', { selector: 'button' }))

        expect(screen.getByText('Password must be at least 8 characters.')).toBeInTheDocument()
        expect(mockCreateUser).not.toHaveBeenCalled()
    })

    it('creates a user with trimmed username and refreshes the list', async () => {
        mockListUsers.mockResolvedValueOnce([]).mockResolvedValueOnce([makeManagedUser({ username: 'newuser' })])
        mockCreateUser.mockResolvedValue(undefined)

        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create User'))

        const inputs = screen.getAllByRole('textbox')
        fireEvent.change(inputs[0]!, { target: { value: '  newuser  ' } })
        const passwordInput = document.querySelector('input[type="password"]')!
        fireEvent.change(passwordInput, { target: { value: 'longenoughpassword' } })
        fireEvent.click(screen.getByText('Create User', { selector: 'button' }))

        await waitFor(() =>
            expect(mockCreateUser).toHaveBeenCalledWith('newuser', 'longenoughpassword', 'viewer')
        )
        await waitFor(() => expect(screen.getByText('newuser')).toBeInTheDocument())
    })

    it('shows an error message on create failure', async () => {
        mockListUsers.mockResolvedValue([])
        mockCreateUser.mockRejectedValue(new Error('username taken'))

        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create User'))

        const inputs = screen.getAllByRole('textbox')
        fireEvent.change(inputs[0]!, { target: { value: 'newuser' } })
        const passwordInput = document.querySelector('input[type="password"]')!
        fireEvent.change(passwordInput, { target: { value: 'longenoughpassword' } })
        fireEvent.click(screen.getByText('Create User', { selector: 'button' }))

        await waitFor(() => expect(screen.getByText('username taken')).toBeInTheDocument())
    })

    it('closes the modal via Cancel', async () => {
        mockListUsers.mockResolvedValue([])
        render(<UserManagement user={makeUser({ role: 'admin' })} />)
        await waitFor(() => expect(screen.getByText('+ Create User')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Create User'))
        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.queryByText('Create User', { selector: 'div' })).not.toBeInTheDocument()
    })
})