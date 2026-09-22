import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useNavigation } from '../hooks/useNavigation'
import type { OverviewAgent } from '../types'

const agentA = { id: 'a', hostname: 'alpha' } as OverviewAgent
const agentB = { id: 'b', hostname: 'bravo' } as OverviewAgent

/** jsdom implements pushState but does not fire popstate for history.back(). */
function popTo(state: unknown) {
    window.dispatchEvent(new PopStateEvent('popstate', { state }))
}

beforeEach(() => {
    window.history.replaceState(null, '')
})

describe('useNavigation', () => {
    it('starts on the overview with nothing selected', () => {
        const { result } = renderHook(() => useNavigation())

        expect(result.current.page).toBe('overview')
        expect(result.current.selectedAgent).toBeNull()
    })

    // Without a seeded entry the first push has nothing behind it and back
    // leaves the app.
    it('seeds the current history entry on mount', () => {
        renderHook(() => useNavigation())

        expect(window.history.state?.spectraNav).toEqual({ page: 'overview', agent: null })
    })

    it('pushes a history entry when navigating', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.navigate('alerts'))

        expect(result.current.page).toBe('alerts')
        expect(window.history.state.spectraNav).toEqual({ page: 'alerts', agent: null })
    })

    it('selecting an agent goes to detail', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))

        expect(result.current.page).toBe('detail')
        expect(result.current.selectedAgent).toEqual(agentA)
    })

    // Picking a different agent from within diagnostics should not bounce the
    // user to the detail page.
    it('selecting an agent from diagnostics stays in diagnostics', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.navigate('diagnostics'))
        act(() => result.current.selectAgent(agentA))

        expect(result.current.page).toBe('diagnostics')
        expect(result.current.selectedAgent).toEqual(agentA)
    })

    it('leaving the agent-scoped pages drops the selection', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))
        act(() => result.current.navigate('agents'))

        expect(result.current.selectedAgent).toBeNull()
    })

    it('keeps the selection moving between detail and diagnostics', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))
        act(() => result.current.navigate('diagnostics'))

        expect(result.current.selectedAgent).toEqual(agentA)
    })

    // The point of the hook: a browser back does what the in-app back does.
    it('restores the previous view on popstate', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))
        expect(result.current.page).toBe('detail')

        act(() => popTo({ spectraNav: { page: 'overview', agent: null } }))

        expect(result.current.page).toBe('overview')
        expect(result.current.selectedAgent).toBeNull()
    })

    it('restores the agent carried in the history entry', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))
        act(() => result.current.selectAgent(agentB))
        act(() => popTo({ spectraNav: { page: 'detail', agent: agentA } }))

        expect(result.current.selectedAgent).toEqual(agentA)
    })

    // An entry pushed by something other than this hook has no state of ours.
    it('falls back to the overview for a foreign history entry', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.navigate('alerts'))
        act(() => popTo(null))

        expect(result.current.page).toBe('overview')
        expect(result.current.selectedAgent).toBeNull()
    })

    // Restoring a popped entry must not push it back on, or back would never
    // get past it.
    it('does not push a new entry while handling a pop', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.navigate('alerts'))
        const before = window.history.length

        act(() => popTo({ spectraNav: { page: 'overview', agent: null } }))

        expect(window.history.length).toBe(before)
    })

    // Signing out replaces rather than pushes, so back cannot return to the
    // previous account's view.
    it('reset replaces the current entry', () => {
        const { result } = renderHook(() => useNavigation())

        act(() => result.current.selectAgent(agentA))
        const before = window.history.length

        act(() => result.current.reset())

        expect(result.current.page).toBe('overview')
        expect(result.current.selectedAgent).toBeNull()
        expect(window.history.length).toBe(before)
        expect(window.history.state.spectraNav).toEqual({ page: 'overview', agent: null })
    })

    it('detaches the popstate listener on unmount', () => {
        const { result, unmount } = renderHook(() => useNavigation())

        act(() => result.current.navigate('alerts'))
        unmount()

        // No act() wrapper: a listener still attached would warn about a state
        // update on an unmounted component.
        popTo({ spectraNav: { page: 'overview', agent: null } })
    })
})