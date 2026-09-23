#!/usr/bin/env sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

nix develop --command go test ./...

attempt=0
until nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query allowed.test \
  -want-ip 192.0.2.10; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 20 ]; then
    echo "gateway did not become ready" >&2
    docker compose logs gateway unbound fixture
    exit 1
  fi
  sleep 1
done

nix develop --command go run ./cmd/dnscheck \
  -server-name p-ana.dns.teendns.test \
  -query blocked.test \
  -want-rcode NXDOMAIN

nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query blocked.test \
  -want-ip 192.0.2.20

nix develop --command go run ./cmd/dnscheck \
  -server-name p-ana.dns.teendns.test \
  -query school.blocked.test \
  -want-ip 192.0.2.40

echo "laboratory checks passed"
