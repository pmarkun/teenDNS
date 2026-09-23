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

nix develop --command go run ./cmd/dnscheck \
  -server-name p-ana.dns.teendns.test \
  -query alias.test \
  -want-rcode NXDOMAIN

nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query alias.test \
  -want-ip 192.0.2.20

if nix develop --command go run ./cmd/dnscheck \
  -server-name p-unknown.dns.teendns.test \
  -query allowed.test; then
  echo "unknown profile endpoint was unexpectedly accepted" >&2
  exit 1
fi

./scripts/lab-cache-test.sh
./scripts/lab-reload-test.sh

echo "laboratory checks passed"
