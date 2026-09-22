import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useStarredAgents } from '../hooks/useStarredAgents'
import type { User } from '../types'

vi.mock('../api', () => ({
    api: {
        userConfig: vi.fn(),
        setUserConfig: vi.fn(),
        deleteUserConfig: vi.fn(),
    },
}))

import { api } from '../api'

const mockUserConfig = api.userConfig as ReturnType<typeof vi.fn>
const mockSetUserConfig = api.setUserConfig as ReturnType<typeof vi.fn>
const mockDeleteUserConfig = api.deleteUserConfig as ReturnType<typeof vi.fn>

const userA = { id: 'a', username: 'alice' } as User
const userB = { id: 'b', username: 'bob' } as User

beforeEach(() => {
    vi.clearAllMocks()
    mockUserConfig.mockResolvedValue({})
    mockSetUserConfig.mockResolvedValue(null)
    mockDeleteUserConfig.mockResolvedValue(null)
})

afterEach(() => {
    vi.useRealTimers()
})

describe('useStarredAgents', () => {
    it('loads the account\'s stars', async () => {
        mockUserConfig.mockResolvedValue({ starred_agents: ['agent-1', 'agent-2'] })

        const { result } = renderHook(() => useStarredAgents(userA))

        await waitFor(() => expect(result.current.starredIds).toEqual(['agent-1', 'agent-2']))
    })

    it('fetches nothing when signed out', async () => {
        renderHook(() => useStarredAgents(null))
        expect(mockUserConfig).not.toHaveBeenCalled()
    })

    it('toggleStar adds and removes', async () => {
        const { result } = renderHook(() => useStarredAgents(userA))
        await waitFor(() => expect(mockUserConfig).toHaveBeenCalled())

        act(() => result.current.toggleStar('agent-1'))
        expect(result.current.starredIds).toEqual(['agent-1'])

        act(() => result.current.toggleStar('agent-1'))
        expect(result.current.starredIds).toEqual([])
    })

    // The initial load must not look like an edit, or signing in would
    // immediately write back what it just read.
    it('does not persist the initial load', async () => {
        vi.useFakeTimers()
        mockUserConfig.mockResolvedValue({ starred_agents: ['agent-1'] })

        const { result } = renderHook(() => useStarredAgents(userA))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })
        expect(result.current.starredIds).toEqual(['agent-1'])

        await act(async () => {
            await vi.advanceTimersByTimeAsync(2000)
        })
        expect(mockSetUserConfig).not.toHaveBeenCalled()
        expect(mockDeleteUserConfig).not.toHaveBeenCalled()
    })

    it('persists an edit after the debounce', async () => {
        vi.useFakeTimers()

        const { result } = renderHook(() => useStarredAgents(userA))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        act(() => result.current.toggleStar('agent-1'))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(600)
        })

        expect(mockSetUserConfig).toHaveBeenCalledWith('starred_agents', ['agent-1'])
    })

    it('deletes the key when the last star is removed', async () => {
        vi.useFakeTimers()
        mockUserConfig.mockResolvedValue({ starred_agents: ['agent-1'] })

        const { result } = renderHook(() => useStarredAgents(userA))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        act(() => result.current.toggleStar('agent-1'))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(600)
        })

        expect(mockDeleteUserConfig).toHaveBeenCalledWith('starred_agents')
        expect(mockSetUserConfig).not.toHaveBeenCalled()
    })

    it('collapses a burst of toggles into one request', async () => {
        vi.useFakeTimers()

        const { result } = renderHook(() => useStarredAgents(userA))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        act(() => result.current.toggleStar('agent-1'))
        act(() => result.current.toggleStar('agent-2'))
        act(() => result.current.toggleStar('agent-3'))

        await act(async () => {
            await vi.advanceTimersByTimeAsync(600)
        })

        expect(mockSetUserConfig).toHaveBeenCalledTimes(1)
        expect(mockSetUserConfig).toHaveBeenCalledWith('starred_agents', ['agent-1', 'agent-2', 'agent-3'])
    })

    // The race this was extracted for: A's in-flight config response landing
    // after B has signed in would put A's stars in B's state.
    it('discards a response that outlived its account', async () => {
        let resolveA: (v: unknown) => void = () => {}
        mockUserConfig.mockImplementationOnce(
            () => new Promise((res) => { resolveA = res as (v: unknown) => void })
        )
        mockUserConfig.mockImplementationOnce(async () => ({ starred_agents: ['b-agent'] }))

        const { result, rerender } = renderHook(({ u }) => useStarredAgents(u), {
            initialProps: { u: userA as User | null },
        })

        rerender({ u: userB })
        await waitFor(() => expect(result.current.starredIds).toEqual(['b-agent']))

        await act(async () => {
            resolveA({ starred_agents: ['a-agent'] })
        })

        expect(result.current.starredIds).toEqual(['b-agent'])
    })

    // hasUserEdited staying true across a switch would make B's initial load
    // look like an edit and persist it over B's real config.
    it('does not persist the next account\'s initial load', async () => {
        vi.useFakeTimers()
        mockUserConfig.mockResolvedValue({ starred_agents: ['agent-1'] })

        const { result, rerender } = renderHook(({ u }) => useStarredAgents(u), {
            initialProps: { u: userA as User | null },
        })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(0)
        })

        // A edits, which arms the persist path.
        act(() => result.current.toggleStar('agent-2'))
        await act(async () => {
            await vi.advanceTimersByTimeAsync(600)
        })
        mockSetUserConfig.mockClear()
        mockDeleteUserConfig.mockClear()

        rerender({ u: userB })
        await act(async () => {
            await vi.advanceTimersByTimeAsync(2000)
        })

        expect(mockSetUserConfig).not.toHaveBeenCalled()
        expect(mockDeleteUserConfig).not.toHaveBeenCalled()
    })
})