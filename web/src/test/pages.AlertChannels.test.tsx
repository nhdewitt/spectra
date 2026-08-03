import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { AlertChannels } from '../pages/AlertChannels'
import type { AlertChannel } from '../types'

vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: {
            listAlertChannels: vi.fn(),
            createAlertChannel: vi.fn(),
            updateAlertChannel: vi.fn(),
            deleteAlertChannel: vi.fn(),
        },
    }
})

import { api, HttpError } from '../api'
const mockList = api.listAlertChannels as ReturnType<typeof vi.fn>
const mockCreate = api.createAlertChannel as ReturnType<typeof vi.fn>
const mockUpdate = api.updateAlertChannel as ReturnType<typeof vi.fn>
const mockDelete = api.deleteAlertChannel as ReturnType<typeof vi.fn>

function makeChannel(overrides: Partial<AlertChannel> = {}): AlertChannel {
    return {
        id: 'ch-1',
        name: 'ops-webhook',
        type: 'webhook',
        config: { url: 'https://example.com/hook' },
        created_at: '',
        ...overrides,
    }
}

beforeEach(() => {
    mockList.mockReset().mockResolvedValue([])
    mockCreate.mockReset()
    mockUpdate.mockReset()
    mockDelete.mockReset()
})

afterEach(() => {
    vi.useRealTimers()
})

describe('AlertChannels - listing', () => {
    it('shows a loading spinner, then renders channels', async () => {
        mockList.mockResolvedValue([makeChannel({ name: 'ops-webhook' })])

        const { container } = render(<AlertChannels />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('ops-webhook')).toBeInTheDocument())
        expect(screen.getByText('webhook')).toBeInTheDocument()
        expect(screen.getByText('https://example.com/hook')).toBeInTheDocument()
    })

    it('shows an error message on load failure', async () => {
        mockList.mockRejectedValue(new Error('connection refused'))
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows the empty state with no channels', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() =>
            expect(screen.getByText('No channels configured. Create one to start receiving alert notifications.')).toBeInTheDocument()
        )
    })

    it('shows a singular channel count for one channel', async () => {
        mockList.mockResolvedValue([makeChannel()])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('1 channel')).toBeInTheDocument())
    })

    it('shows a plural channel count for more than one channel', async () => {
        mockList.mockResolvedValue([makeChannel({ id: 'ch-1' }), makeChannel({ id: 'ch-2' })])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('2 channels')).toBeInTheDocument())
    })

    it('shows the email recipient as the target for an email channel', async () => {
        mockList.mockResolvedValue([makeChannel({ type: 'email', config: { to: 'alerts@example.com' } })])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('alerts@example.com')).toBeInTheDocument())
    })
})

describe('AlertChannels - create/edit modal', () => {
    it('opens a blank Create Channel modal', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Create Channel'))
        expect(screen.getByText('Create Channel', { selector: 'div' })).toBeInTheDocument()
        expect(screen.getByPlaceholderText('e.g. ops-webhook')).toHaveValue('')
    })

    it('opens Edit pre-filled with the existing channel', async () => {
        mockList.mockResolvedValue([makeChannel({ name: 'ops-webhook', config: { url: 'https://example.com/hook' } })])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('Edit')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Edit'))
        expect(screen.getByText('Edit Channel')).toBeInTheDocument()
        expect(screen.getByDisplayValue('ops-webhook')).toBeInTheDocument()
        expect(screen.getByDisplayValue('https://example.com/hook')).toBeInTheDocument()
    })

    it('requires a name before saving', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Channel'))

        fireEvent.click(screen.getByText('Create Channel', { selector: 'button' }))
        expect(screen.getByText('Name is required.')).toBeInTheDocument()
        expect(mockCreate).not.toHaveBeenCalled()
    })

    it('requires a webhook URL for a webhook channel', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Channel'))

        fireEvent.change(screen.getByPlaceholderText('e.g. ops-webhook'), { target: { value: 'my channel' } })
        fireEvent.click(screen.getByText('Create Channel', { selector: 'button' }))

        expect(screen.getByText('Webhook URL is required.')).toBeInTheDocument()
        expect(mockCreate).not.toHaveBeenCalled()
    })

    it('requires a recipient for an email channel', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Channel'))

        fireEvent.change(screen.getByPlaceholderText('e.g. ops-webhook'), { target: { value: 'my channel' } })
        fireEvent.change(screen.getByDisplayValue('Webhook'), { target: { value: 'email' } })
        fireEvent.click(screen.getByText('Create Channel', { selector: 'button' }))

        expect(screen.getByText('Email recipient is required.')).toBeInTheDocument()
        expect(mockCreate).not.toHaveBeenCalled()
    })

    it('creates a webhook channel with trimmed values and refreshes the list', async () => {
        mockList.mockResolvedValueOnce([]).mockResolvedValueOnce([makeChannel({ name: 'new-hook' })])
        mockCreate.mockResolvedValue(makeChannel({ name: 'new-hook' }))

        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Channel'))

        fireEvent.change(screen.getByPlaceholderText('e.g. ops-webhook'), { target: { value: '  new-hook  ' } })
        fireEvent.change(screen.getByPlaceholderText('https://example.com/hook'), { target: { value: '  https://x.test/hook  ' } })
        fireEvent.click(screen.getByText('Create Channel', { selector: 'button' }))

        await waitFor(() => expect(mockCreate).toHaveBeenCalledWith('new-hook', 'webhook', { url: 'https://x.test/hook' }))
        await waitFor(() => expect(screen.queryByText('Create Channel', { selector: 'div' })).not.toBeInTheDocument())
    })

    it('updates an existing channel', async () => {
        mockList.mockResolvedValue([makeChannel({ id: 'ch-1', name: 'ops-webhook' })])
        mockUpdate.mockResolvedValue(makeChannel({ id: 'ch-1', name: 'renamed' }))

        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('Edit')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Edit'))

        fireEvent.change(screen.getByDisplayValue('ops-webhook'), { target: { value: 'renamed' } })
        fireEvent.click(screen.getByText('Save Changes'))

        await waitFor(() =>
            expect(mockUpdate).toHaveBeenCalledWith('ch-1', 'renamed', 'webhook', { url: 'https://example.com/hook' })
        )
    })

    it('shows the server error message on save failure', async () => {
        mockList.mockResolvedValue([])
        mockCreate.mockRejectedValue(new HttpError(400, 'invalid url'))

        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())
        fireEvent.click(screen.getByText('+ Create Channel'))

        fireEvent.change(screen.getByPlaceholderText('e.g. ops-webhook'), { target: { value: 'ch' } })
        fireEvent.change(screen.getByPlaceholderText('https://example.com/hook'), { target: { value: 'not-a-url' } })
        fireEvent.click(screen.getByText('Create Channel', { selector: 'button' }))

        await waitFor(() => expect(screen.getByText('invalid url')).toBeInTheDocument())
    })

    it('closes the modal via Cancel and via clicking the overlay', async () => {
        mockList.mockResolvedValue([])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('+ Create Channel')).toBeInTheDocument())

        fireEvent.click(screen.getByText('+ Create Channel'))
        fireEvent.click(screen.getByText('Cancel'))
        expect(screen.queryByText('Create Channel', { selector: 'div' })).not.toBeInTheDocument()
    })
})

describe('AlertChannels - delete flow', () => {
    it('requires a confirm click before deleting', async () => {
        mockList.mockResolvedValue([makeChannel({ id: 'ch-1', name: 'ops-webhook' })])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete'))
        expect(screen.getByText('Delete ops-webhook?')).toBeInTheDocument()
        expect(mockDelete).not.toHaveBeenCalled()

        fireEvent.click(screen.getByText('Confirm'))
        await waitFor(() => expect(mockDelete).toHaveBeenCalledWith('ch-1'))
    })

    it('cancels the delete confirmation without deleting', async () => {
        mockList.mockResolvedValue([makeChannel({ id: 'ch-1', name: 'ops-webhook' })])
        render(<AlertChannels />)
        await waitFor(() => expect(screen.getByText('Delete')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Delete'))
        fireEvent.click(screen.getByText('Cancel'))

        expect(screen.queryByText('Delete ops-webhook?')).not.toBeInTheDocument()
        expect(mockDelete).not.toHaveBeenCalled()
    })

    it('shows an error that clears itself after a failed delete', async () => {
        vi.useFakeTimers()
        mockList.mockResolvedValue([makeChannel({ id: 'ch-1', name: 'ops-webhook' })])
        mockDelete.mockRejectedValue(new HttpError(409, 'channel in use'))

        render(<AlertChannels />)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        fireEvent.click(screen.getByText('Delete'))
        fireEvent.click(screen.getByText('Confirm'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })

        expect(screen.getByText('channel in use')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
        expect(screen.queryByText('channel in use')).not.toBeInTheDocument()
    })
})