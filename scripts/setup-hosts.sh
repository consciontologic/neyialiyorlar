#!/usr/bin/env bash
# Map neyialiyorlar.local -> loopback. Idempotent; backs up /etc/hosts first.
set -euo pipefail

HOST="neyialiyorlar.local"
IP="127.0.0.1"
HF="/etc/hosts"

if grep -qE "[[:space:]]${HOST}(\$|[[:space:]])" "$HF"; then
  echo "ℹ️  ${HOST} already mapped"
  exit 0
fi

sudo cp "$HF" "${HF}.neyialiyorlar.bak.$(date +%s)"   # reversible backup
printf '%s\t%s\n' "$IP" "$HOST" | sudo tee -a "$HF" >/dev/null
echo "✅ added ${IP} ${HOST} to ${HF}"
