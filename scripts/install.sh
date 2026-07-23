#!/usr/bin/env bash
set -euo pipefail

repo="shellus/ags"
asset="ags-linux-amd64"
base="https://github.com/$repo/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

command -v curl >/dev/null 2>&1 || { printf 'curl is required\n' >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { printf 'sha256sum is required\n' >&2; exit 1; }
[[ "$(uname -m)" == "x86_64" ]] || { printf 'only amd64 is supported\n' >&2; exit 1; }

curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
(cd "$tmp" && grep "  $asset\$" checksums.txt | sha256sum -c -)
chmod 0755 "$tmp/$asset"

target_dir="${AGS_INSTALL_DIR:-/usr/local/bin}"
if [[ -w "$target_dir" ]]; then
  install -m 0755 "$tmp/$asset" "$target_dir/ags"
elif command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "$tmp/$asset" "$target_dir/ags"
else
  printf 'cannot write %s and sudo is unavailable\n' "$target_dir" >&2
  exit 1
fi

printf 'Installed AGS to %s/ags\n' "$target_dir"
