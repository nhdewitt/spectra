ALTER TABLE metrics_memory
    DROP COLUMN IF EXISTS swap_in_pages,
    DROP COLUMN IF EXISTS swap_out_pages;