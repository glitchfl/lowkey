#!/usr/bin/env bash
# runs the real lowkey binary's doctor against the fakeriot program
set -euo pipefail
lowkey=${1:-bin/lowkey}
fakeriot=${2:-bin/fakeriot}
dir=$(mktemp -d)
trap 'kill "$pid" 2>/dev/null || true; rm -rf "$dir"' EXIT

"$fakeriot" -dir "$dir" > "$dir/fakeriot.out" &
pid=$!
for _ in $(seq 50); do
  [ -s "$dir/fakeriot.env" ] && break
  sleep 0.1
done
set -a
. "$dir/fakeriot.env"
set +a
LOWKEY_DIR="$dir" "$lowkey" doctor
