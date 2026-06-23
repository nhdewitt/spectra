-- name: GetSMTPConfig :one
-- Returns the single SMTP config row. Returns no rows if SMTP has never been
-- configured; callers treat pgx.ErrNoRows as "not configured".
SELECT id, enabled, host, port, username, password_encrypted, from_address, tls_mode, updated_at
FROM smtp_config
WHERE id = TRUE;

-- name: UpsertSMTPConfig :one
-- Inserts or updates the single SMTP config row. The fixed boolean PK means the
-- ON CONFLICT path always targets the one existing row.
INSERT INTO smtp_config (
	id, enabled, host, port, username, password_encrypted, from_address, tls_mode, updated_at
) VALUES (
	@id, @enabled, @host, @port, @username, @password_encrypted, @from_address, @tls_mode, NOW()
)
ON CONFLICT (id) DO UPDATE
SET	enabled = EXCLUDED.enabled,
	host = EXCLUDED.host,
	port = EXCLUDED.port,
	username = EXCLUDED.username,
	password_encrypted = EXCLUDED.password_encrypted,
	from_address = EXCLUDED.from_address,
	tls_mode = EXCLUDED.tls_mode,
	updated_at = NOW()
RETURNING id, enabled, host, port, username, password_encrypted, from_address, tls_mode, updated_at;
