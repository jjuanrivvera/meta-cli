#!/usr/bin/env bash
set -euo pipefail
manifest="${1:-api-manifest.json}"
test -f "$manifest" || { echo "missing $manifest" >&2; exit 1; }
binary="$(jq -r '.binary' "$manifest")"
test -x "bin/$binary" || make build >/dev/null
fail=0
while IFS=$'\t' read -r path verbs; do
  read -r -a words <<<"$path"
  for verb in $verbs; do
    if "bin/$binary" "${words[@]}" "$verb" --help >/dev/null 2>&1; then
      printf '  ok %s %s\n' "$path" "$verb"
    else
      printf '  missing %s %s\n' "$path" "$verb" >&2
      fail=1
    fi
  done
done < <(jq -r '.resources[] | "\(.path)\t\(.verbs | join(" "))"' "$manifest")
test "$fail" -eq 0 || exit 1
echo "spec surface matches manifest"

