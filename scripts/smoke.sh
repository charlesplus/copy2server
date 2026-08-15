#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

run_smoke() {
  local name="$1"
  local port="$2"
  local cmd="$3"
  local dir="/tmp/copy2server-${name}-uploads"
  local cfg="/tmp/copy2server-${name}.json"
  local log="/tmp/copy2server-${name}.log"
  local upload="/tmp/copy2server-${name}.txt"
  local quota_gb="0.00000006"
  local quota_bytes=64

  rm -rf "$dir"
  mkdir -p "$dir"
  printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' > "$dir/old-a.txt"
  printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' > "$dir/old-b.txt"
  touch -t 202001010101 "$dir/old-a.txt"
  touch -t 202001010102 "$dir/old-b.txt"
  printf 'hello' > "$upload"
  printf '{"addr":"127.0.0.1:%s","uploadDir":"%s","maxUploadMB":4,"maxStorageGB":%s,"retentionDays":15,"indexPath":"%s/index.html"}\n' "$port" "$dir" "$quota_gb" "$ROOT" > "$cfg"

  (cd "$ROOT" && CONFIG="$cfg" sh -c "$cmd") > "$log" 2>&1 &
  local pid=$!
  trap 'kill "$pid" 2>/dev/null || true' RETURN

  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${port}/api/files" >/dev/null 2>&1; then
      break
    fi
    sleep 0.25
  done
  curl -fsS "http://127.0.0.1:${port}/api/files" >/dev/null
  (cd "$ROOT" && go run ./cmd/copy2server --server "http://127.0.0.1:${port}" --no-copy "$upload") >/tmp/copy2server-${name}-cli.out

  for _ in $(seq 1 40); do
    local total
    total=$(find "$dir" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')
    if [ "$total" -le "$quota_bytes" ]; then
      break
    fi
    sleep 0.25
  done
  local total
  total=$(find "$dir" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')
  if [ "$total" -gt "$quota_bytes" ]; then
    echo "$name cleanup total $total exceeds quota $quota_bytes" >&2
    cat "$log" >&2
    return 1
  fi
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  trap - RETURN
  echo "$name smoke ok"
}

run_smoke go 18382 'go run .'
run_smoke python 18383 'python3 server.py'
run_smoke node 18384 'node server.js'
