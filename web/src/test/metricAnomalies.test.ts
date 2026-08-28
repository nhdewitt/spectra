import { describe, it, expect } from 'vitest'
import { byGroup, worstSeverity, diskAnomalies, networkAnomalies } from '../metricAnomalies'
import type { Anomaly, TimePoint } from '../anomaly'
import type { Thresholds } from '../types'

const THRESHOLDS: Thresholds = {
    cpu_warn: 80, cpu_crit: 95,
    mem_warn: 80, mem_crit: 95,
    disk_warn: 80, disk_crit: 95,
    temp_warn: 70, temp_crit: 85,
    stale_seconds: 120, offline_seconds: 600,
}

function at(i: number): string {
    return new Date(Date.UTC(2026, 0, 1, 0, i)).toISOString()
}

describe('worstSeverity', () => {
    const warn: Anomaly = { key: 'a', label: 'A', kind: 'nonzero', severity: 'warn', count: 1, peak: 1, peakTime: '', detail: '' }
    const crit: Anomaly = { ...warn, key: 'b', severity: 'crit' }

    it('returns null when clean', () => {
        expect(worstSeverity([])).toBeNull()
    })

    it('returns warn when only warnings are present', () => {
        expect(worstSeverity([warn, warn])).toBe('warn')
    })

    it('returns crit when any finding is critical', () => {
        expect(worstSeverity([warn, crit])).toBe('crit')
    })
})

describe('byGroup', () => {
    it('finds a problem on a mount that is not the selected one', () => {
        // The reason this exists: the Disk panel shows one mount at a time, so
        // a full /var must still reach the summary badge when / is selected.
        const rows: TimePoint[] = [
            ...Array.from({ length: 3 }, (_, i) => ({ time: at(i), mountpoint: '/', used_percent: 20, inodes_percent: 5 })),
            ...Array.from({ length: 3 }, (_, i) => ({ time: at(i), mountpoint: '/var', used_percent: 99, inodes_percent: 5 })),
        ]

        const found = byGroup(rows, 'mountpoint', (g) => diskAnomalies(g, THRESHOLDS))

        expect(found).toHaveLength(1)
        expect(found[0].label).toBe('/var Disk Usage')
        expect(found[0].severity).toBe('crit')
    })

    it('namespaces keys by group so two mounts do not collide', () => {
        const rows: TimePoint[] = [
            { time: at(0), mountpoint: '/', used_percent: 99, inodes_percent: 0 },
            { time: at(0), mountpoint: '/var', used_percent: 99, inodes_percent: 0 },
        ]

        const found = byGroup(rows, 'mountpoint', (g) => diskAnomalies(g, THRESHOLDS))
        const keys = found.map((f) => f.key)

        expect(new Set(keys).size).toBe(keys.length)
        expect(keys).toContain('/:used_percent')
        expect(keys).toContain('/var:used_percent')
    })

    it('returns nothing when every group is clean', () => {
        const rows: TimePoint[] = [
            { time: at(0), mountpoint: '/', used_percent: 10, inodes_percent: 2 },
            { time: at(1), mountpoint: '/var', used_percent: 12, inodes_percent: 3 },
        ]
        expect(byGroup(rows, 'mountpoint', (g) => diskAnomalies(g, THRESHOLDS))).toEqual([])
    })

    it('skips rows with a missing group value', () => {
        const rows: TimePoint[] = [{ time: at(0), mountpoint: '', used_percent: 99 }]
        expect(byGroup(rows, 'mountpoint', (g) => diskAnomalies(g, THRESHOLDS))).toEqual([])
    })

    it('does not let one interface mask another', () => {
        // eth0 clean, eth1 dropping. A whole-series detector would average them
        // together; grouping keeps eth1 visible.
        const rows: TimePoint[] = [
            ...Array.from({ length: 4 }, (_, i) => ({ time: at(i), interface: 'eth0', rx_errors: 0, tx_errors: 0, rx_drops: 0, tx_drops: 0 })),
            ...Array.from({ length: 4 }, (_, i) => ({ time: at(i), interface: 'eth1', rx_errors: 0, tx_errors: 0, rx_drops: 3, tx_drops: 0 })),
        ]

        const found = byGroup(rows, 'interface', networkAnomalies)

        expect(found).toHaveLength(1)
        expect(found[0].label).toBe('eth1 RX Drops')
    })

    it('sorts crit findings ahead of warn across groups', () => {
        const rows: TimePoint[] = [
            { time: at(0), interface: 'eth0', rx_errors: 0, tx_errors: 0, rx_drops: 1, tx_drops: 0 },
            ...Array.from({ length: 3 }, (_, i) => ({ time: at(i), interface: 'eth1', rx_errors: 5, tx_errors: 0, rx_drops: 0, tx_drops: 0 })),
        ]

        const found = byGroup(rows, 'interface', networkAnomalies)

        expect(found[0].severity).toBe('crit')
        expect(found[0].label).toBe('eth1 RX Errors')
    })
})