DROP TABLE IF EXISTS current_updates;

ALTER TABLE metrics_wifi DROP COLUMN IF EXISTS link_quality;

ALTER TABLE metrics_pi DROP COLUMN IF EXISTS soft_temp_limit;
ALTER TABLE metrics_pi DROP COLUMN IF EXISTS undervoltage_occurred;
ALTER TABLE metrics_pi DROP COLUMN IF EXISTS freq_cap_occurred;
ALTER TABLE metrics_pi DROP COLUMN IF EXISTS throttled_occurred;
ALTER TABLE metrics_pi DROP COLUMN IF EXISTS soft_temp_limit_occurred;