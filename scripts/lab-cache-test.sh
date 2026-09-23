#!/usr/bin/env sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

docker compose restart unbound >/dev/null

attempt=0
until curl --fail --silent 'http://127.0.0.1:18080/count?name=shared.test' >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 20 ]; then
    echo "fixture metrics did not become ready" >&2
    exit 1
  fi
  sleep 1
done

before=$(curl --fail --silent 'http://127.0.0.1:18080/count?name=shared.test')

nix develop --command go run ./cmd/dnscheck \
  -server-name p-ana.dns.teendns.test \
  -query shared.test \
  -want-ip 192.0.2.30 \
  -want-max-ttl 300

nix develop --command go run ./cmd/dnscheck \
  -server-name p-bia.dns.teendns.test \
  -query shared.test \
  -want-ip 192.0.2.30 \
  -want-max-ttl 300

after=$(curl --fail --silent 'http://127.0.0.1:18080/count?name=shared.test')
delta=$((after - before))

if [ "$delta" -ne 1 ]; then
  echo "expected one upstream query for two profiles, got $delta" >&2
  exit 1
fi

echo "cache isolation check passed: two profiles, one upstream query"
