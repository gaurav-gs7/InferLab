#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: scripts/check-coverage.sh <coverage-profile> [minimum-percent]" >&2
  exit 2
fi

profile=$1
minimum=${2:-90}
if [[ ! -f "$profile" ]]; then
  echo "Coverage profile does not exist: $profile" >&2
  exit 1
fi

summary=$(awk '
  NR > 1 {
    total += $2
    if ($3 > 0) {
      covered += $2
    }
  }
  END {
    if (total == 0) {
      exit 1
    }
    printf "%.6f %d %d", 100 * covered / total, covered, total
  }
' "$profile")
read -r coverage covered total <<< "$summary"

if ! awk -v minimum="$minimum" 'BEGIN {exit !(minimum ~ /^[0-9]+([.][0-9]+)?$/ && minimum >= 0 && minimum <= 100)}'; then
  echo "Invalid minimum coverage percentage: $minimum" >&2
  exit 2
fi

if ! awk -v coverage="$coverage" -v minimum="$minimum" 'BEGIN {exit !(coverage >= minimum)}'; then
  printf 'Aggregate statement coverage %.2f%% (%d/%d) is below %.2f%%.\n' "$coverage" "$covered" "$total" "$minimum" >&2
  exit 1
fi

printf 'Coverage gate passed: %.2f%% (%d/%d statements) >= %.2f%%.\n' "$coverage" "$covered" "$total" "$minimum"
