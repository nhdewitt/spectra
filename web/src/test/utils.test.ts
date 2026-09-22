import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    agentStatus,
    formatBytes,
    pivotByGroup,
    rankGroupsByLatest,
    roundToInterval,
    formatLogTime,
    formatUptime,
    statusColor,
    severityColor,
    sortAgentsBySeverity,
    sortAgentsByStatus,
    formatNetworkRate,
    levelColor,
    severityOrder,
    logEntrySpan,
    logRangeStart,
    LOG_RANGES
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
    // severity color, so a value the agent adds later is not shown as benign.
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
        expect(span).toContain(formatLogTime(1_600_000_000))
    })

    it('falls back to the bare count when first_seen is missing', () => {
        expect(logEntrySpan({ ...base, count: 4 })).toBe('4\u00d7')
    })
})

describe('logRangeStart', () => {
    const now = new Date('2026-09-14T12:00:00.000Z')

    // Zero is the default the panel opens on, and it has to mean "send no
    // bound at all" - a Date here would silently filter a screen that has
    // always shown everything the agent had.
    it('returns undefined for an unbounded range', () => {
        expect(logRangeStart(0, now)).toBeUndefined()
        expect(logRangeStart(-1, now)).toBeUndefined()
    })

    it('subtracts the range from now', () => {
        expect(logRangeStart(1, now)?.toISOString()).toBe('2026-09-14T11:00:00.000Z')
        expect(logRangeStart(24, now)?.toISOString()).toBe('2026-09-13T12:00:00.000Z')
        expect(logRangeStart(168, now)?.toISOString()).toBe('2026-09-07T12:00:00.000Z')
    })

    it('offers exactly one unbounded option, listed first', () => {
        expect(LOG_RANGES.filter((r) => r.hours <= 0)).toHaveLength(1)
        expect(LOG_RANGES[0]!.hours).toBe(0)
    })
})
// "Since boot" on a host with a year of uptime returns entries from several
// calendar years, and a bare "Aug 3" gives no indication which.
describe('formatLogTime', () => {
    const now = new Date('2026-09-18T12:00:00Z')

    it('omits the year for an entry from the current year', () => {
        const ts = new Date('2026-08-03T19:56:00Z').getTime() / 1000
        expect(formatLogTime(ts, now)).not.toContain('2026')
    })

    it('includes the year for an entry from an earlier year', () => {
        const ts = new Date('2025-08-03T19:56:00Z').getTime() / 1000
        expect(formatLogTime(ts, now)).toContain('2025')
    })

    // Clocks skew and agents can be ahead of the server.
    it('includes the year for an entry from a later year', () => {
        const ts = new Date('2027-01-02T00:00:00Z').getTime() / 1000
        expect(formatLogTime(ts, now)).toContain('2027')
    })

    it('keeps the time of day in every case', () => {
        const ts = new Date('2025-08-03T19:56:07Z').getTime() / 1000
        expect(formatLogTime(ts, now)).toMatch(/\d{1,2}:\d{2}:\d{2}/)
    })
})

// The stale branch returned "offline", so a host between the two thresholds
// was reported as fully down and the "stale" status was unreachable.
describe('agentStatus liveness thresholds', () => {
    const t = { ...DEFAULT_THRESHOLDS, stale_seconds: 120, offline_seconds: 300 }

    function agentSeen(secondsAgo: number) {
        return {
            last_seen: new Date(Date.now() - secondsAgo * 1000).toISOString(),
            cpu_usage: 0,
            ram_percent: 0,
            disk_max_percent: 0,
            max_temp: 0,
        } as unknown as OverviewAgent
    }

    it('is online inside the stale threshold', () => {
        expect(agentStatus(agentSeen(30), t).status).toBe('online')
    })

    it('is stale between the thresholds', () => {
        expect(agentStatus(agentSeen(200), t).status).toBe('stale')
    })

    it('is offline past the offline threshold', () => {
        expect(agentStatus(agentSeen(400), t).status).toBe('offline')
    })

    it('is offline with no heartbeat at all', () => {
        expect(agentStatus({ last_seen: null } as unknown as OverviewAgent, t).status).toBe('offline')
    })
})

// --- Pivot and ranking ---
// These were only exercised through the MetricsTab DiskPanel tests, and the
// forward-fill had no assertion of its own. The disk tooltip now looks samples
// up by the bucket roundToInterval produces, so the rounding is load-bearing.

interface Sample {
    time: string
    mount: string
    pct: number | null
}

function sample(time: string, mount: string, pct: number | null = 50): Sample {
    return { time, mount, pct }
}

const mountOf = (s: Sample) => s.mount
const pctOf = (s: Sample) => s.pct

describe('roundToInterval', () => {
    it('rounds to the nearest bucket, not down', () => {
        expect(roundToInterval('2026-01-01T00:00:07.000Z', 5000))
            .toBe('2026-01-01T00:00:05.000Z')
        expect(roundToInterval('2026-01-01T00:00:08.000Z', 5000))
            .toBe('2026-01-01T00:00:10.000Z')
    })

    it('leaves a value already on a boundary alone', () => {
        expect(roundToInterval('2026-01-01T00:00:05.000Z', 5000))
            .toBe('2026-01-01T00:00:05.000Z')
    })

    it('collapses samples within one bucket to the same key', () => {
        const a = roundToInterval('2026-01-01T00:00:04.100Z', 5000)
        const b = roundToInterval('2026-01-01T00:00:06.900Z', 5000)
        expect(a).toBe(b)
    })
})

describe('pivotByGroup', () => {
    it('returns nothing for no rows', () => {
        expect(pivotByGroup([], mountOf, pctOf, ['/'])).toEqual([])
    })

    it('merges samples in the same bucket into one row', () => {
        const out = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 50),
                sample('2026-01-01T00:00:01.000Z', '/var', 70),
            ],
            mountOf, pctOf, ['/', '/var'],
        )

        expect(out).toHaveLength(1)
        expect(out[0]!['/']).toBe(50)
        expect(out[0]!['/var']).toBe(70)
    })

    it('keeps separate buckets separate', () => {
        const out = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 50),
                sample('2026-01-01T00:00:30.000Z', '/', 60),
            ],
            mountOf, pctOf, ['/'],
        )

        expect(out).toHaveLength(2)
        expect(out[0]!['/']).toBe(50)
        expect(out[1]!['/']).toBe(60)
    })

    it('sets _ts from the bucket time', () => {
        const out = pivotByGroup(
            [sample('2026-01-01T00:00:00.000Z', '/', 50)],
            mountOf, pctOf, ['/'],
        )

        expect(out[0]!._ts).toBe(Date.parse(out[0]!.time as string))
    })

    it('honors a custom interval', () => {
        const out = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 50),
                sample('2026-01-01T00:00:20.000Z', '/', 60),
            ],
            mountOf, pctOf, ['/'], 60_000,
        )

        expect(out).toHaveLength(1)
    })

    it('skips rows with no group', () => {
        const out = pivotByGroup(
            [
                { time: '2026-01-01T00:00:00.000Z', mount: '', pct: 50 },
                sample('2026-01-01T00:00:00.000Z', '/', 60),
            ],
            mountOf, pctOf, ['/'],
        )

        expect(out).toHaveLength(1)
        expect(out[0]!['/']).toBe(60)
    })

    it('a later sample for the same group in one bucket wins', () => {
        const out = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 50),
                sample('2026-01-01T00:00:02.000Z', '/', 99),
            ],
            mountOf, pctOf, ['/'],
        )

        expect(out).toHaveLength(1)
        expect(out[0]!['/']).toBe(99)
    })

    describe('forward-fill', () => {
        it('carries the last value into a bucket the group is missing from', () => {
            const out = pivotByGroup(
                [
                    sample('2026-01-01T00:00:00.000Z', '/', 50),
                    sample('2026-01-01T00:00:00.000Z', '/var', 70),
                    sample('2026-01-01T00:00:30.000Z', '/', 55),
                ],
                mountOf, pctOf, ['/', '/var'],
            )

            expect(out).toHaveLength(2)
            expect(out[1]!['/']).toBe(55)
            expect(out[1]!['/var']).toBe(70)
        })

        it('does not back-fill before a group first appears', () => {
            const out = pivotByGroup(
                [
                    sample('2026-01-01T00:00:00.000Z', '/', 50),
                    sample('2026-01-01T00:00:30.000Z', '/var', 70),
                ],
                mountOf, pctOf, ['/', '/var'],
            )

            expect(out[0]!['/var']).toBeUndefined()
            expect(out[1]!['/var']).toBe(70)
        })

        it('carries a value across several gaps', () => {
            const out = pivotByGroup(
                [
                    sample('2026-01-01T00:00:00.000Z', '/', 50),
                    sample('2026-01-01T00:00:30.000Z', '/var', 70),
                    sample('2026-01-01T00:01:00.000Z', '/var', 71),
                ],
                mountOf, pctOf, ['/', '/var'],
            )

            expect(out[1]!['/']).toBe(50)
            expect(out[2]!['/']).toBe(50)
        })

        // A null reading is filled over rather than left as a gap, so a
        // failed collection shows the previous value instead of a break.
        it('fills over an explicit null', () => {
            const out = pivotByGroup(
                [
                    sample('2026-01-01T00:00:00.000Z', '/', 50),
                    sample('2026-01-01T00:00:30.000Z', '/', null),
                ],
                mountOf, pctOf, ['/'],
            )

            expect(out[1]!['/']).toBe(50)
        })

        // Only groups named in `groups` are filled. One present in the rows
        // but absent from that list is recorded where it appears and nowhere
        // else -- which is what the top-N ranking relies on.
        it('ignores groups outside the requested list', () => {
            const out = pivotByGroup(
                [
                    sample('2026-01-01T00:00:00.000Z', '/home', 90),
                    sample('2026-01-01T00:00:30.000Z', '/', 50),
                ],
                mountOf, pctOf, ['/'],
            )

            expect(out[0]!['/home']).toBe(90)
            expect(out[1]!['/home']).toBeUndefined()
        })
    })

    // Rows are keyed by insertion order, not sorted, so out-of-order input
    // produces out-of-order output.
    it('preserves input order rather than sorting by time', () => {
        const out = pivotByGroup(
            [
                sample('2026-01-01T00:01:00.000Z', '/', 60),
                sample('2026-01-01T00:00:00.000Z', '/', 50),
            ],
            mountOf, pctOf, ['/'],
        )

        expect(out[0]!['/']).toBe(60)
        expect(out[1]!['/']).toBe(50)
    })
})

describe('rankGroupsByLatest', () => {
    it('orders by the most recent value, highest first', () => {
        const pivoted = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 10),
                sample('2026-01-01T00:00:00.000Z', '/var', 80),
                sample('2026-01-01T00:00:00.000Z', '/home', 45),
            ],
            mountOf, pctOf, ['/', '/var', '/home'],
        )

        expect(rankGroupsByLatest(pivoted, ['/', '/var', '/home']))
            .toEqual(['/var', '/home', '/'])
    })

    // The latest value, not the highest ever seen.
    it('uses the last value rather than the peak', () => {
        const pivoted = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 99),
                sample('2026-01-01T00:00:00.000Z', '/var', 10),
                sample('2026-01-01T00:00:30.000Z', '/', 5),
                sample('2026-01-01T00:00:30.000Z', '/var', 20),
            ],
            mountOf, pctOf, ['/', '/var'],
        )

        expect(rankGroupsByLatest(pivoted, ['/', '/var'])).toEqual(['/var', '/'])
    })

    it('sorts a group with no numeric value last', () => {
        const pivoted = pivotByGroup(
            [sample('2026-01-01T00:00:00.000Z', '/', 50)],
            mountOf, pctOf, ['/'],
        )

        expect(rankGroupsByLatest(pivoted, ['/', '/never-seen']))
            .toEqual(['/', '/never-seen'])
    })

    it('does not mutate the group list it was given', () => {
        const groups = ['/', '/var']
        const pivoted = pivotByGroup(
            [
                sample('2026-01-01T00:00:00.000Z', '/', 10),
                sample('2026-01-01T00:00:00.000Z', '/var', 80),
            ],
            mountOf, pctOf, groups,
        )

        rankGroupsByLatest(pivoted, groups)

        expect(groups).toEqual(['/', '/var'])
    })

    it('returns an empty list for no groups', () => {
        expect(rankGroupsByLatest([], [])).toEqual([])
    })

    it('handles an empty pivot', () => {
        expect(rankGroupsByLatest([], ['/', '/var'])).toEqual(['/', '/var'])
    })
})