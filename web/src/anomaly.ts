/**
 * Anomaly detection for metric drill-in views.
 * 
 * Three detectors, because the metrics fall into three different categories:
 * 
 * 	threshold	the operator has declared a limit (cpu, memory, disk, temp)
 * 	nonZero		the metric should be zero at rest (errors, drops)
 * 	outlier		no threshold exists and none would be portable across different
 * 				hardware (io latency, queue depth, per-core spread)
 */

export type AnomalyKind = "threshold" | "nonzero" | "outlier";
export type AnomalySeverity = "warn" | "crit";

export interface Anomaly {
	/** Series key the finding belongs to */
	key: string;
	/** Human label for the series */
	label: string;
	kind: AnomalyKind;
	severity: AnomalySeverity;
	/** How many samples in the window tripped this detector */
	count: number;
	/** Worst value seen */
	peak: number;
	/** ISO timestamp of the worst value */
	peakTime: string;
	/** One line explaining what tripped, formatted for display */
	detail: string;
}

/**
 * Minimal shape every metric row satisfies.
 */
export interface TimePoint {
	time: string;
}

/** Convenience shape for building fixtures and pivoted rows. */
export type Sample = TimePoint & Record<string, unknown>;

/**
 * Read one field off a row whose type does not declare it.
 * 
 * The cast is confined to this one function. Callers pass a key that is not in
 * T's declared shape, so there is no type-safe way to express the lookup. Keeping
 * one audited cast here than an index signature of every metric type.
 */
function readNumber(row: TimePoint, key: string): number | null {
	const raw = (row as unknown as Record<string, unknown>)[key];
	if (raw == null) return null;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : null;
}

/**
 * Numeric values for one key, skipping nulls and non-finite entries.
 * 
 * Gaps are common: a bucketed query averages over an interval with no samples
 * and yields null, and a sensor can drop out mid-window.
 */
function seriesValues(rows: readonly TimePoint[], key: string): { value: number; time: string }[] {
	const out: { value: number; time: string }[] = [];
	for (const row of rows) {
		const n = readNumber(row, key);
		if (n === null) continue;
		out.push({ value: n, time: row.time });
	}
	return out;
}

export function median(sorted: number[]): number {
	if (sorted.length === 0) return NaN;
	const mid = sorted.length >> 1;
	if (sorted.length % 2 === 0) {
		const lower = sorted[mid - 1];
		const upper = sorted[mid];
		if (lower == null || upper == null) return NaN;
		return (lower + upper) / 2;
	}
	return sorted[mid] ?? NaN;
}

/**
 * Median absolute deviation, scaled to be comparable with a standard deviation
 * for normally distributed data.
 * 
 * MAD rather than mean/stddev because a single large spike inflates stddev
 * enough to hide itself.
 */
export function medianAbsoluteDeviation(values: number[]): { median: number; mad: number } {
	if (values.length === 0) return { median: NaN, mad: NaN }
	const sorted = [...values].sort((a, b) => a - b);
	const med = median(sorted);
	const deviations = values.map((v) => Math.abs(v - med)).sort((a, b) => a - b);
	// 1.4826 makes MAD a consistent estimateor of sigma for a normal distribution,
	// so the threshold below reads in familiar sigma units.
	return { median: med, mad: median(deviations) * 1.4826 };
}

export interface OutlierOptions {
	/** Deviations from the median before a point counts (default 4) */
	sigma?: number;
	/** Minimum samples before the detector will report anything (default 12) */
	minSamples?: number;
	/**
	 * Absolute floor: poiunts at or below this are never flagged regardless of how
	 * quiet the baseline is. Without it, a disk idling at 0.1ms latency makes 0.4ms
	 * a "4-sigma event" (statistically true, but noise).
	 */
	floor?: number;
	format?: (v: number) => string;
}

/**
 * Points that sit far from the window's own baseline.
 * 
 * Only detects high outliers. For every metric this is applied to (latency, queue
 * depth, per-core usage), unusually low is good news and reporting it would train
 * the user to ignore the panel.
 */
export function detectOutliers(
	rows: readonly TimePoint[],
	key: string,
	label: string,
	opts: OutlierOptions = {}
): Anomaly | null {
	const { sigma = 4, minSamples = 12, floor = 0, format = (v) => v.toFixed(2) } = opts;

	const points = seriesValues(rows, key);
	if (points.length < minSamples) return null;

	const { median: med, mad } = medianAbsoluteDeviation(points.map((p) => p.value));
	// A zero MAD means over half the window is identical. Any variation would then
	// be infinitely many sigmals out, so fall back to the floor.
	const cutoff = mad > 0 ? med + sigma * mad : Math.max(med, floor);
	if (!Number.isFinite(cutoff)) return null;

	const hits = points.filter((p) => p.value > cutoff && p.value > floor);
	if (hits.length === 0) return null;

	const worst = hits.reduce((a, b) => (b.value > a.value ? b : a));
	return {
		key,
		label,
		kind: "outlier",
		severity: hits.length > points.length * 0.1 ? "crit" : "warn",
		count: hits.length,
		peak: worst.value,
		peakTime: worst.time,
		detail: `${hits.length} of ${points.length} samples above ${format(cutoff)} (baseline ${format(med)}), peaking at ${format(worst.value)}`,
	};
}

/**
 * Any nonzero value in a series that should sit at zero.
 * 
 * Single-sample blips stay "warn", sustained nonzero is "crit".
 */
export function detectNonZero(
	rows: readonly TimePoint[],
	key: string,
	label: string,
	opts: { format?: (v: number) => string; sustainedSamples?: number } = {}
): Anomaly | null {
	const { format = (v) => v.toFixed(2), sustainedSamples = 2 } = opts;

	const points = seriesValues(rows, key);
	const hits = points.filter((p) => p.value > 0);
	if (hits.length === 0) return null;

	const worst = hits.reduce((a, b) => (b.value > a.value ? b : a));
	return {
		key,
		label,
		kind: "nonzero",
		severity: hits.length >= sustainedSamples ? "crit" : "warn",
		count: hits.length,
		peak: worst.value,
		peakTime: worst.time,
		detail:
			hits.length >= sustainedSamples
				? `Nonzero across ${hits.length} samples, peaking at ${format(worst.value)}/s - this should be zero`
				: `A single nonzero sample of ${format(worst.value)}/s`,
	};
}

/**
 * Samples breaching an operator-configured limit.
 * 
 * `crit` is optional so callers with only one limit still behave sensibly.
 */
export function detectThreshold(
	rows: readonly TimePoint[],
	key: string,
	label: string,
	limits: { warn?: number; crit?: number },
	opts: { format?: (v: number) => string } = {}
): Anomaly | null {
	const { format = (v) => `${v.toFixed(1)}%` } = opts;
	const { warn, crit } = limits;

	const points = seriesValues(rows, key);
	if (points.length === 0) return null;

	const critHits = crit != null ? points.filter((p) => p.value >= crit) : [];
	const warnHits = warn != null ? points.filter((p) => p.value >= warn) : [];
	const hits = critHits.length > 0 ? critHits : warnHits;
	if (hits.length === 0) return null;

	const limit = critHits.length > 0 ? crit! : warn!;
	const worst = hits.reduce((a, b) => (b.value > a.value ? b : a));
	return {
		key,
		label,
		kind: "threshold",
		severity: critHits.length > 0 ? "crit" : "warn",
		count: hits.length,
		peak: worst.value,
		peakTime: worst.time,
		detail: `${hits.length} of ${points.length} samples at or above ${format(limit)}, peaking at ${format(worst.value)}`,
	};
}

/**
 * A single core sustained near saturation while the rest of the box is idle.
 * 
 * Measures the spread of per-core means across the window, not the spread within
 * one sample.
 * 
 * Requires the busiest core to be absolutely busy, not just relatively busier. A
 * host averaging 15% with its hottest core at 40% is balanced enough, and reporting
 * it would be a false positive.
 */
export function detectCoreImbalance(
	rows: readonly TimePoint[],
	opts: { minBusy?: number; minSpread?: number; minSamples?: number } = {}
): Anomaly | null {
	const { minBusy = 70, minSpread = 50, minSamples = 12 } = opts;

	// Running sum per core index. Cores can drop out of a sample, so each keeps
	// its own count rather than dividing by the total row count.
	const sums: number[] = [];
	const counts: number[] = [];
	let considered = 0;

	for (const row of rows) {
		const cores = (row as unknown as Record<string, unknown>).core_usages;
		if (!Array.isArray(cores) || cores.length < 2) continue;
		considered++;
		cores.forEach((v, i) => {
			if (typeof v !== "number" || !Number.isFinite(v)) return;
			sums[i] = (sums[i] ?? 0) + v;
			counts[i] = (counts[i] ?? 0) + 1;
		});
	}

	if (considered < minSamples) return null;

	const means: number[] = [];
	for (let i = 0; i < sums.length; i++) {
		const count = counts[i];
		const sum = sums[i];
		if (count !== undefined && count > 0 && sum !== undefined) {
			means.push(sum / count);
		}
	}
	if (means.length < 2) return null;

	const hottest = Math.max(...means);
	const coolest = Math.min(...means);
	const spread = hottest - coolest;

	if (hottest < minBusy || spread < minSpread) return null;

	const hottestCore = means.indexOf(hottest);
	return {
		key: "core_usages",
		label: "Core Imbalance",
		kind: "outlier",
		severity: hottest >= 90 ? "crit" : "warn",
		count: considered,
		peak: hottest,
		peakTime: rows[rows.length - 1]?.time ?? "",
		detail: `Core ${hottestCore} averaged ${hottest.toFixed(0)}% across the window while the quietest core averaged ${coolest.toFixed(0)}% - one core is pinned`,
	};
}

/** Worst-first, then most frequent */
export function sortAnomalies(anomalies: Anomaly[]): Anomaly[] {
	const rank = { crit: 0, warn: 1 };
	return [...anomalies].sort(
		(a, b) => rank[a.severity] - rank[b.severity] || b.count - a.count
	);
}

/** Run a set of detectors and drop the nulls */
export function collect(...found: (Anomaly | null)[]): Anomaly[] {
	return sortAnomalies(found.filter((a): a is Anomaly => a !== null));
}