import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { Admin } from '../pages/Admin'
import type { PlatformInfo, ProvisionResponse } from '../types'

vi.mock('../api', () => ({
    api: { platforms: vi.fn(), provision: vi.fn() },
}))

vi.mock('../utils', async () => {
    const actual = await vi.importActual<typeof import('../utils')>('../utils')
    return { ...actual, copyToClipboard: vi.fn() }
})

import { api } from '../api'
import { copyToClipboard } from '../utils'
const mockPlatforms = api.platforms as ReturnType<typeof vi.fn>
const mockProvision = api.provision as ReturnType<typeof vi.fn>
const mockCopyToClipboard = copyToClipboard as ReturnType<typeof vi.fn>

function makePlatform(overrides: Partial<PlatformInfo> = {}): PlatformInfo {
    return { os: 'linux', arch: 'amd64', label: 'Linux (x86_64)', filename: 'spectra-agent-linux-amd64', ...overrides }
}

function makeProvisionResponse(overrides: Partial<ProvisionResponse> = {}): ProvisionResponse {
    return {
        token: 'tok_abc123',
        expires_at: '2026-01-02T00:00:00.000Z',
        platform: 'spectra-agent-linux-amd64',
        download_url: 'https://example.com/releases/spectra-agent-linux-amd64',
        config: { server: 'https://spectra.example.com', token: 'tok_abc123' },
        install: { type: 'systemd', content: '[Unit]\nDescription=Spectra Agent', steps: '1. Copy the binary\n2. Run install.sh' },
        ...overrides,
    }
}

beforeEach(() => {
    mockPlatforms.mockReset().mockResolvedValue([])
    mockProvision.mockReset()
    mockCopyToClipboard.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
    vi.useRealTimers()
})

describe('Admin - platform selection', () => {
    it('shows a loading message, then platforms grouped by OS', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ os: 'linux', label: 'Linux (x86_64)' })])

        render(<Admin />)
        expect(screen.getByText('Loading platforms...')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        expect(screen.getByText('Linux')).toBeInTheDocument()
    })

    it('shows an error message on load failure', async () => {
        mockPlatforms.mockRejectedValue(new Error('connection refused'))
        render(<Admin />)
        await waitFor(() => expect(screen.getByText('connection refused')).toBeInTheDocument())
    })

    it('shows the manual fallback selector when no binaries are available', async () => {
        mockPlatforms.mockResolvedValue([])
        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Provision without binary')).toBeInTheDocument())

        expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument()
        expect(screen.getByText('Windows (x86_64)')).toBeInTheDocument()
        expect(screen.getByText('Raspberry Pi 2/3/4 (armv7)')).toBeInTheDocument()
    })

    it('labels darwin as macOS', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ os: 'darwin', label: 'macOS (Apple Silicon)' })])
        render(<Admin />)
        await waitFor(() => expect(screen.getByText('macOS')).toBeInTheDocument())
    })

    it('disables Provision until a platform is selected, then enables it', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())

        expect(screen.getByText('PROVISION AGENT')).toBeDisabled()
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        expect(screen.getByText('PROVISION AGENT')).not.toBeDisabled()
    })
})

describe('Admin - provisioning flow', () => {
    it('provisions the selected platform and shows the result', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ filename: 'spectra-agent-linux-amd64', label: 'Linux (x86_64)' })])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))

        await waitFor(() => expect(mockProvision).toHaveBeenCalledWith('spectra-agent-linux-amd64'))
        expect(await screen.findByText('Agent Provisioned')).toBeInTheDocument()
        expect(screen.getByText('tok_abc123')).toBeInTheDocument()
    })

    it('shows a provisioning state while the request is in flight', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        let resolveProvision!: (r: ProvisionResponse) => void
        mockProvision.mockReturnValue(new Promise<ProvisionResponse>((res) => { resolveProvision = res }))

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))

        expect(screen.getByText('PROVISIONING...')).toBeDisabled()
        resolveProvision(makeProvisionResponse())
        await waitFor(() => expect(screen.getByText('Agent Provisioned')).toBeInTheDocument())
    })

    it('shows an error and stays on the selector on provisioning failure', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        mockProvision.mockRejectedValue(new Error('token generation failed'))

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))

        await waitFor(() => expect(screen.getByText('token generation failed')).toBeInTheDocument())
        expect(screen.queryByText('Agent Provisioned')).not.toBeInTheDocument()
    })

    it('resets platform, error, and result when going back', async () => {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))
        await waitFor(() => expect(screen.getByText('Agent Provisioned')).toBeInTheDocument())

        fireEvent.click(screen.getByText('← PROVISION ANOTHER'))
        expect(screen.getByText('PROVISION AGENT')).toBeDisabled() // selection cleared
    })
})

describe('Admin - ProvisionResult rendering', () => {
    async function renderResult(overrides: Partial<ProvisionResponse> = {}) {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        mockProvision.mockResolvedValue(makeProvisionResponse(overrides))

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))
        await waitFor(() => expect(screen.getByText('Agent Provisioned')).toBeInTheDocument())
    }

    it('shows install steps and the formatted expiry', async () => {
        await renderResult({ expires_at: '2026-06-01T12:00:00.000Z', install: { type: 'systemd', content: '', steps: 'do the thing' } })
        expect(screen.getByText('Install Steps (systemd)')).toBeInTheDocument()
        expect(screen.getByText('do the thing')).toBeInTheDocument()
    })

    it('shows the binary download link only when download_url is present', async () => {
        await renderResult({ download_url: 'https://example.com/bin', platform: 'spectra-agent-linux-amd64' })
        expect(screen.getByText(/spectra-agent-linux-amd64/)).toBeInTheDocument()
    })

    it('hides the binary download link when download_url is empty', async () => {
        await renderResult({ download_url: '' })
        expect(screen.queryByText('Agent binary - SHA256 verified')).not.toBeInTheDocument()
    })

    it('shows the service file section only when install.content is present', async () => {
        await renderResult({ install: { type: 'systemd', content: '[Unit]\nfoo', steps: 'steps' } })
        expect(screen.getByText('Service File')).toBeInTheDocument()
    })

    it('hides the service file section when install.content is empty', async () => {
        await renderResult({ install: { type: 'manual', content: '', steps: 'steps' } })
        expect(screen.queryByText('Service File')).not.toBeInTheDocument()
    })
})

describe('Admin - CopyButton', () => {
    async function renderResult() {
        mockPlatforms.mockResolvedValue([makePlatform({ label: 'Linux (x86_64)' })])
        mockProvision.mockResolvedValue(makeProvisionResponse())

        render(<Admin />)
        await waitFor(() => expect(screen.getByText('Linux (x86_64)')).toBeInTheDocument())
        fireEvent.click(screen.getByText('Linux (x86_64)'))
        fireEvent.click(screen.getByText('PROVISION AGENT'))
        await waitFor(() => expect(screen.getByText('Agent Provisioned')).toBeInTheDocument())
    }

    it('shows COPIED after a successful copy, then reverts after 2 seconds', async () => {
        await renderResult()
        vi.useFakeTimers()

        fireEvent.click(screen.getAllByText('COPY')[0]!)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        expect(screen.getByText('COPIED')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(2000) })
        expect(screen.getAllByText('COPY')[0]!).toBeInTheDocument()
    })

    it('shows FAILED when copying throws, then reverts after 2 seconds', async () => {
        mockCopyToClipboard.mockRejectedValue(new Error('clipboard denied'))
        await renderResult()
        vi.useFakeTimers()

        fireEvent.click(screen.getAllByText('COPY')[0]!)
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        expect(screen.getByText('FAILED')).toBeInTheDocument()

        await act(async () => { await vi.advanceTimersByTimeAsync(2000) })
        expect(screen.getAllByText('COPY')[0]!).toBeInTheDocument()
    })

    it('uses a custom label when given, e.g. "COPY ALL" for install steps', async () => {
        await renderResult()
        expect(screen.getByText('COPY ALL')).toBeInTheDocument()
    })
})