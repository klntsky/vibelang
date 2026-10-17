#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
command -v go >/dev/null 2>&1 || { echo "build-effeck: Go is required." >&2; exit 1; }
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath"
mkdir -p "$root/packages/compiler/native"
entry=./cmd/tsc
[ ! -d "$root/typescript/tsc/cmd/effeck" ] || entry=./cmd/effeck
echo "build-effeck: CGO_ENABLED=0 go build $entry"
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go build -o "$root/packages/compiler/native/effeck" "$entry")
echo "build-effeck: $root/packages/compiler/native/effeck"
