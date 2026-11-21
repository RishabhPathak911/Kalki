#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Immutable upstream revisions and independently recorded SHA-256 hashes.
# --offline verifies available files and never contacts the network.
hash() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d ' ' -f 1; else shasum -a 256 "$1" | cut -d ' ' -f 1; fi; }
while read -r expected relative url; do
  target="data/datasets/$relative"
  if [[ -f "$target" ]]; then
    [[ "$(hash "$target")" == "$expected" ]] || { echo "Checksum mismatch: $relative" >&2; exit 1; }
    continue
  fi
  if [[ "${1:-}" == '--offline' ]]; then echo "Unavailable offline: $relative"; continue; fi
  mkdir -p "$(dirname "$target")"
  temp=$(mktemp "${target}.XXXXXX")
  trap 'rm -f "$temp"' EXIT
  curl --fail --silent --show-error --location "$url" -o "$temp"
  [[ "$(hash "$temp")" == "$expected" ]] || { echo "Checksum mismatch: $relative" >&2; exit 1; }
  mv "$temp" "$target"
  trap - EXIT
  echo "Verified: $relative"
done < scripts/datasets.sha256
