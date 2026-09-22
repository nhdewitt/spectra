import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useSession } from '../hooks/useSession'
import type { User } from '../types'

vi.mock('../api', () => ({
    api: {
        me: vi.fn(),
        logout: vi.fn(),
    },
}))

import { api } from '../api'

const mockMe = api.me as ReturnType<typeof vi.fn>
const mockLogout = api.logout as ReturnType<typeof vi.fn>

const testUser: User = { id: 'u1', username: 'nathan', role: 'admin' } as User

beforeEach(() => {
    vi.clearAllMocks()
    window.__spectraLogout = undefined
    mockMe.mockRejectedValue(new Error('unauthenticated'))
    mockLogout.mockResolvedValue(null)
})

describe('useSession', () => {
    it('settles checking after the mount-time session probe', async () => {
        const { result } = renderHook(() => useSession())

        expect(result.current.checking).toBe(true)
        await waitFor(() => expect(result.current.checking).toBe(false))
        expect(result.current.user).toBeNull()
    })

    it('adopts an existing session', async () => {
        mockMe.mockResolvedValue(testUser)

        const { result } = renderHook(() => useSession())

        await waitFor(() => expect(result.current.user).toEqual(testUser))
    })

    it('signIn clears a previous logout reason', async () => {
        const { result } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.checking).toBe(false))

        act(() => {
            window.__spectraLogout!()
        })
        expect(result.current.logoutReason).toBe('Your session has expired.')

        act(() => {
            result.current.signIn(testUser)
        })
        expect(result.current.logoutReason).toBeNull()
        expect(result.current.user).toEqual(testUser)
    })

    it('explicit logout calls the endpoint and reports no reason', async () => {
        mockMe.mockResolvedValue(testUser)
        const { result } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.user).toEqual(testUser))

        await act(async () => {
            await result.current.logout()
        })

        expect(mockLogout).toHaveBeenCalled()
        expect(result.current.user).toBeNull()
        expect(result.current.logoutReason).toBeNull()
    })

    it('expiry sets the reason and does not call the endpoint', async () => {
        mockMe.mockResolvedValue(testUser)
        const { result } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.user).toEqual(testUser))

        act(() => {
            window.__spectraLogout!()
        })

        expect(mockLogout).not.toHaveBeenCalled()
        expect(result.current.user).toBeNull()
        expect(result.current.logoutReason).toBe('Your session has expired.')
    })

    // The guard this was extracted for: api.logout() 401ing re-enters the
    // expiry handler, which must not report an expiry the user never had.
    it('a 401 during explicit logout does not report an expiry', async () => {
        mockMe.mockResolvedValue(testUser)
        mockLogout.mockImplementation(async () => {
            window.__spectraLogout?.()
            throw new Error('401')
        })

        const { result } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.user).toEqual(testUser))

        await act(async () => {
            await result.current.logout()
        })

        expect(result.current.user).toBeNull()
        expect(result.current.logoutReason).toBeNull()
    })

    // api.ts no longer clears the handler, so repeated expiries must all land.
    it('keeps handling expiries after the first one', async () => {
        mockMe.mockResolvedValue(testUser)
        const { result } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.user).toEqual(testUser))

        act(() => {
            window.__spectraLogout!()
        })
        act(() => {
            result.current.signIn(testUser)
        })
        expect(result.current.logoutReason).toBeNull()

        act(() => {
            window.__spectraLogout!()
        })
        expect(result.current.logoutReason).toBe('Your session has expired.')
    })

    it('runs onSignOut when the session is cleared', async () => {
        mockMe.mockResolvedValue(testUser)
        const onSignOut = vi.fn()

        const { result } = renderHook(() => useSession(onSignOut))
        await waitFor(() => expect(result.current.user).toEqual(testUser))

        act(() => {
            window.__spectraLogout!()
        })

        expect(onSignOut).toHaveBeenCalledTimes(1)
    })

    // onSignOut is held in a ref so a caller passing an inline closure does
    // not churn the returned callbacks on every render.
    it('keeps logout stable across renders', async () => {
        const { result, rerender } = renderHook(() => useSession(() => {}))
        await waitFor(() => expect(result.current.checking).toBe(false))

        const first = result.current.logout
        rerender()
        expect(result.current.logout).toBe(first)
    })

    it('detaches the expiry handler on unmount', async () => {
        const { result, unmount } = renderHook(() => useSession())
        await waitFor(() => expect(result.current.checking).toBe(false))

        expect(typeof window.__spectraLogout).toBe('function')
        unmount()
        expect(window.__spectraLogout).toBeUndefined()
    })
})