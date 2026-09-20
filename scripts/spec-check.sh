#!/usr/bin/env bash
# spec-check.sh — the determinism anchor (cliwright GOAL.md §11).
# The built CLI's command surface must match the spec-derived manifest, so two runs
# on the same API converge on the same surface. Copied into a generated CLI under scripts/.
# Usage: ./scripts/spec-check.sh [api-manifest.json]
set -uo pipefail
MANIFEST="${1:-api-manifest.json}"

[[ -f "$MANIFEST" ]] || { echo "✗ $MANIFEST missing — §11 requires a checked-in spec-derived manifest"; exit 1; }
BIN="$(jq -r '.binary // "__BINARY__"' "$MANIFEST")"
BIN_PATH="bin/$BIN"
make build >/dev/null 2>&1 || { echo "✗ cannot build $BIN for the surface check"; exit 1; }
[[ -x "$BIN_PATH" ]] || { echo "✗ build did not produce $BIN_PATH"; exit 1; }

fail=0
# Every resource AND each of its declared verbs must be a reachable command — not just the
# resource (a resource missing `delete` must fail, or the manifest isn't really enforced).
while IFS=$'\t' read -r r verbs; do
  read -r -a words <<<"$r"
  resolved=$("$BIN_PATH" __surface resolve "${words[@]}" 2>/dev/null) || resolved=""
  if [[ "$resolved" != "$r" ]]; then
    printf "  ✗ resource missing or resolved elsewhere: %s (got %s)\n" "$r" "${resolved:-nothing}"
    fail=1
    continue
  fi
  printf "  ✓ resource: %s\n" "$r"
  for v in $verbs; do
    expected="$r $v"
    resolved=$("$BIN_PATH" __surface resolve "${words[@]}" "$v" 2>/dev/null) || resolved=""
    if [[ "$resolved" == "$expected" ]]; then
      printf "      ✓ %s\n" "$expected"
    else
      printf "      ✗ %s resolved as: %s\n" "$expected" "${resolved:-nothing}"
      fail=1
    fi
  done
done < <(jq -r '.resources[] | "\(.name // .path)\t\(.verbs // [] | join(" "))"' "$MANIFEST")

# Cobra treats `known-group unknown-verb --help` as help for the known group and exits zero.
# Resolve through the command tree and compare exact canonical paths instead of trusting help.
if "$BIN_PATH" __surface resolve pages posts definitely-not-a-verb >/dev/null 2>&1; then
  echo "  ✗ resolver accepted an unknown verb"
  fail=1
else
  echo "  ✓ unknown verbs are rejected by exact command resolution"
fi

if [[ $fail -ne 0 ]]; then echo "✗ CLI surface diverges from $MANIFEST"; exit 1; fi
echo "✓ surface matches $MANIFEST"
