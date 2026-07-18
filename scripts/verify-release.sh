#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
audit="$root/dist/release-audit"
fuzztime=${FUZZTIME:-10s}
export GOCACHE=${GOCACHE:-"$root/.gocache"}
export GOMODCACHE=${GOMODCACHE:-"$root/.gomodcache"}

rm -rf "$audit"
mkdir -p "$audit/bin" "$audit/safety-gate"
cd "$root"

git diff --check
go mod verify
if [[ -n "$(gofmt -l .)" ]]; then
  echo "Go source is not formatted." >&2
  exit 1
fi
go vet ./...
go test -coverprofile="$audit/coverage.out" -covermode=atomic ./...
bash scripts/check-coverage.sh "$audit/coverage.out" 90
coverage=$(go tool cover -func="$audit/coverage.out" | awk '/^total:/ {print $3}')
go test -race ./...
go test -shuffle=on -count=3 ./...
FUZZTIME="$fuzztime" make fuzz

targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64)
for target in "${targets[@]}"; do
  goos=${target%/*}
  goarch=${target#*/}
  suffix=""
  if [[ "$goos" == windows ]]; then
    suffix=".exe"
  fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -o "$audit/bin/inferlab-$goos-$goarch$suffix" ./cmd/inferlab
done

make build
bash scripts/demo-safety-case.sh "$audit/safety-gate"
./bin/inferlab change validate examples/qwen-vllm-batching-change.json
./bin/inferlab runtime validate examples/runtime-signature-l4-vllm.json
./bin/inferlab evidence validate examples/guidellm-observed-evidence.json
./bin/inferlab adapter normalize guidellm-fixture-v1 examples/guidellm-adapter-input.json > "$audit/normalized-report.json"
./bin/inferlab adapter validate "$audit/normalized-report.json"

echo "Release audit passed: coverage=$coverage fuzztime=$fuzztime targets=${targets[*]}"
