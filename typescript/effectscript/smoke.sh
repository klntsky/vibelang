#!/bin/sh
# Builds tsc and @effectscript/runtime, then compiles and runs every case under
# effectscript/cases with effectscript/harness.mjs, which describes the cases.
#
# Usage: effectscript/smoke.sh [--status] [--verbose] [--jobs N] [case...]
#
#   --status    one line per case: name, owner, and ok, FAIL, pending, or XPASS
#   case        a case name, or a substring that selects every case containing it
#
# Environment:
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../experiments/node_modules, else the main
#                          checkout's ~/effectscript/experiments/node_modules)
#   EFFECTSCRIPT_RUNTIME   the @effectscript/runtime package directory
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
runtime=${EFFECTSCRIPT_RUNTIME:-$root/../packages/runtime}
runtime=$(cd "$runtime" && pwd)

# -trimpath keys the Go build cache by module path instead of directory, so
# every jj workspace shares one set of cache entries. Without DWARF the cache
# entries are smaller; panics still print stack traces.
echo "build: go build -trimpath -gcflags=all=-dwarf=false -o built/local/tsc ./cmd/tsc"
(cd "$root/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$tsc" ./cmd/tsc)

# The runtime's dist/ is not checked in. Build it from source with this tsc, so
# a commit that changes the compiler and the runtime is tested as one. The
# runtime resolves effect from its own node_modules; link the copy the cases
# use when that directory has none, so a program loads one copy of effect.
if [ -f "$runtime/tsconfig.json" ]; then
	if [ ! -e "$runtime/node_modules/effect" ]; then
		mkdir -p "$runtime/node_modules"
		ln -s "$effect_modules/effect" "$runtime/node_modules/effect"
	fi
	echo "build: tsc -p $runtime/tsconfig.json"
	"$tsc" -p "$runtime/tsconfig.json"
fi

EFFECT_NODE_MODULES=$effect_modules EFFECTSCRIPT_RUNTIME=$runtime exec node "$root/effectscript/harness.mjs" "$@"
