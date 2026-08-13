import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Sidebar } from '../components/Sidebar'
import type { User, OverviewAgent, Page } from '../types'

vi.mock('../api', () => ({
    api: { overviewPage: vi.fn() },
}))

import { api } from '../api'
const mockOverviewPage = api.overviewPage as ReturnType<typeof vi.fn>

beforeEach(() => {
    mockOverviewPage.mockReset().mockResolvedValue({ agents: [], page: 1, size: 20 })
})

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'admin', role: 'admin', ...overrides } as User
}

function makeAgent(overrides: Partial<OverviewAgent> = {}): OverviewAgent {
    return {
        id: 'a1',
        hostname: 'test-host-1',
        os: 'linux',
        platform: 'proxmox',
        arch: 'amd64',
        cpu_cores: 8,
        last_seen: new Date().toISOString(),
        version: '1.0.0',
        cpu_usage: 10,
        load_normalized: 0.1,
        ram_percent: 20,
        swap_percent: 0,
        disk_max_percent: 30,
        net_rx_bytes: 0,
        net_tx_bytes: 0,
        max_temp: 40,
        uptime: 100,
        process_count: 10,
        reboot_required: false,
        updated_at: new Date().toISOString(),
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

const noop = () => {}

function renderSidebar(overrides: {
    user?: User
    currentPage?: Page
    selectedAgent?: OverviewAgent | null
    starredIds?: string[]
    version?: string
    onNavigate?: (p: Page) => void
    onSelectAgent?: (a: OverviewAgent) => void
} = {}) {
    return render(
        <Sidebar
            user={overrides.user ?? makeUser()}
            currentPage={overrides.currentPage ?? 'overview'}
            onNavigate={overrides.onNavigate ?? noop}
            selectedAgent={overrides.selectedAgent ?? null}
            onSelectAgent={overrides.onSelectAgent ?? noop}
            starredIds={overrides.starredIds ?? []}
            version={overrides.version ?? '1.0.0'}
        />
    )
}

describe('Sidebar', () => {
    it('shows all nav items for an admin user, with Diagnostics collapsed by default', () => {
        renderSidebar({ user: makeUser({ role: 'admin' }), currentPage: 'overview' })

        expect(screen.getByText('Fleet Overview')).toBeInTheDocument()
        expect(screen.getByText('Agent Detail')).toBeInTheDocument()
        expect(screen.getByText('Agent Mgmt')).toBeInTheDocument()
        expect(screen.getByText('Tags')).toBeInTheDocument()
        expect(screen.getByText('Alerts')).toBeInTheDocument()
        expect(screen.getByText('User Mgmt')).toBeInTheDocument()
        expect(screen.queryByText('Diagnostics')).not.toBeInTheDocument()
    })

    it('hides User Mgmt for a non-admin user', () => {
        renderSidebar({ user: makeUser({ role: 'viewer' }) })
        expect(screen.queryByText('User Mgmt')).not.toBeInTheDocument()
    })

    it('reveals Diagnostics and navigates to detail when Agent Detail is clicked', () => {
        const onNavigate = vi.fn()
        renderSidebar({ currentPage: 'overview', onNavigate })

        expect(screen.queryByText('Diagnostics')).not.toBeInTheDocument()

        fireEvent.click(screen.getByText('Agent Detail'))

        expect(onNavigate).toHaveBeenCalledWith('detail')
        expect(screen.getByText('Diagnostics')).toBeInTheDocument()
    })

    it('auto-expands the detail section when mounted directly on detail or diagnostics', () => {
        renderSidebar({ currentPage: 'diagnostics' })
        expect(screen.getByText('Diagnostics')).toBeInTheDocument()
    })

    it('calls onNavigate with the clicked item key for plain nav items', () => {
        const onNavigate = vi.fn()
        renderSidebar({ onNavigate })

        fireEvent.click(screen.getByText('Tags'))
        expect(onNavigate).toHaveBeenCalledWith('tags')

        fireEvent.click(screen.getByText('Alerts'))
        expect(onNavigate).toHaveBeenCalledWith('alerts')

        fireEvent.click(screen.getByText('Agent Mgmt'))
        expect(onNavigate).toHaveBeenCalledWith('agents')
    })

    it('hides the Quick Access section when there are no starred agents', () => {
        renderSidebar({ starredIds: [] })
        expect(screen.queryByText('Quick Access')).not.toBeInTheDocument()
    })

    it('shows only starred agents, sorted alphabetically by hostname', async () => {
        const zeta = makeAgent({ id: 'z', hostname: 'zeta' })
        const alpha = makeAgent({ id: 'a', hostname: 'alpha' })
        // Server already sorts/filters by the ids= filter - the mock returns
        // exactly what a real starred-only, hostname-sorted response would.
        mockOverviewPage.mockResolvedValue({ agents: [alpha, zeta], page: 1, size: 2 })

        renderSidebar({ starredIds: ['z', 'a'] })

        await waitFor(() => expect(screen.getByText('Quick Access')).toBeInTheDocument())
        expect(screen.queryByText('not-starred')).not.toBeInTheDocument()
        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ ids: ['z', 'a'], sort: 'hostname', order: 'asc' })
        )

        const alphaEl = screen.getByText('alpha')
        const zetaEl = screen.getByText('zeta')
        expect(alphaEl.compareDocumentPosition(zetaEl) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    it('selects the clicked starred agent and navigates to detail', async () => {
        const onSelectAgent = vi.fn()
        const onNavigate = vi.fn()
        const agent = makeAgent({ id: 'a1', hostname: 'test-host-1' })
        mockOverviewPage.mockResolvedValue({ agents: [agent], page: 1, size: 1 })

        renderSidebar({ starredIds: ['a1'], onSelectAgent, onNavigate })

        await waitFor(() => expect(screen.getByText('test-host-1')).toBeInTheDocument())
        fireEvent.click(screen.getByText('test-host-1'))

        expect(onSelectAgent).toHaveBeenCalledWith(agent)
        expect(onNavigate).toHaveBeenCalledWith('detail')
    })

    it('shows the version in the footer', () => {
        renderSidebar({ version: '1.2.3' })
        expect(screen.getByText('v1.2.3')).toBeInTheDocument()
    })

    it('shows an em dash when version is empty', () => {
        renderSidebar({ version: '' })
        expect(screen.getByText('—')).toBeInTheDocument()
    })

    it('navigates to settings when the username is clicked', () => {
        const onNavigate = vi.fn()
        renderSidebar({ user: makeUser({ username: 'test-admin' }), onNavigate })

        fireEvent.click(screen.getByText('test-admin'))
        expect(onNavigate).toHaveBeenCalledWith('settings')
    })

    it('navigates to overview when the logo/title is clicked', () => {
        const onNavigate = vi.fn()
        renderSidebar({ currentPage: 'settings', onNavigate })

        fireEvent.click(screen.getByText('Spectra'))
        expect(onNavigate).toHaveBeenCalledWith('overview')
    })
})