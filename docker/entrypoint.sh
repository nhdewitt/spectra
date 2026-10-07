#!/bin/sh
# Orchestrates the disposable stack: migrate + admin (setup), seed a synthetic
# fleet, then run the server in the foreground.
set -e

echo "=> Running setup (migrations + admin)..."
spectra-setup -from /app/setup.yaml -config /app/server.json

echo "==> Clearing any existing seed agents..."
spectra-seed -db "$SEED_DB_URL" -clean || true

echo "===> Seeding ${SEED_N} synthetic agents..."
spectra-seed -db "$SEED_DB_URL" -n "$SEED_N" -confirm

# Setup writes the secret key here; outside Docker the systemd unit loads it.
if [ -f /etc/spectra/spectra.env ]; then
	set -a
	. /etc/spectra/spectra.env
	set +a
fi

echo "====> Starting spectra-server..."
exec spectra-server -config /app/server.json