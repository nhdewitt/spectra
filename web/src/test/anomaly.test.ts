import { describe, it, expect } from 'vitest'
import {
    median,
    medianAbsoluteDeviation,
    detectOutliers,
    detectNonZero,
    detectThreshold,
    detectCoreImbalance,
    sortAnomalies,
    collect,
    type Sample,
    type TimePoint,
} from '../anomaly'

function series(values: (number | null)[], key = 'v'): Sample[] {
    return values.map((value, i) => ({
        time: new Date(Date.UTC(2026, 0, 1, 0, i)).toISOString(),
        [key]: value,
    }))
}

describe('median', () => {
    it('returns the middle value for odd counts', () => {
        expect(median([1, 2, 3])).toBe(2)
    })

    it('averages the two middle values for even counts', () => {
        expect(median([1, 2, 3, 4])).toBe(2.5)
    })

    it('returns NaN for an empty list', () => {
        expect(median([])).toBeNaN()
    })
})

describe('medianAbsoluteDeviation', () => {
    it('reports zero deviation for a constant series', () => {
        const { median: med, mad } = medianAbsoluteDeviation([5, 5, 5, 5])
        expect(med).toBe(5)
        expect(mad).toBe(0)
    })

    it('is not inflated by a single extreme value', () => {
        // The whole reason for MAD over stddev: the spike must not widen the
        // band that is supposed to catch it.
        const quiet = [2, 2, 2, 2, 2, 2, 2, 2, 2, 2]
        const withSpike = [...quiet, 400]

        const a = medianAbsoluteDeviation(quiet)
        const b = medianAbsoluteDeviation(withSpike)

        expect(b.mad).toBe(a.mad)
        expect(b.median).toBe(2)
    })
})

describe('detectOutliers', () => {
    it('returns null when there are too few samples to have a baseline', () => {
        expect(detectOutliers(series([1, 2, 900]), 'v', 'V')).toBeNull()
    })

    it('flags a spike far above the window baseline', () => {
        const rows = series([...Array(20).fill(2), 90])
        const found = detectOutliers(rows, 'v', 'Read Latency', { floor: 5 })

        expect(found).not.toBeNull()
        expect(found!.count).toBe(1)
        expect(found!.peak).toBe(90)
        expect(found!.label).toBe('Read Latency')
        expect(found!.kind).toBe('outlier')
    })

    it('does not flag low outliers', () => {
        // Unusually fast is good news; reporting it would train the user to
        // ignore the panel.
        const rows = series([...Array(20).fill(50), 1])
        expect(detectOutliers(rows, 'v', 'V', { floor: 0 })).toBeNull()
    })

    it('respects the absolute floor when the baseline is very quiet', () => {
        // A disk idling at 0.1ms makes 0.4ms a 4-sigma event - statistically
        // true, operationally meaningless.
        const rows = series([...Array(20).fill(0.1), 0.4])
        expect(detectOutliers(rows, 'v', 'V', { floor: 5 })).toBeNull()
    })

    it('does not flag a flat series against its own baseline', () => {
        // Regression: the zero-MAD fallback used to drop the median and
        // compare everything to the floor, so a constant series was reported
        // as every sample being an outlier above a cutoff of 0.
        const rows = series(Array(20).fill(50))
        expect(detectOutliers(rows, 'v', 'V', { floor: 0 })).toBeNull()
    })

    it('still flags a spike when the rest of the window is constant', () => {
        // The other half of that fallback: a flat baseline must not suppress a
        // genuine excursion above it.
        const rows = series([...Array(20).fill(50), 300])
        const found = detectOutliers(rows, 'v', 'V', { floor: 0 })

        expect(found).not.toBeNull()
        expect(found!.count).toBe(1)
        expect(found!.peak).toBe(300)
    })

    it('falls back to the floor when the series is perfectly flat', () => {
        // A zero MAD would make any variation infinitely many sigmas out.
        const rows = series([...Array(20).fill(1), 40])
        const found = detectOutliers(rows, 'v', 'V', { floor: 10 })

        expect(found).not.toBeNull()
        expect(found!.peak).toBe(40)
    })

    it('escalates to crit when a large share of the window is affected', () => {
        const rows = series([...Array(20).fill(2), ...Array(6).fill(90)])
        const found = detectOutliers(rows, 'v', 'V', { floor: 5 })

        expect(found!.severity).toBe('crit')
    })

    it('ignores null gaps rather than treating them as zero', () => {
        // Bucketed queries AVG over empty intervals and yield null. Counting
        // those as 0 would manufacture a baseline of zero and flag everything.
        const rows = series([...Array(20).fill(50), null, null])
        expect(detectOutliers(rows, 'v', 'V', { floor: 1 })).toBeNull()
    })

    it('returns null for a key that is absent from every row', () => {
        expect(detectOutliers(series(Array(20).fill(1)), 'missing', 'M')).toBeNull()
    })
})

describe('detectNonZero', () => {
    it('returns null for a clean series', () => {
        expect(detectNonZero(series([0, 0, 0, 0]), 'v', 'RX Errors')).toBeNull()
    })

    it('reports a single blip as warn, not crit', () => {
        const found = detectNonZero(series([0, 0, 3, 0]), 'v', 'RX Errors')

        expect(found!.severity).toBe('warn')
        expect(found!.count).toBe(1)
        expect(found!.peak).toBe(3)
    })

    it('escalates sustained nonzero to crit', () => {
        const found = detectNonZero(series([0, 2, 5, 1]), 'v', 'RX Drops')

        expect(found!.severity).toBe('crit')
        expect(found!.count).toBe(3)
        expect(found!.peak).toBe(5)
    })

    it('records the time of the worst sample, not the first', () => {
        const rows = series([0, 1, 9, 2])
        const found = detectNonZero(rows, 'v', 'RX Errors')

        expect(found!.peakTime).toBe(rows[2].time)
    })

    it('works on a series with only one sample', () => {
        const found = detectNonZero(series([4]), 'v', 'TX Errors')
        expect(found!.count).toBe(1)
    })
})

describe('detectThreshold', () => {
    const limits = { warn: 80, crit: 95 }

    it('returns null when everything is under the warn limit', () => {
        expect(detectThreshold(series([10, 20, 79]), 'v', 'CPU', limits)).toBeNull()
    })

    it('reports warn when the warn limit is crossed but crit is not', () => {
        const found = detectThreshold(series([10, 85, 20]), 'v', 'CPU', limits)

        expect(found!.severity).toBe('warn')
        expect(found!.count).toBe(1)
    })

    it('reports crit and counts only crit breaches once crit is crossed', () => {
        // Two samples are over warn but only one is over crit; the crit count
        // is what gets reported so the detail line is not misleading.
        const found = detectThreshold(series([10, 85, 97]), 'v', 'CPU', limits)

        expect(found!.severity).toBe('crit')
        expect(found!.count).toBe(1)
        expect(found!.peak).toBe(97)
    })

    it('treats a value exactly at the limit as a breach', () => {
        const found = detectThreshold(series([80]), 'v', 'CPU', limits)
        expect(found!.severity).toBe('warn')
    })

    it('works when only a warn limit is configured', () => {
        const found = detectThreshold(series([10, 90]), 'v', 'CPU', { warn: 80 })
        expect(found!.severity).toBe('warn')
    })

    it('returns null for an empty series', () => {
        expect(detectThreshold([], 'v', 'CPU', limits)).toBeNull()
    })
})

describe('detectCoreImbalance', () => {
    function coreRows(rows: number[][]): Sample[] {
        return rows.map((core_usages, i) => ({
            time: new Date(Date.UTC(2026, 0, 1, 0, i)).toISOString(),
            core_usages,
        }))
    }

    it('does not fire on ordinary scheduler churn', () => {
        // Regression: the detector used to measure spread WITHIN a sample, so
        // any multi-core host fired on nearly every sample. Each core here
        // swings wildly but they all average out to roughly the same load.
        const rows = coreRows(
            Array.from({ length: 40 }, (_, i) =>
                Array.from({ length: 8 }, (_, c) => ((i + c) % 8) * 12)
            )
        )
        expect(detectCoreImbalance(rows)).toBeNull()
    })

    it('flags one core pinned while the rest idle', () => {
        const rows = coreRows(Array.from({ length: 40 }, () => [98, 3, 4, 2, 5, 3, 4, 2]))
        const found = detectCoreImbalance(rows)

        expect(found).not.toBeNull()
        expect(found!.peak).toBeCloseTo(98, 5)
        expect(found!.severity).toBe('crit')
        expect(found!.detail).toContain('Core 0')
    })

    it('ignores a merely relative imbalance on an idle box', () => {
        // Hottest core averages 40%: busier than its neighbors, but nothing is
        // saturated and there is no problem to report.
        const rows = coreRows(Array.from({ length: 40 }, () => [40, 2, 2, 2]))
        expect(detectCoreImbalance(rows)).toBeNull()
    })

    it('does not fire when every core is saturated', () => {
        // A fully loaded box is busy, not imbalanced.
        const rows = coreRows(Array.from({ length: 40 }, () => [95, 96, 94, 97]))
        expect(detectCoreImbalance(rows)).toBeNull()
    })

    it('returns null when there are too few samples', () => {
        expect(detectCoreImbalance(coreRows([[100, 1]]))).toBeNull()
    })

    it('averages each core over the samples it actually appears in', () => {
        // A core missing from some rows must not be penalized by dividing its
        // sum across every row.
        const rows: Sample[] = [
            ...coreRows(Array.from({ length: 20 }, () => [99, 2])),
            ...coreRows(Array.from({ length: 20 }, () => [99])),
        ]
        const found = detectCoreImbalance(rows)

        // Core 1 appears in only half the rows, all at 2%. Its mean is 2, not 1.
        expect(found).not.toBeNull()
        expect(found!.detail).toContain('2%')
    })

    it('ignores rows with a missing or single-element core array', () => {
        const rows: TimePoint[] = [
            ...coreRows(Array.from({ length: 20 }, () => [50, 50])),
            { time: '2026-01-01T01:00:00.000Z', core_usages: null },
            { time: '2026-01-01T01:01:00.000Z', core_usages: [50] },
        ]
        expect(detectCoreImbalance(rows)).toBeNull()
    })
})

describe('sortAnomalies and collect', () => {
    it('puts crit before warn, then more frequent first', () => {
        const sorted = sortAnomalies([
            { key: 'a', label: 'A', kind: 'nonzero', severity: 'warn', count: 9, peak: 1, peakTime: '', detail: '' },
            { key: 'b', label: 'B', kind: 'nonzero', severity: 'crit', count: 1, peak: 1, peakTime: '', detail: '' },
            { key: 'c', label: 'C', kind: 'nonzero', severity: 'crit', count: 5, peak: 1, peakTime: '', detail: '' },
        ])

        expect(sorted.map((a) => a.key)).toEqual(['c', 'b', 'a'])
    })

    it('drops nulls and sorts in one pass', () => {
        const found = collect(
            null,
            detectNonZero(series([0, 1, 1]), 'v', 'Drops'),
            null,
            detectThreshold(series([99]), 'v', 'CPU', { warn: 80, crit: 95 })
        )

        expect(found).toHaveLength(2)
        expect(found.every((a) => a !== null)).toBe(true)
    })

    it('returns an empty array when nothing is found', () => {
        expect(collect(null, null)).toEqual([])
    })
})