import {
	collect,
	detectCoreImbalance,
	detectNonZero,
	detectOutliers,
	detectThreshold,
	sortAnomalies,
	type Anomaly,
	type TimePoint,
} from "./anomaly";
import type { Thresholds } from "./types";

const rate = (v: number) => v.toFixed(2);
const ms = (v: number) => `${v.toFixed(1)}ms`;
const pct = (v: number) => `${v.toFixed(1)}%`;

export function cpuAnomalies(data: readonly TimePoint[], t: Thresholds): Anomaly[] {
	return collect(
		detectThreshold(data, "usage", "CPU Usage", { warn: t.cpu_warn, crit: t.cpu_crit }),
		// No configured limit for iowait (high means the CPU is blocked on storage,
		// so it's a disk symptom)
		detectOutliers(data, "iowait", "IO Wait", { floor: 10, format: pct }),
		detectCoreImbalance(data)
	);
}

export function loadAnomalies(data: readonly TimePoint[], cores: number): Anomaly[] {
	// Load above core count means work is queuing, so it is the floor.
	return collect(
		detectOutliers(data, "load_1m", "Load 1m", { floor: cores, format: (v) => v.toFixed(2) }),
		detectOutliers(data, "load_15m", "Load 15m", { floor: cores, format: (v) => v.toFixed(2) })
	);
}

export function memoryAnomalies(data: readonly TimePoint[], t: Thresholds): Anomaly[] {
	return collect(
		detectThreshold(data, "ram_percent", "RAM", { warn: t.mem_warn, crit: t.mem_crit }),
		detectOutliers(data, "swap_in_pages", "Swap In", {
			floor: 100,
			format: (v) => `${v.toFixed(0)} pages/s`,
		})
	);
}

export function diskAnomalies(rows: readonly TimePoint[], t: Thresholds): Anomaly[] {
	return collect(
		detectThreshold(rows, "used_percent", "Disk Usage", { warn: t.disk_warn, crit: t.disk_crit }),
		detectThreshold(rows, "inodes_percent", "Inodes", { warn: t.disk_warn, crit: t.disk_crit })
	);
}

export function diskIOAnomalies(rows: readonly TimePoint[]): Anomaly[] {
	// Compare each device to its own baseline
	return collect(
		detectOutliers(rows, "read_latency_ms", "Read Latency", { floor: 5, format: ms }),
		detectOutliers(rows, "write_latency_ms", "Write Latency", { floor: 5, format: ms }),
		detectOutliers(rows, "io_in_progress", "Queue Depth", { floor: 2, format: (v) => v.toFixed(1) })
	);
}

export function networkAnomalies(rows: readonly TimePoint[]): Anomaly[] {
	// Rates, not counters. Nonzero means packets are being lost right now.
	return collect(
		detectNonZero(rows, "rx_errors", "RX Errors", { format: rate }),
		detectNonZero(rows, "tx_errors", "TX Errors", { format: rate }),
		detectNonZero(rows, "rx_drops", "RX Drops", { format: rate }),
		detectNonZero(rows, "tx_drops", "TX Drops", { format: rate })
	);
}

export function temperatureAnomalies(
	pivoted: readonly TimePoint[],
	sensors: readonly string[],
	t: Thresholds
): Anomaly[] {
	return collect(
		...sensors.map((s) =>
			detectThreshold(pivoted, s, s, { warn: t.temp_warn, crit: t.temp_crit }, {
				format: (v) => `${v.toFixed(1)}°C`,
			})
		)
	);
}

/** Worst severity across a set (or null when clean). */
export function worstSeverity(anomalies: readonly Anomaly[]): "crit" | "warn" | null {
	if (anomalies.some((a) => a.severity === "crit")) return "crit";
	if (anomalies.length > 0) return "warn";
	return null;
}

/**
 * Run a per-family detector across every group in a multi-entity series and
 * prefix each finding with its group.
 */
export function byGroup<T extends TimePoint>(
	rows: readonly T[],
	groupKey: keyof T & string,
	detect: (subset: readonly TimePoint[]) => Anomaly[]
): Anomaly[] {
	const groups = new Map<string, T[]>();
	for (const row of rows) {
		const name = String(row[groupKey] ?? "");
		if (!name) continue;
		const bucket = groups.get(name);
		if (bucket) bucket.push(row);
		else groups.set(name, [row]);
	}

	const out: Anomaly[] = [];
	for (const [name, subset] of groups) {
		for (const found of detect(subset)) {
			out.push({ ...found, key: `${name}:${found.key}`, label: `${name} ${found.label}` });
		}
	}

	return sortAnomalies(out);
}