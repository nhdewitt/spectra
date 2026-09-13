import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    formatBytes,
    formatUptime,
    statusColor,
    severityColor,
    sortAgentsBySeverity,
    sortAgentsByStatus,
    formatNetworkRate,
    levelColor,
    severityOrder,
    logEntrySpan
} from '../utils';
import { themeVars } from '../theme';
import type { LogEntry, OverviewAgent } from '../types';
import { DEFAULT_THRESHOLDS } from '../types';

describe('formatBytes', () => {
    it('returns "0 B" for null/undefined/zero', () => {
        expect(formatBytes(null)).toBe('0 B')
        expect(formatBytes(undefined)).toBe('0 B')
        expect(formatBytes(0)).toBe('0 B')
    })

    it('formats bytes', () => {
        expect(formatBytes(500)).toBe('500.0 B')
    })

    it('formats kilobytes', () => {
        expect(formatBytes(1536)).toBe('1.5 KB')
    })

    it('formats megabytes', () => {
        expect(formatBytes(10 * 1024 * 1024)).toBe('10.0 MB')
    })

    it('formats gigabytes', () => {
        expect(formatBytes(1073741824)).toBe('1.0 GB')
    })

    it('formats terabytes', () => {
        expect(formatBytes(2 * 1024 ** 4)).toBe('2.0 TB')
    })

    it('caps unit at TB for very large values', () => {
        const result = formatBytes(1024 ** 5)
        expect(result).toContain('TB')
    })
})

describe('formatUptime', () => {
    it('returns dash for null/undefined/zero', () => {
        expect(formatUptime(null)).toBe('—')
        expect(formatUptime(undefined)).toBe('—')
        expect(formatUptime(0)).toBe('—')
    })

    it('formats hours and minutes', () => {
        expect(formatUptime(3661)).toBe('1h 1m')
    })

    it('formats days and hours', () => {
        expect(formatUptime(90000)).toBe('1d 1h')
    })

    it('shows 0h for sub-hour values', () => {
        expect(formatUptime(300)).toBe('0h 5m')
    })
})

describe('statusColor', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date('2026-01-01T00:00:00Z'))
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('returns textDim for null last_seen', () => {
        expect(statusColor({ last_seen: null }, DEFAULT_THRESHOLDS)).toBe(themeVars.textDim)
    })

    it('returns ok for recent heartbeat', () => {
        const recent = new Date(Date.now() - 30_000).toISOString()
        expect(statusColor({ last_seen: recent }, DEFAULT_THRESHOLDS)).toBe(themeVars.ok)
    })

    it('returns ok at exactly 119 seconds ago', () => {
        const ts = new Date(Date.now() - 119_000).toISOString()
        expect(statusColor({ last_seen: ts }, DEFAULT_THRESHOLDS)).toBe(themeVars.ok)
    })

    it('returns warn at exactly 120 seconds ago', () => {
        const ts = new Date(Date.now() - 120_000).toISOString()
        expect(statusColor({ last_seen: ts }, DEFAULT_THRESHOLDS)).toBe(themeVars.warn)
    })

    it('returns warn for stale heartbeat', () => {
        const stale_2m = new Date(Date.now() - 2 * 60_000).toISOString()
        const stale_9m = new Date(Date.now() - 9 * 60_000).toISOString()
        expect(statusColor({ last_seen: stale_2m }, DEFAULT_THRESHOLDS)).toBe(themeVars.warn)
        expect(statusColor({ last_seen: stale_9m }, DEFAULT_THRESHOLDS)).toBe(themeVars.warn)
    })

    it('returns warn at exactly 599 seconds ago', () => {
        const ts = new Date(Date.now() - 599_000).toISOString()
        expect(statusColor({ last_seen: ts }, DEFAULT_THRESHOLDS)).toBe(themeVars.warn)
    })

    it('returns danger at exactly 600 seconds ago', () => {
        const ts = new Date(Date.now() - 600_000).toISOString()
        expect(statusColor({ last_seen: ts }, DEFAULT_THRESHOLDS)).toBe(themeVars.danger)
    })

    it('returns danger for old heartbeat', () => {
        const old = new Date(Date.now() - 15 * 60_000).toISOString()
        expect(statusColor({ last_seen: old }, DEFAULT_THRESHOLDS)).toBe(themeVars.danger)
    })
})

describe('severityColor', () => {
    it('returns textMuted below warn threshold', () => {
        expect(severityColor(10, [50, 75, 90])).toBe(themeVars.textMuted)
    })

    it('returns warn at warn threshold', () => {
        expect(severityColor(75, [50, 75, 90])).toBe(themeVars.warn)
    })

    it('returns danger at danger threshold', () => {
        expect(severityColor(95, [50, 75, 90])).toBe(themeVars.danger)
    })
})

function makeAgent(overrides: Partial<OverviewAgent>): OverviewAgent {
    return {
        id: 'test-id',
        hostname: 'test-host',
        os: 'linux',
        platform: 'ubuntu',
        arch: 'amd64',
        cpu_cores: 4,
        last_seen: new Date().toISOString(),
        version: '1.0.0',
        cpu_usage: null,
        load_normalized: null,
        ram_percent: null,
        swap_percent: null,
        disk_max_percent: null,
        net_rx_bytes: null,
        net_tx_bytes: null,
        max_temp: null,
        uptime: null,
        process_count: null,
        reboot_required: null,
        updated_at: null,
        ip_address: null,
        ...overrides,
    }
}

describe('sortAgentsBySeverity', () => {
    it('sorts by combined cpu + disk + ram descending', () => {
        const low = makeAgent({ id: 'low', cpu_usage: 10, disk_max_percent: 10, ram_percent: 10 })
        const high = makeAgent({ id: 'high', cpu_usage: 90, disk_max_percent: 80, ram_percent: 70 })
        const mid = makeAgent({ id: 'mid', cpu_usage: 50, disk_max_percent: 40, ram_percent: 30 })

        const result = sortAgentsBySeverity([low, high, mid])
        expect(result.map(a => a.id)).toEqual(['high', 'mid', 'low'])
    })

    it('treats null values as zero', () => {
        const withNulls = makeAgent({ id: 'nulls', cpu_usage: null, disk_max_percent: null, ram_percent: null })
        const withValues = makeAgent({ id: 'values', cpu_usage: 50, disk_max_percent: 0, ram_percent: 0 })

        const result = sortAgentsBySeverity([withNulls, withValues])
        expect(result[0]!.id).toBe('values')
    })

    it('does not mutate the input array', () => {
        const agents = [
            makeAgent({ id: 'a', cpu_usage: 10 }),
            makeAgent({ id: 'b', cpu_usage: 90 }),
        ]
        const original = [...agents]
        sortAgentsBySeverity(agents)
        expect(agents.map(a => a.id)).toEqual(original.map(a => a.id))
    })
})

describe('sortAgentsByStatus', () => {
    it('groups online before stale before offline', () => {
        const online = makeAgent({ id: 'online', last_seen: new Date(Date.now() - 30_000).toISOString() })
        const stale = makeAgent({ id: 'stale', last_seen: new Date(Date.now() - 5 * 60_000).toISOString() })
        const offline = makeAgent({ id: 'offline', last_seen: null })

        const result = sortAgentsByStatus([offline, online, stale], DEFAULT_THRESHOLDS)
        expect(result.map(a => a.id)).toEqual(['online', 'stale', 'offline'])
    })

    it('sorts alphabetically by hostname within same status group', () => {
        const now = new Date().toISOString()
        const b = makeAgent({ id: 'b', hostname: 'bravo', last_seen: now })
        const a = makeAgent({ id: 'a', hostname: 'alpha', last_seen: now })
        const c = makeAgent({ id: 'c', hostname: 'charlie', last_seen: now })

        const result = sortAgentsByStatus([c, a, b], DEFAULT_THRESHOLDS)
        expect(result.map(a => a.hostname)).toEqual(['alpha', 'bravo', 'charlie'])
    })

    it('falls back to os, arch, then id for identical hostnames', () => {
        const now = new Date().toISOString()
        const a = makeAgent({ id: '2', hostname: 'same', os: 'linux', arch: 'amd64', last_seen: now })
        const b = makeAgent({ id: '1', hostname: 'same', os: 'linux', arch: 'amd64', last_seen: now })

        const result = sortAgentsByStatus([a, b], DEFAULT_THRESHOLDS)
        expect(result.map(a => a.id)).toEqual(['1', '2'])
    })

    it('does not mutate the input array', () => {
        const agents = [
            makeAgent({ id: 'b', hostname: 'bravo '}),
            makeAgent({ id: 'a', hostname: 'alpha' }),
        ]
        const original = [...agents]
        sortAgentsByStatus(agents, DEFAULT_THRESHOLDS)
        expect(agents.map(a => a.id)).toEqual(original.map(a => a.id))
    })
})

describe('formatNetworkRate', () => {
    // The agent normalizes every platform to bits per second before sending.
    it('formats the common ethernet link speeds', () => {
        expect(formatNetworkRate(10_000_000)).toBe('10Mbps')
        expect(formatNetworkRate(100_000_000)).toBe('100Mbps')
        expect(formatNetworkRate(1_000_000_000)).toBe('1Gbps')
        expect(formatNetworkRate(2_500_000_000)).toBe('2.5Gbps')
        expect(formatNetworkRate(5_000_000_000)).toBe('5Gbps')
    })

    it('handles speeds above 10G', () => {
        // These are the ones a hardcoded switch would miss, falling through to
        // a fallback that was wrong by a factor of a million.
        expect(formatNetworkRate(10_000_000_000)).toBe('10Gbps')
        expect(formatNetworkRate(25_000_000_000)).toBe('25Gbps')
        expect(formatNetworkRate(40_000_000_000)).toBe('40Gbps')
        expect(formatNetworkRate(100_000_000_000)).toBe('100Gbps')
    })

    it('drops a trailing zero but keeps a real fraction', () => {
        expect(formatNetworkRate(1_000_000_000)).toBe('1Gbps')
        expect(formatNetworkRate(1_500_000_000)).toBe('1.5Gbps')
    })

    it('returns null when the speed is unknown', () => {
        // Collectors report 0 for a downed link, a virtual interface, or an
        // unreadable sysfs entry.
        expect(formatNetworkRate(0)).toBeNull()
        expect(formatNetworkRate(-1)).toBeNull()
        expect(formatNetworkRate(NaN)).toBeNull()
    })

    it('falls back to bits per second below 1Kbps', () => {
        expect(formatNetworkRate(500)).toBe('500bps')
    })
})

describe('levelColor', () => {
    it('treats every level at or above ERROR as danger', () => {
        for (const level of ['EMERGENCY', 'ALERT', 'CRITICAL', 'ERROR']) {
            expect(levelColor(level)).toBe(themeVars.danger)
        }
    })

    it('separates warning and notice', () => {
        expect(levelColor('WARNING')).toBe(themeVars.warn)
        expect(levelColor('NOTICE')).toBe(themeVars.accent)
    })

    // An unrecognized level must read as "no opinion" rather than borrowing a
    // severity colour, so a value the agent adds later is not shown as benign.
    it('falls back to muted for unknown levels', () => {
        expect(levelColor('INFO')).toBe(themeVars.textMuted)
        expect(levelColor('DEBUG')).toBe(themeVars.textMuted)
        expect(levelColor('')).toBe(themeVars.textMuted)
        expect(levelColor('TRACE')).toBe(themeVars.textMuted)
    })
})

describe('severityOrder', () => {
    // Mirrors levelToPriority on the agent: lower is more severe. Sorting on a
    // LogLevel string instead compares alphabetically, which is what broke the
    // FreeBSD collector.
    it('ranks levels by severity, not alphabetically', () => {
        const levels = ['EMERGENCY', 'ALERT', 'CRITICAL', 'ERROR', 'WARNING', 'NOTICE', 'INFO', 'DEBUG']
        for (let i = 1; i < levels.length; i++) {
            expect(severityOrder(levels[i - 1]!)).toBeLessThan(severityOrder(levels[i]!))
        }
        expect(severityOrder('ERROR')).toBeLessThan(severityOrder('NOTICE'))
        expect(severityOrder('ERROR')).toBeLessThan(severityOrder('WARNING'))
    })

    it('sorts unknown levels last', () => {
        expect(severityOrder('TRACE')).toBe(99)
        expect(severityOrder('')).toBe(99)
    })
})

describe('logEntrySpan', () => {
    const base: LogEntry = {
        timestamp: 1_700_000_000,
        source: 'WinEvent:MsiInstaller',
        level: 'ERROR',
        message: 'Error 1704. An installation is suspended.',
    }

    it('renders nothing when the entry occurred once', () => {
        expect(logEntrySpan(base)).toBe('')
        expect(logEntrySpan({ ...base, count: 0 })).toBe('')
    })

    // count === 1 should never reach the UI: the agent zeroes it so a single
    // entry serializes as it did before folding existed. Treating it as a run
    // would put "1x" on an ordinary row.
    it('treats a stored count of 1 as no run', () => {
        expect(logEntrySpan({ ...base, count: 1, first_seen: 1_600_000_000 })).toBe('')
    })

    it('reports the count and the start of the run', () => {
        const span = logEntrySpan({ ...base, count: 9069, first_seen: 1_600_000_000 })
        expect(span).toMatch(/^9069\u00d7 since /)
        expect(span).toContain(
            new Date(1_600_000_000 * 1000).toLocaleString(undefined, {
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
            }),
        )
    })

    it('falls back to the bare count when first_seen is missing', () => {
        expect(logEntrySpan({ ...base, count: 4 })).toBe('4\u00d7')
    })
})