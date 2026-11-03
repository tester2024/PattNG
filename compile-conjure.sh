#!/bin/bash
set -o errexit
set -o pipefail
set -o nounset
__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "Go: go not found. install the Go toolchain"
  exit 1
fi

CORE_DIR="$__dir/tor-core"
if [[ ! -f "$CORE_DIR/go.mod" ]]; then
  echo "Tor core sources not found at $CORE_DIR"
  exit 1
fi

ABIS="armeabi-v7a arm64-v8a x86_64"

goarch_for () {
  case "$1" in
    armeabi-v7a) echo "arm" ;;
    arm64-v8a)   echo "arm64" ;;
    x86_64)      echo "amd64" ;;
    *) echo "unsupported abi: $1" >&2; exit 1 ;;
  esac
}

goarm_for () {
  case "$1" in
    armeabi-v7a) echo "7" ;;
    *)           echo "" ;;
  esac
}

OUT_DIR="$__dir/libs-conjure"
mkdir -p "$OUT_DIR"

for abi in $ABIS; do
  goarch="$(goarch_for "$abi")"
  goarm="$(goarm_for "$abi")"
  mkdir -p "$OUT_DIR/$abi"

  echo "[conjure] building conjure-client for $abi (linux/$goarch${goarm:+ v$goarm})"
  (
    cd "$CORE_DIR"
    env CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" GOARM="$goarm" \
      go build -trimpath -ldflags "-s -w" -o "$OUT_DIR/$abi/libconjure.so" ./cmd/conjure
  )
done

echo "[conjure] staged:"
for abi in $ABIS; do
  ls -la "$OUT_DIR/$abi/libconjure.so"
done
