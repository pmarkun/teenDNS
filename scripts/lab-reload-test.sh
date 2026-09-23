#!/usr/bin/env sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query dynamic.test \
  -want-ip 192.0.2.50

nix develop --command go run ./cmd/labpolicy \
  -hostname p-bia.dns.teendns.test \
  -domain dynamic.test \
  -action block

docker compose kill --signal SIGHUP gateway >/dev/null
sleep 1

nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query dynamic.test \
  -want-rcode NXDOMAIN

echo "policy reload check passed"
