package database

import "embed"

// MigrationsFS embeds the SQL migration files so spectra-server and
// spectra-setup always run migrations matching their own build - there's
// no separate migrations directory to keep in sync with what's deployed.
//
//go:embed migrations
var MigrationsFS embed.FS
