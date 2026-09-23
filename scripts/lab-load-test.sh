#!/usr/bin/env sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

duration=${LOAD_DURATION:-10s}

nix develop --command go run ./cmd/labprofiles -count 50
docker compose kill --signal SIGHUP gateway >/dev/null
sleep 1

nix develop --command go run ./cmd/loadtest \
  -profiles 50 \
  -requests 500 \
  -concurrency 100 \
  -minimum-qps 50

nix develop --command go run ./cmd/loadtest \
  -profiles 50 \
  -concurrency 100 \
  -duration "$duration" \
  -rate 50
