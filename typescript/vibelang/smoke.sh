#!/bin/sh
# Builds tsc and vibelang-runtime, then compiles and runs every case under
# vibelang/cases with vibelang/harness.mjs, which describes the cases.
#
# Usage: vibelang/smoke.sh [--status] [--verbose] [--jobs N] [case...]
#
#   --status    one line per case: name, owner, and ok, FAIL, pending, or XPASS
#   case        a case name, or a substring that selects every case containing it
#
# Environment:
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../experiments/node_modules, else the main
#                          checkout's ~/effectscript/experiments/node_modules)
#   VIBELANG_RUNTIME   the vibelang-runtime package directory
#                          (default: ../packages/runtime, built here with tsc)
#
# The suite fails when a case that is not pending fails.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
tsc="$root/built/local/tsc"
effect_modules=${EFFECT_NODE_MODULES:-}
if [ -z "$effect_modules" ]; then
	effect_modules="$root/../experiments/node_modules"
	[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effectscript/experiments/node_modules"
fi
runtime=${VIBELANG_RUNTIME:-$root/../packages/runtime}
runtime=$(cd "$runtime" && pwd)

# -trimpath keys the Go build cache by module path instead of directory, so
# every jj workspace shares one set of cache entries. Without DWARF the cache
# entries are smaller; panics still print stack traces.
echo "build: go build -trimpath -gcflags=all=-dwarf=false -o built/local/tsc ./cmd/tsc"
(cd "$root/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$tsc" ./cmd/tsc)
# The runtime's dist/ is not checked in, and the Go tests compile cases
# against it, so build it before they run. Build it from source with this tsc, so
# a commit that changes the compiler and the runtime is tested as one. The
# runtime resolves effect from its own node_modules; point that at the copy
# the cases use, replacing any other install, so a program and its type
# checking see one copy of effect.
if [ -f "$runtime/tsconfig.json" ]; then
	effect_real=$(cd "$effect_modules/effect" && pwd -P)
	if [ "$(cd "$runtime/node_modules/effect" 2>/dev/null && pwd -P)" != "$effect_real" ]; then
		mkdir -p "$runtime/node_modules"
		rm -rf "$runtime/node_modules/effect"
		ln -s "$effect_real" "$runtime/node_modules/effect"
	fi
	echo "build: tsc -p $runtime/tsconfig.json"
	"$tsc" -p "$runtime/tsconfig.json"
fi

echo "test: go test ./internal/vibelang/"
(cd "$root/tsc" && CGO_ENABLED=0 go test -trimpath -gcflags=all=-dwarf=false ./internal/vibelang/ >/dev/null) ||
	(cd "$root/tsc" && CGO_ENABLED=0 go test -trimpath -gcflags=all=-dwarf=false ./internal/vibelang/; exit 1)

EFFECT_NODE_MODULES=$effect_modules VIBELANG_RUNTIME=$runtime exec node "$root/vibelang/harness.mjs" "$@"
