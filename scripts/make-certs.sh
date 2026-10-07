#!/usr/bin/env bash
# Generate a locally-trusted cert for neyialiyorlar.local.
# Rationale: a real trusted chain avoids browser warnings + lets the Flutter
# PWA register a service worker (SW requires a secure context).
set -euo pipefail

command -v mkcert >/dev/null || { echo "install mkcert first (see README)"; exit 1; }

# Check if certs already exist; skip if they do (idempotent)
if [[ -f deploy/nginx/certs/neyialiyorlar.local.pem && -f deploy/nginx/certs/neyialiyorlar.local-key.pem ]]; then
  echo "ℹ️  certs already exist at deploy/nginx/certs/; skipping"
  exit 0
fi

mkcert -install                                  # adds the local root CA to the OS/browser trust store
mkdir -p deploy/nginx/certs
mkcert -cert-file deploy/nginx/certs/neyialiyorlar.local.pem \
       -key-file  deploy/nginx/certs/neyialiyorlar.local-key.pem \
       "neyialiyorlar.local" "*.neyialiyorlar.local" "localhost" 127.0.0.1 ::1
echo "✅ certs written to deploy/nginx/certs/ (gitignored)"
