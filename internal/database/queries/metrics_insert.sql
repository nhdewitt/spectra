-- name: InsertCPU :exec
INSERT INTO metrics_cpu (time, agent_id, usage, core_usages, load_1m, load_5m, load_15m, iowait)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertMemory :exec
INSERT INTO metrics_memory (time, agent_id, ram_total, ram_used, ram_available, ram_percent, swap_total, swap_used, swap_percent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertDisk :exec
INSERT INTO metrics_disk (time, agent_id, device, mountpoint, filesystem, disk_type, total_bytes, used_bytes, free_bytes, used_percent, inodes_total, inodes_used, inodes_percent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: InsertDiskIO :exec
INSERT INTO metrics_disk_io (time, agent_id, device, read_bytes, write_bytes, read_ops, write_ops, read_latency, write_latency, io_in_progress)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: InsertNetwork :exec
INSERT INTO metrics_network (time, agent_id, interface, mac, mtu, speed, rx_bytes, rx_packets, rx_errors, rx_drops, tx_bytes, tx_packets, tx_errors, tx_drops)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: InsertTemperature :exec
INSERT INTO metrics_temperature (time, agent_id, sensor, temperature, max_temp)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertWifi :exec
INSERT INTO metrics_wifi (time, agent_id, interface, ssid, bssid, frequency_mhz, signal_dbm, noise_dbm, bitrate_mbps)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertSystem :exec
INSERT INTO metrics_system (time, agent_id, uptime, process_count, user_count, boot_time)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertContainer :exec
INSERT INTO metrics_container (time, agent_id, container_id, name, image, state, source, kind, cpu_percent, cpu_cores, memory_bytes, memory_limit, net_rx_bytes, net_tx_bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: InsertPi :exec
INSERT INTO metrics_pi (time, agent_id, metric_type, arm_freq_hz, core_freq_hz, gpu_freq_hz, core_volts, sdram_c_volts, sdram_i_volts, sdram_p_volts, soft_temp_limit, throttled, under_voltage, freq_capped, undervoltage_occurred, freq_cap_occurred, throttled_occurred, soft_temp_limit_occurred, gpu_mem_total, gpu_mem_used, gpu_temp)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21);