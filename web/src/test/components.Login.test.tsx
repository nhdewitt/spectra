import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Login } from '../components/Login'
import type { User } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: { login: vi.fn() },
    }
})

import { api, HttpError } from '../api'
const mockLogin = api.login as ReturnType<typeof vi.fn>

const noop = () => {}

beforeEach(() => {
    mockLogin.mockReset()
})

describe('Login', () => {
    it('disables the submit button until both fields are filled', () => {
        render(<Login onLogin={noop} />)

        expect(screen.getByRole('button', { name: /login/i })).toBeDisabled()

        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        expect(screen.getByRole('button', { name: /login/i })).toBeDisabled()

        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'hunter2' } })
        expect(screen.getByRole('button', { name: /login/i })).not.toBeDisabled()
    })

    it('calls api.login with the entered credentials and onLogin with the result on success', async () => {
        const user: User = { id: 'u1', username: 'admin', role: 'admin' } as User
        mockLogin.mockResolvedValue(user)
        const onLogin = vi.fn()

        render(<Login onLogin={onLogin} />)

        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'hunter2' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() => expect(onLogin).toHaveBeenCalledWith(user))
        expect(mockLogin).toHaveBeenCalledWith('admin', 'hunter2')
    })

    it('shows a loading state while the request is in flight, then clears it', async () => {
        let resolveLogin!: (u: User) => void
        mockLogin.mockReturnValue(new Promise<User>((res) => { resolveLogin = res }))

        render(<Login onLogin={noop} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'hunter2' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        expect(await screen.findByRole('button', { name: '...' })).toBeDisabled()

        resolveLogin({ id: 'u1', username: 'admin', role: 'admin' } as User)
        await waitFor(() => expect(screen.getByRole('button', { name: /login/i })).toBeInTheDocument())
    })

    it('shows the raw error message for a generic Error', async () => {
        mockLogin.mockRejectedValue(new Error('invalid credentials'))

        render(<Login onLogin={noop} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrong' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() => expect(screen.getByText('invalid credentials')).toBeInTheDocument())
    })

    it('shows a specific message for a 404 (route/server misconfiguration)', async () => {
        mockLogin.mockRejectedValue(new HttpError(404, 'not found'))

        render(<Login onLogin={noop} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'x' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() =>
            expect(screen.getByText('Login service is unavailable. Check server URL/route.')).toBeInTheDocument()
        )
    })

    it('shows a specific message for a 429 (rate limited)', async () => {
        mockLogin.mockRejectedValue(new HttpError(429, 'rate limited'))

        render(<Login onLogin={noop} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'x' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() =>
            expect(screen.getByText('Too many login attempts. Please try again later.')).toBeInTheDocument()
        )
    })

    it('falls back to the error message for other HTTP statuses (e.g. 401)', async () => {
        mockLogin.mockRejectedValue(new HttpError(401, 'invalid username or password'))

        render(<Login onLogin={noop} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrong' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() => expect(screen.getByText('invalid username or password')).toBeInTheDocument())
    })

    it('clears a previous error as soon as a new submit begins', async () => {
        mockLogin.mockRejectedValueOnce(new Error('invalid credentials'))
        const user: User = { id: 'u1', username: 'admin', role: 'admin' } as User
        mockLogin.mockResolvedValueOnce(user)
        const onLogin = vi.fn()

        render(<Login onLogin={onLogin} />)
        fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } })
        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrong' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() => expect(screen.getByText('invalid credentials')).toBeInTheDocument())

        fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'right' } })
        fireEvent.click(screen.getByRole('button', { name: /login/i }))

        await waitFor(() => expect(onLogin).toHaveBeenCalledWith(user))
        expect(screen.queryByText('invalid credentials')).not.toBeInTheDocument()
    })

    it('shows the passed-in message (e.g. session expired) above the form', () => {
        render(<Login onLogin={noop} message="Your session has expired." />)
        expect(screen.getByText('Your session has expired.')).toBeInTheDocument()
    })

    it('renders no message banner when message is not provided', () => {
        render(<Login onLogin={noop} />)
        expect(screen.queryByText(/session/i)).not.toBeInTheDocument()
    })
})