import { useState, useEffect, useRef } from "react";
import { api } from "../api";
import type { OverviewAgent } from "../types";

export interface SparkData {
    cpu: number[];
    mem: number[];
    disk: number[];
}

/**
 * Maintain rolling sparkline history for all agents.
 * 
 * On first load, seeds history from GET /overview/sparklines.
 * On each subsequent poll cycle, appends the latest values from overview data.
 * Offline (last_seen >= 10m) freeze - their sparklines show the last known
 * data without appending zeros.
 */
export function useSparkHistory(
    agents: OverviewAgent[],
    maxPoints = 30
): Map<string, SparkData> {
    const historyRef = useRef<Map<string, SparkData>>(new Map());
    const seededRef = useRef(false);
    const [, setTick] = useState(0);

    // Seed from single API call on first load
    useEffect(() => {
        if (agents.length === 0 || seededRef.current) return;
        seededRef.current = true;

        api
            .sparklines()
            .then((data) => {
                const history = historyRef.current;
                for (const [agentId, spark] of Object.entries(data)) {
                    history.set(agentId, {
                        cpu: spark.cpu.slice(-maxPoints),
                        mem: spark.mem.slice(-maxPoints),
                        disk: spark.disk.slice(-maxPoints),
                    });
                }
                setTick((t) => t + 1);
            })
            .catch(() => {
                // Seed failed - accumulate from polling instead
            });
    }, [agents, maxPoints]);

    // Append new data points
    useEffect(() => {
        if (agents.length === 0) return;

        const history = historyRef.current;
        let changed = false;

        for (const agent of agents) {
            // Skip offline agents - freeze sparkline
            if (!agent.last_seen) continue;
            const ago = (Date.now() - new Date(agent.last_seen).getTime()) / 1000;
            if (ago >= 600) continue;

            let existing = history.get(agent.id);
            if (!existing) {
                existing = { cpu: [], mem: [], disk: [] };
                history.set(agent.id, existing);
            }

            existing.cpu.push(agent.cpu_usage ?? 0);
            existing.mem.push(agent.ram_percent ?? 0);
            existing.disk.push(agent.disk_max_percent ?? 0);
            if (existing.cpu.length > maxPoints) existing.cpu.shift();
            if (existing.mem.length > maxPoints) existing.mem.shift();
            if (existing.disk.length > maxPoints) existing.disk.shift();
            changed = true;
        }

        if (changed) setTick((t) => t + 1);
    }, [agents, maxPoints]);

    return historyRef.current;
}