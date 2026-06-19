-- Server-wide SMTP transport configuration for alert email delivery
--
-- Exactly one row is permitted: the boolean primary key defaults to true
-- and is CHECK-constrained to true, so any insert collides on the PK.
-- Reads/writes use a fixed WHERE id = true. Admin-managed; regular users
-- only set the recipient on individual email channels, never the transport.

CREATE TABLE IF NOT EXISTS smtp_config (
	id					BOOLEAN		PRIMARY KEY DEFAULT TRUE CHECK (id),
	enabled				BOOLEAN		NOT NULL DEFAULT FALSE,
	host				TEXT		NOT NULL DEFAULT '',
	port				INTEGER		NOT NULL DEFAULT 587,
	username			TEXT		NOT NULL DEFAULT '',
	password_encrypted	TEXT		NOT NULL DEFAULT '',
	from_address		TEXT		NOT NULL DEFAULT '',
	tls_mode			TEXT		NOT NULL DEFAULT 'starttls'
							CHECK (tls_mode IN ('starttls', 'implicit', 'none')),
	updated_at			TIMESTAMPTZ	NOT NULL DEFAULT NOW()
);