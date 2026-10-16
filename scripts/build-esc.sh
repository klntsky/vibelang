#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
command -v go >/dev/null 2>&1 || { echo "build-esc: Go is required." >&2; exit 1; }
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath"
mkdir -p "$root/packages/compiler/native"
entry=./cmd/tsc
[ ! -d "$root/typescript/tsc/cmd/esc" ] || entry=./cmd/esc
echo "build-esc: CGO_ENABLED=0 go build $entry"
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go build -o "$root/packages/compiler/native/esc" "$entry")
echo "build-esc: $root/packages/compiler/native/esc"
