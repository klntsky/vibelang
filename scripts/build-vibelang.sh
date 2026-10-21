#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
command -v go >/dev/null 2>&1 || { echo "build-vibelang: Go is required." >&2; exit 1; }
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath"
mkdir -p "$root/packages/compiler/native"
entry=./cmd/tsc
[ ! -d "$root/typescript/tsc/cmd/vibelang" ] || entry=./cmd/vibelang
echo "build-vibelang: CGO_ENABLED=0 go build $entry"
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go build -o "$root/packages/compiler/native/vibelang" "$entry")
echo "build-vibelang: $root/packages/compiler/native/vibelang"
# The dprint plugin for .vibe files runs this binary; its manifest names a zip of it.
"$root/packages/compiler/native/vibelang" --write-dprint-plugin "$root/packages/compiler" >/dev/null
echo "build-vibelang: $root/packages/compiler/dprint-plugin.json"
