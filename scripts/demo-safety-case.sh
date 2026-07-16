#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
binary=${INFERLAB_BIN:-"$root/bin/inferlab"}

if [[ ! -x "$binary" ]]; then
  echo "InferLab binary is missing; run 'make build' first." >&2
  exit 1
fi

if [[ $# -gt 1 ]]; then
  echo "usage: scripts/demo-safety-case.sh [output-directory]" >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  work=$1
  mkdir -p "$work"
  if [[ -n "$(find "$work" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    echo "output directory must be empty: $work" >&2
    exit 1
  fi
  cleanup=false
else
  work=$(mktemp -d "${TMPDIR:-/tmp}/inferlab-safety-case.XXXXXX")
  cleanup=true
fi

finish() {
  rm -f "$work/demo-private.pem"
  if [[ "$cleanup" == true ]]; then
    rm -rf "$work"
  fi
}
trap finish EXIT

cp -R "$root/examples/." "$work/"

if "$binary" evaluate "$work/block-gate.json" "$work/block-result.json"; then
  block_code=0
else
  block_code=$?
fi
if [[ $block_code -ne 3 ]]; then
  echo "BLOCK fixture exited $block_code, want 3" >&2
  exit 1
fi

if "$binary" evaluate "$work/missing-evidence-gate.json" "$work/inconclusive-result.json"; then
  inconclusive_code=0
else
  inconclusive_code=$?
fi
if [[ $inconclusive_code -ne 4 ]]; then
  echo "INCONCLUSIVE fixture exited $inconclusive_code, want 4" >&2
  exit 1
fi

"$binary" safety-case assemble "$work/block-safety-case-descriptor.json" "$work/block-safety-case.json"
"$binary" safety-case assemble "$work/inconclusive-safety-case-descriptor.json" "$work/inconclusive-safety-case.json"
"$binary" safety-case keygen "$work/demo-private.pem" "$work/demo-public.pem"

for case_name in block inconclusive; do
  "$binary" safety-case sign \
    "$work/$case_name-safety-case.json" \
    "$work/demo-private.pem" \
    "$work/$case_name-safety-case.sig.json"
  "$binary" safety-case verify \
    "$work/$case_name-safety-case.json" \
    "$work/$case_name-safety-case.sig.json" \
    "$work/demo-public.pem" \
    "$work"
done

rm -f "$work/demo-private.pem"
echo "Verified public-safe BLOCK and INCONCLUSIVE safety cases."
if [[ "$cleanup" == false ]]; then
  echo "Reviewable artifacts: $work"
fi
