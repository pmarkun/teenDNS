#!/usr/bin/env sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

mkdir -p .local/certs
nix develop --command go run ./cmd/certgen -out .local/certs
docker compose up --build --detach
