import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { AgentDetail } from '../pages/AgentDetail'
import type { Agent, OverviewAgent, SystemMetric, User, RangeSelection } from '../types'

vi.mock('../api', () => ({
    api: { agent: vi.fn(), agentSystemLatest: vi.fn(), overviewPage: vi.fn() },
}))

vi.mock('../components/MetricsTab', () => ({
    MetricsTab: (props: { agentId: string; rangeSel: RangeSelection; cores: number }) => (
        <div data-testid="tab-metrics" data-agent={props.agentId} data-cores={props.cores} />
    ),
}))
vi.mock('../components/ProcessesTab', () => ({
    ProcessesTab: (props: { agentId: string }) => <div data-testid="tab-processes" data-agent={props.agentId} />,
}))
vi.mock('../components/ServicesTab', () => ({
    ServicesTab: (props: { agentId: string }) => <div data-testid="tab-services" data-agent={props.agentId} />,
}))
vi.mock('../components/ApplicationsTab', () => ({
    ApplicationsTab: (props: { agentId: string }) => <div data-testid="tab-apps" data-agent={props.agentId} />,
}))
vi.mock('../components/UpdatesTab', () => ({
    UpdatesTab: (props: { agentId: string }) => <div data-testid="tab-updates" data-agent={props.agentId} />,
}))
vi.mock('../components/ContainersTab', () => ({
    ContainersTab: (props: { agentId: string }) => <div data-testid="tab-containers" data-agent={props.agentId} />,
}))
vi.mock('../components/AgentLabelChips', () => ({
    AgentLabelChips: (props: { agentId: string; isAdmin: boolean }) => (
        <div data-testid="label-chips" data-admin={String(props.isAdmin)} />
    ),
}))

import { api } from '../api'
const mockAgent = api.agent as ReturnType<typeof vi.fn>
const mockSystemLatest = api.agentSystemLatest as ReturnType<typeof vi.fn>
const mockOverviewPage = api.overviewPage as ReturnType<typeof vi.fn>

function makeOverviewAgent(overrides: Partial<OverviewAgent> = {}): OverviewAgent {
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
        updated_at: '',
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

function makeAgent(overrides: Partial<Agent> = {}): Agent {
    return {
        id: 'a1',
        hostname: 'test-host-1',
        os: 'linux',
        platform: 'proxmox',
        arch: 'amd64',
        cpu_model: 'AMD EPYC',
        cpu_cores: 8,
        ram_total: 34359738368,
        registered_at: '',
        last_seen: new Date().toISOString(),
        ip_address: '198.51.100.10',
        ...overrides,
    }
}

function makeSystemInfo(overrides: Partial<SystemMetric> = {}): SystemMetric {
    return {
        time: '',
        agent_id: 'a1',
        uptime: 3600,
        process_count: 120,
        user_count: 2,
        boot_time: '1735689600',
        ...overrides,
    }
}

function makeUser(overrides: Partial<User> = {}): User {
    return { id: 'u1', username: 'test-admin', role: 'admin', ...overrides } as User
}

const noop = () => {}

beforeEach(() => {
    mockAgent.mockReset().mockResolvedValue(undefined)
    mockSystemLatest.mockReset().mockResolvedValue(undefined)
    mockOverviewPage.mockReset().mockResolvedValue({ agents: [], page: 1, size: 20 })
})

afterEach(() => {
    vi.useRealTimers()
})

describe('AgentDetail - header', () => {
    it('shows the hostname and calls onBack', () => {
        const onBack = vi.fn()
        render(
            <AgentDetail agent={makeOverviewAgent()} user={makeUser()} onSelectAgent={noop}
                onBack={onBack} starredIds={[]} onToggleStar={noop} />
        )
        expect(screen.getByText('test-host-1')).toBeInTheDocument()
        fireEvent.click(screen.getByText('← BACK'))
        expect(onBack).toHaveBeenCalledTimes(1)
    })

    it('shows the reboot badge only when reboot_required is true', () => {
        const { rerender } = render(
            <AgentDetail agent={makeOverviewAgent({ reboot_required: false })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )
        expect(screen.queryByText('REBOOT')).not.toBeInTheDocument()

        rerender(
            <AgentDetail agent={makeOverviewAgent({ reboot_required: true })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )
        expect(screen.getByText('REBOOT')).toBeInTheDocument()
    })

    it('shows a filled star when starred and calls onToggleStar', () => {
        const onToggleStar = vi.fn()
        render(
            <AgentDetail agent={makeOverviewAgent({ id: 'a1' })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={['a1']} onToggleStar={onToggleStar} />
        )
        expect(screen.getByTitle('Remove from quick access')).toBeInTheDocument()
        fireEvent.click(screen.getByTitle('Remove from quick access'))
        expect(onToggleStar).toHaveBeenCalledWith('a1')
    })

    it('pluralizes core count correctly', () => {
        const { rerender } = render(
            <AgentDetail agent={makeOverviewAgent({ cpu_cores: 1 })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )
        expect(screen.getByText('1 core')).toBeInTheDocument()

        rerender(
            <AgentDetail agent={makeOverviewAgent({ cpu_cores: 4 })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )
        expect(screen.getByText('4 cores')).toBeInTheDocument()
    })
})

describe('AgentDetail - agent switcher dropdown', () => {
    it('opens on hostname click, shows search results, and switches on selection', async () => {
        const onSelectAgent = vi.fn()
        const zeta = makeOverviewAgent({ id: 'z', hostname: 'zeta' })
        const alpha = makeOverviewAgent({ id: 'a', hostname: 'alpha' })
        mockOverviewPage.mockResolvedValue({ agents: [alpha, zeta], page: 1, size: 20 })

        render(
            <AgentDetail agent={zeta} user={makeUser()} onSelectAgent={onSelectAgent}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        expect(screen.queryByText('alpha')).not.toBeInTheDocument()
        fireEvent.click(screen.getByText('zeta'))

        await waitFor(() => expect(screen.getByText('alpha')).toBeInTheDocument())
        expect(mockOverviewPage).toHaveBeenCalledWith(
            expect.objectContaining({ search: undefined, sort: 'hostname', order: 'asc' })
        )

        fireEvent.click(screen.getByText('alpha'))
        expect(onSelectAgent).toHaveBeenCalledWith(alpha)
    })

    it('closes the dropdown on Escape', async () => {
        const agent = makeOverviewAgent()
        mockOverviewPage.mockResolvedValue({ agents: [agent], page: 1, size: 20 })

        render(
            <AgentDetail agent={agent} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        fireEvent.click(screen.getByText('test-host-1'))
        await waitFor(() => expect(screen.getAllByText('test-host-1').length).toBeGreaterThan(1)) // header + search result

        fireEvent.keyDown(window, { key: 'Escape' })
        expect(screen.getAllByText('test-host-1')).toHaveLength(1) // dropdown closed
    })

    it('debounces the search box before querying', async () => {
        vi.useFakeTimers()
        const agent = makeOverviewAgent()
        mockOverviewPage.mockResolvedValue({ agents: [agent], page: 1, size: 20 })

        render(
            <AgentDetail agent={agent} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        fireEvent.click(screen.getByText('test-host-1'))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        mockOverviewPage.mockClear()

        fireEvent.change(screen.getByPlaceholderText('Search agents...'), { target: { value: 'test' } })
        await act(async () => { await vi.advanceTimersByTimeAsync(100) })
        expect(mockOverviewPage).not.toHaveBeenCalled()

        await act(async () => { await vi.advanceTimersByTimeAsync(200) })
        expect(mockOverviewPage).toHaveBeenCalledWith(expect.objectContaining({ search: 'test' }))

        vi.useRealTimers()
    })
})

describe('AgentDetail - tabs', () => {
    it('shows the metrics tab by default, wired with agentId, rangeSel, and cores', () => {
        render(
            <AgentDetail agent={makeOverviewAgent({ id: 'a1', cpu_cores: 4 })} user={makeUser()}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )
        const tab = screen.getByTestId('tab-metrics')
        expect(tab).toHaveAttribute('data-agent', 'a1')
        expect(tab).toHaveAttribute('data-cores', '4')
    })

    it('switches between every tab', () => {
        const agent = makeOverviewAgent()
        render(
            <AgentDetail agent={agent} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        fireEvent.click(screen.getByText('processes'))
        expect(screen.getByTestId('tab-processes')).toBeInTheDocument()

        fireEvent.click(screen.getByText('services'))
        expect(screen.getByTestId('tab-services')).toBeInTheDocument()

        fireEvent.click(screen.getByText('containers'))
        expect(screen.getByTestId('tab-containers')).toBeInTheDocument()

        fireEvent.click(screen.getByText('apps'))
        expect(screen.getByTestId('tab-apps')).toBeInTheDocument()

        fireEvent.click(screen.getByText('updates'))
        expect(screen.getByTestId('tab-updates')).toBeInTheDocument()
    })

    it('dims the time range picker outside metrics/containers tabs', () => {
        const agent = makeOverviewAgent()
        const { container } = render(
            <AgentDetail agent={agent} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        const pickerWrapper = container.querySelector('[style*="pointer-events"]') as HTMLElement
        expect(pickerWrapper.style.opacity).toBe('1')

        fireEvent.click(screen.getByText('processes'))
        expect(pickerWrapper.style.opacity).toBe('0.3')

        fireEvent.click(screen.getByText('containers'))
        expect(pickerWrapper.style.opacity).toBe('1')
    })
})

describe('AgentDetail - live agent / system info polling', () => {
    it('shows system info stats once loaded', async () => {
        mockSystemLatest.mockResolvedValue(makeSystemInfo({ user_count: 3, process_count: 150 }))

        render(
            <AgentDetail agent={makeOverviewAgent()} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        await waitFor(() => expect(screen.getByText('3')).toBeInTheDocument())
        expect(screen.getByText('150')).toBeInTheDocument()
    })

    it('shows CPU model, RAM, and label chips once the live agent loads', async () => {
        mockAgent.mockResolvedValue(makeAgent({ cpu_model: 'AMD EPYC 7302P' }))

        render(
            <AgentDetail agent={makeOverviewAgent()} user={makeUser({ role: 'admin' })}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        await waitFor(() => expect(screen.getByText('AMD EPYC 7302P')).toBeInTheDocument())
        expect(screen.getByTestId('label-chips')).toHaveAttribute('data-admin', 'true')
    })

    it('marks label chips as non-admin for a viewer', async () => {
        mockAgent.mockResolvedValue(makeAgent())

        render(
            <AgentDetail agent={makeOverviewAgent()} user={makeUser({ role: 'viewer' })}
                onSelectAgent={noop} onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        await waitFor(() => expect(screen.getByTestId('label-chips')).toBeInTheDocument())
        expect(screen.getByTestId('label-chips')).toHaveAttribute('data-admin', 'false')
    })

    it('shows the live IP address once loaded, alongside the static info row', async () => {
        mockAgent.mockResolvedValue(makeAgent({ ip_address: '198.51.100.42' }))

        render(
            <AgentDetail agent={makeOverviewAgent()} user={makeUser()} onSelectAgent={noop}
                onBack={noop} starredIds={[]} onToggleStar={noop} />
        )

        await waitFor(() => expect(screen.getByText('198.51.100.42')).toBeInTheDocument())
    })
})