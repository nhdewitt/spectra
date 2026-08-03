import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { SMTPSettings } from '../pages/SMTPSettings'
import type { SMTPConfig } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: { smtpConfig: vi.fn(), updateSMTPConfig: vi.fn(), testSMTPConfig: vi.fn() },
    }
})

import { api, HttpError } from '../api'
const mockSmtpConfig = api.smtpConfig as ReturnType<typeof vi.fn>
const mockUpdateSMTPConfig = api.updateSMTPConfig as ReturnType<typeof vi.fn>
const mockTestSMTPConfig = api.testSMTPConfig as ReturnType<typeof vi.fn>

function makeConfig(overrides: Partial<SMTPConfig> = {}): SMTPConfig {
    return {
        enabled: false,
        host: '',
        port: 587,
        username: '',
        password_set: false,
        from_address: '',
        tls_mode: 'starttls',
        updated_at: '',
        ...overrides,
    }
}

beforeEach(() => {
    mockSmtpConfig.mockReset().mockResolvedValue(makeConfig())
    mockUpdateSMTPConfig.mockReset()
    mockTestSMTPConfig.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('SMTPSettings - loading', () => {
    it('shows a loading message, then the loaded config', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ host: 'smtp.example.com', port: 465, from_address: 'alerts@example.com' }))

        render(<SMTPSettings />)
        expect(screen.getByText('Loading…')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByDisplayValue('smtp.example.com')).toBeInTheDocument())
        expect(screen.getByDisplayValue('465')).toBeInTheDocument()
        expect(screen.getByDisplayValue('alerts@example.com')).toBeInTheDocument()
    })

    it('shows an error message if loading fails', async () => {
        mockSmtpConfig.mockRejectedValue(new Error('unreachable'))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('unreachable')).toBeInTheDocument())
    })

    it('falls back to the default port when the server returns 0', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ port: 0 }))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByDisplayValue('587')).toBeInTheDocument())
    })
})

describe('SMTPSettings - password state machine', () => {
    it('offers to set a password when none is stored', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: false }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('No password stored.')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Set Password'))
        expect(screen.getByPlaceholderText('Enter new password')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.getByText('No password stored.')).toBeInTheDocument()
    })

    it('shows stored/Change/Clear when a password is already set', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: true }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('•••••••• stored')).toBeInTheDocument())

        expect(screen.getByText('Change')).toBeInTheDocument()
        expect(screen.getByText('Clear')).toBeInTheDocument()
    })

    it('marks the password for clearing, then can revert to Keep Current', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: true }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Clear')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Clear'))
        expect(screen.getByText('Password will be cleared on save.')).toBeInTheDocument()

        fireEvent.click(screen.getByText('Keep Current'))
        expect(screen.getByText('•••••••• stored')).toBeInTheDocument()
    })

    it('reveals the password input when changing an existing password', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: true }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Change')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Change'))
        expect(screen.getByPlaceholderText('Enter new password')).toBeInTheDocument()
    })
})

describe('SMTPSettings - TLS mode port suggestion', () => {
    it('suggests the conventional port when switching modes from a known default', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ port: 587, tls_mode: 'starttls' }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByDisplayValue('587')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('STARTTLS'), { target: { value: 'implicit' } })
        expect(screen.getByDisplayValue('465')).toBeInTheDocument()
    })

    it('does not override a custom port the user already typed', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ port: 587, tls_mode: 'starttls' }))
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByDisplayValue('587')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('587'), { target: { value: '2525' } })
        fireEvent.change(screen.getByDisplayValue('STARTTLS'), { target: { value: 'implicit' } })

        expect(screen.getByDisplayValue('2525')).toBeInTheDocument()
    })
})

describe('SMTPSettings - save', () => {
    it('omits the password field when intent is "keep"', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ host: 'smtp.example.com', password_set: true }))
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig({ password_set: true }))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(mockUpdateSMTPConfig).toHaveBeenCalled())
        const payload = mockUpdateSMTPConfig.mock.calls[0]![0]
        expect(payload).not.toHaveProperty('password')
    })

    it('sends an empty password string when intent is "clear"', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: true }))
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig({ password_set: false }))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Clear')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Clear'))
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(mockUpdateSMTPConfig).toHaveBeenCalled())
        expect(mockUpdateSMTPConfig.mock.calls[0]![0]).toMatchObject({ password: '' })
    })

    it('sends the new password when intent is "set" with a value', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: false }))
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig({ password_set: true }))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Set Password')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Set Password'))
        fireEvent.change(screen.getByPlaceholderText('Enter new password'), { target: { value: 'hunter2' } })
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(mockUpdateSMTPConfig).toHaveBeenCalled())
        expect(mockUpdateSMTPConfig.mock.calls[0]![0]).toMatchObject({ password: 'hunter2' })
    })

    it('omits the password when intent is "set" but the field is left empty', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig({ password_set: false }))
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig())

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Set Password')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Set Password'))
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(mockUpdateSMTPConfig).toHaveBeenCalled())
        expect(mockUpdateSMTPConfig.mock.calls[0]![0]).not.toHaveProperty('password')
    })

    it('trims host, username, and from_address before sending', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig())

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('smtp.example.com'), { target: { value: '  smtp.example.com  ' } })
        fireEvent.change(screen.getByPlaceholderText('alerts@example.com'), { target: { value: '  alerts@example.com  ' } })
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(mockUpdateSMTPConfig).toHaveBeenCalled())
        expect(mockUpdateSMTPConfig.mock.calls[0]![0]).toMatchObject({
            host: 'smtp.example.com',
            from_address: 'alerts@example.com',
        })
    })

    it('shows a notice on success that clears itself after 3 seconds', async () => {
        vi.useFakeTimers()
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockUpdateSMTPConfig.mockResolvedValue(makeConfig())

        render(<SMTPSettings />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Save'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        expect(screen.getByText('SMTP configuration saved.')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
        expect(screen.queryByText('SMTP configuration saved.')).not.toBeInTheDocument()
    })

    it('shows the server error message for an HttpError on save failure', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockUpdateSMTPConfig.mockRejectedValue(new HttpError(400, 'invalid host'))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(screen.getByText('invalid host')).toBeInTheDocument())
    })

    it('shows a generic message for a non-HttpError save failure', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockUpdateSMTPConfig.mockRejectedValue(new Error('boom'))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Save'))

        await waitFor(() => expect(screen.getByText('Failed to save SMTP config.')).toBeInTheDocument())
    })

    it('shows Saving... and disables the button while the save is in flight', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        let resolveSave!: (c: SMTPConfig) => void
        mockUpdateSMTPConfig.mockReturnValue(new Promise<SMTPConfig>((res) => { resolveSave = res }))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Save'))

        expect(screen.getByText('Saving...')).toBeDisabled()
        resolveSave(makeConfig())
        await waitFor(() => expect(screen.getByText('Save')).toBeInTheDocument())
    })
})

describe('SMTPSettings - test send', () => {
    it('requires a recipient before sending, without calling the API', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Send Test')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Send Test'))

        expect(screen.getByText('Enter a recipient address to send a test message.')).toBeInTheDocument()
        expect(mockTestSMTPConfig).not.toHaveBeenCalled()
    })

    it('sends a test message and shows a confirmation notice', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockTestSMTPConfig.mockResolvedValue({ status: 'sent' })

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Send Test')).toBeInTheDocument())

        fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'me@example.com' } })
        fireEvent.click(screen.getByText('Send Test'))

        await waitFor(() => expect(screen.getByText('Test message sent to me@example.com.')).toBeInTheDocument())
        expect(mockTestSMTPConfig).toHaveBeenCalledWith(expect.objectContaining({ test_to: 'me@example.com' }))
    })

    it('shows the server error message for an HttpError on test failure', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockTestSMTPConfig.mockRejectedValue(new HttpError(500, 'smtp handshake failed'))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Send Test')).toBeInTheDocument())
        fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'me@example.com' } })
        fireEvent.click(screen.getByText('Send Test'))

        await waitFor(() => expect(screen.getByText('smtp handshake failed')).toBeInTheDocument())
    })

    it('shows a generic message for a non-HttpError test failure', async () => {
        mockSmtpConfig.mockResolvedValue(makeConfig())
        mockTestSMTPConfig.mockRejectedValue(new Error('boom'))

        render(<SMTPSettings />)
        await waitFor(() => expect(screen.getByText('Send Test')).toBeInTheDocument())
        fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'me@example.com' } })
        fireEvent.click(screen.getByText('Send Test'))

        await waitFor(() => expect(screen.getByText('Test send failed.')).toBeInTheDocument())
    })
})