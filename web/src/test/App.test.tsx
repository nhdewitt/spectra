import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import App from '../App'

// Every api call App makes on mount. me() rejecting keeps user null, so only
// Login renders -- the expiry handler is installed regardless of auth state,
// which is what lets this test stay off the dashboard tree.
// importActual keeps HttpError, which Login uses in an instanceof check.
vi.mock('../api', async () => {
    const actual = await vi.importActual<typeof import('../api')>('../api')
    return {
        ...actual,
        api: {
            me: vi.fn(),
            version: vi.fn(),
            logout: vi.fn(),
            thresholds: vi.fn(),
            userConfig: vi.fn(),
            setUserConfig: vi.fn(),
            deleteUserConfig: vi.fn(),
            overviewStats: vi.fn(),
            login: vi.fn(),
        },
    }
})

import { api } from '../api'

const mocked = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
    vi.clearAllMocks()
    window.__spectraLogout = undefined
    mocked.me!.mockRejectedValue(new Error('unauthenticated'))
    mocked.version!.mockResolvedValue({ version: 'test' })
    mocked.thresholds!.mockResolvedValue({})
    mocked.userConfig!.mockResolvedValue({})
    mocked.overviewStats!.mockResolvedValue(null)
    mocked.logout!.mockResolvedValue(null)
})

async function renderApp() {
    render(<App />)
    // Past the checking splash.
    await waitFor(() => expect(screen.getByLabelText(/username/i)).toBeInTheDocument())
}

describe('session expiry handling', () => {
    it('installs the expiry handler', async () => {
        await renderApp()
        expect(typeof window.__spectraLogout).toBe('function')
    })

    it('shows the expiry message when the handler fires', async () => {
        await renderApp()
        expect(screen.queryByText(/session has expired/i)).not.toBeInTheDocument()

        act(() => {
            window.__spectraLogout!()
        })

        await waitFor(() =>
            expect(screen.getByText(/session has expired/i)).toBeInTheDocument()
        )
    })

    // The regression this pairs with lives in api.ts, which no longer clears
    // the handler. Firing it twice must not detach it.
    it('leaves the handler installed after it fires', async () => {
        await renderApp()
        const handler = window.__spectraLogout

        act(() => {
            window.__spectraLogout!()
        })
        expect(window.__spectraLogout).toBe(handler)

        act(() => {
            window.__spectraLogout!()
        })
        expect(window.__spectraLogout).toBe(handler)
    })

    // Expiry must not call the logout endpoint: the cookie is already invalid,
    // so the request would 401 and re-enter the handler it came from.
    it('does not hit the logout endpoint on expiry', async () => {
        await renderApp()

        act(() => {
            window.__spectraLogout!()
        })

        await waitFor(() =>
            expect(screen.getByText(/session has expired/i)).toBeInTheDocument()
        )
        expect(mocked.logout).not.toHaveBeenCalled()
    })
})