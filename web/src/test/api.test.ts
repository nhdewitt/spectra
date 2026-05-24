import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api, HttpError } from '../api'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

function jsonResponse(data: unknown, status = 200) {
    return {
        ok: true,
        status,
        json: () => Promise.resolve(data),
        text: () => Promise.resolve(JSON.stringify(data)),
    }
}

function errorResponse(status: number, body = '') {
    return {
        ok: false,
        status,
        statusText: 'Error',
        json: () => Promise.resolve({}),
        text: () => Promise.resolve(body),
    }
}

function lastFetchCall() {
    const [url, opts] = mockFetch.mock.calls.at(-1) as [string, RequestInit & { headers: Record<string, string> }]
    return {
        url,
        method: opts.method ?? 'GET',
        headers: opts.headers,
        credentials: opts.credentials,
        body: opts.body ? JSON.parse(opts.body as string) : undefined,
        signal: opts.signal,
    }
}

beforeEach(() => {
    mockFetch.mockReset()
    window.__spectraLogout = undefined
})

describe('apiFetch intervals', () => {
    it('includes credentials and content-type', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ username: 'admin', role: 'admin' }))
        await api.me()

        const call = lastFetchCall()
        expect(call.credentials).toBe('include')
        expect(call.headers['Content-Type']).toBe('application/json')
    })

    it('calls __spectraLogout on 401', async () => {
        const logout = vi.fn()
        window.__spectraLogout = logout
        mockFetch.mockResolvedValueOnce(errorResponse(401))

        await expect(api.me()).rejects.toThrow(HttpError)
        expect(logout).toHaveBeenCalled()
    })

    it('throws HttpError with status and message on non-ok response', async () => {
        mockFetch.mockResolvedValueOnce(errorResponse(500, 'database error'))

        try {
            await api.me()
            expect.fail('should have thrown')
        } catch (e) {
            expect(e).toBeInstanceOf(HttpError)
            expect((e as HttpError).status).toBe(500)
            expect((e as HttpError).message).toBe('database error')
        }
    })

    it('returns null for 204 responses', async() => {
        mockFetch.mockResolvedValueOnce({
            ok: true,
            status: 204,
            json: () => Promise.resolve(null),
            text: () => Promise.resolve(''),
        })

        const result = await api.logout()
        expect(result).toBeNull()
    })
})

describe('auth endpoints', () => {
    it('login posts credentials', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ username: 'admin', role: 'admin' }))
        await api.login('admin', 'password123')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/auth/login')
        expect(call.method).toBe('POST')
        expect(call.body).toEqual({ username: 'admin', password: 'password123' })
    })

    it('logout posts to correct endpoint', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse(null, 204))
        await api.logout()

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/auth/logout')
        expect(call.method).toBe('POST')
    })
})

describe('overview endpoints', () => {
    it('fetches overview', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.overview()

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/overview')
    })

    it('fetches sparklines', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({}))
        await api.sparklines()

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/overview/sparklines')
    })

    it('fetches fleet heatmap with start and end', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.fleetHeatmap('2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/overview/heatmap?start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z')
    })
})

describe('agent detail endpoints', () => {
    const id = 'abc-123'

    it('fetches single agent', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({}))
        await api.agent(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/abc-123')
    })

    it('deletes agent', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse(null, 204))
        await api.deleteAgent(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/abc-123')
        expect(call.method).toBe('DELETE')
    })
})

describe('time-series metric endpoints', () => {
    const id = 'agent-1'

    it('uses default range when not specified', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentCPU(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/cpu?range=1h')
    })

    it('uses quick range selection', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentMemory(id, { type: 'quick', range: '7d' })

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/memory?range=7d')
    })

    it('uses custom range selection with encoding', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentDisk(id, {
            type: 'custom',
            start: '2026-01-01T00:00:00Z',
            end: '2026-01-2T00:00:00Z',
        })

        const call = lastFetchCall()
        expect(call.url).toContain('start=')
        expect(call.url).toContain('end=')
    })

    it('passes signal through for abort', async () => {
        const controller = new AbortController()
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentCPU(id, { type: 'quick', range: '1h' }, { signal: controller.signal })

        const call = lastFetchCall()
        expect(call.signal).toBe(controller.signal)
    })

    const metricEndpoints = [
        ['agentCPU', 'cpu'],
        ['agentMemory', 'memory'],
        ['agentDisk', 'disk'],
        ['agentDiskIO', 'diskio'],
        ['agentNetwork', 'network'],
        ['agentTemperature', 'temperature'],
        ['agentSystem', 'system'],
        ['agentContainers', 'containers'],
        ['agentWifi', 'wifi'],
        ['agentPi', 'pi'],
    ] as const

    metricEndpoints.forEach(([method, path]) => {
        it(`${method} hits /agents/{id}/${path}`, async () => {
            mockFetch.mockResolvedValueOnce(jsonResponse([]))
            await (api[method] as Function)(id)

            const call = lastFetchCall()
            expect(call.url).toContain(`/agents/agent-1/${path}?`)
        })
    })
})

describe('current state endpoints', () => {
    const id = 'agent-1'

    it('fetches processes with sort and limit', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentProcesses(id, 'memory', 50)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/processes?sort=memory&limit=50')
    })

    it('uses default sort and limit for processes', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentProcesses(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/processes?sort=cpu&limit=20')
    })

    it('fetches services', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentServices(id)
        
        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/services')
    })

    it('fetches applications', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentApplications(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/applications')
    })

    it('fetches updates', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentUpdates(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/updates')
    })

    it('fetches latest system', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.agentSystemLatest(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/system/latest')
    })
})

describe('agent config endpoints', () => {
    const id = 'agent-1'

    it('fetches config', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({}))
        await api.agentConfig(id)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/config')
    })

    it('sets config with PUT', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse(null, 204))
        await api.setAgentConfig(id, 'log_level', 'debug')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/config')
        expect(call.method).toBe('PUT')
        expect(call.body).toEqual({ key: 'log_level', value: 'debug' })
    })

    it('deletes config with key param', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse(null, 204))
        await api.deleteAgentConfig(id, 'labels')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/agents/agent-1/config?key=labels')
        expect(call.method).toBe('DELETE')
    })
})

describe('diagnostics endpoints', () => {
    const agentId = 'agent-1'

    it('triggers logs with default level', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ command_id: 'cmd-1', message: 'queued' }))
        await api.triggerLogs(agentId)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/logs?agent=agent-1&level=WARNING')
        expect(call.method).toBe('POST')
    })

    it('triggers disk usage', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ command_id: 'cmd-2', message: 'queued' }))
        await api.triggerDisk(agentId, '/var', 10)

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/disk?agent=agent-1&top_n=10&path=%2Fvar')
    })

    it('triggers disk without path', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ command_id: 'cmd-3', message: 'queued' }))
        await api.triggerDisk(agentId, '', 5)
        
        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/disk?agent=agent-1&top_n=5')
    })

    it('triggers network with target', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ command_id: 'cmd-4', message: 'queued' }))
        await api.triggerNetwork(agentId, 'ping', '8.8.8.8')
        
        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/network?agent=agent-1&action=ping&target=8.8.8.8')
    })

    it('triggers network without target', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ command_id: 'cmd-5', messaage: 'queued' }))
        await api.triggerNetwork(agentId, 'netstat')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/network?agent=agent-1&action=netstat')
    })

    it('fetches command result', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({ id: 'cmd-1', done: true }))
        await api.commandResult('cmd-1')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/commands/cmd-1')
    })
})

describe('provisioning endpoints', () => {
    it('fetches platforms', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse([]))
        await api.platforms()

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/platforms')
    })

    it('provisions agent', async () => {
        mockFetch.mockResolvedValueOnce(jsonResponse({}))
        await api.provision('linux-amd64')

        const call = lastFetchCall()
        expect(call.url).toBe('/api/v1/admin/provision')
        expect(call.method).toBe('POST')
        expect(call.body).toEqual({ platform: 'linux-amd64' })
    })

    it('generates token', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'abc123' }))
    await api.generateToken()

    const call = lastFetchCall()
    expect(call.url).toBe('/api/v1/admin/tokens')
    expect(call.method).toBe('POST')
    })

    it('fetches upgrade instructions', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ type: 'systemd', steps: '...' }))
    await api.upgradeInstructions('agent-1')

    const call = lastFetchCall()
    expect(call.url).toBe('/api/v1/agents/agent-1/upgrade-instructions')
    })

    it('fetches uninstall instructions', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ type: 'systemd', steps: '...' }))
    await api.uninstallInstructions('agent-1')

    const call = lastFetchCall()
    expect(call.url).toBe('/api/v1/agents/agent-1/uninstall-instructions')
    })    
})