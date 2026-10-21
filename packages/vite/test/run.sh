#!/bin/sh
# Builds the vibelang compiler and vibelang-runtime from this checkout, links
# the fixtures' node_modules, and runs the vibelang-vite tests.
#
# Usage: sh packages/vite/test/run.sh [node --test options]
#
# Environment:
#   VIBELANG_BINARY        a native vibelang compiler; skips the Go build
#   EFFECT_NODE_MODULES  a node_modules directory that holds effect 4.0.5
#                        (default: experiments/node_modules, else the main
#                        checkout's ~/effectscript/experiments/node_modules)
set -eu

package=$(cd "$(dirname "$0")/.." && pwd)
repo=$(cd "$package/../.." && pwd)
runtime=$repo/packages/runtime

if [ -z "${VIBELANG_BINARY:-}" ]; then
	VIBELANG_BINARY=$repo/packages/compiler/native/vibelang
	echo "build: go build -o packages/compiler/native/vibelang ./cmd/tsc"
	(cd "$repo/typescript/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$VIBELANG_BINARY" ./cmd/tsc)
fi
export VIBELANG_BINARY

effect_modules=${EFFECT_NODE_MODULES:-$repo/experiments/node_modules}
[ -d "$effect_modules/effect" ] || effect_modules=$HOME/effectscript/experiments/node_modules
effect=$(cd "$effect_modules/effect" && pwd -P)

# The runtime's dist/ is not checked in. Build it with this compiler against
# the same effect the fixtures use, so a test loads one copy of effect.
mkdir -p "$runtime/node_modules"
rm -rf "$runtime/node_modules/effect"
ln -s "$effect" "$runtime/node_modules/effect"
echo "build: vibelang -p packages/runtime/tsconfig.json"
"$VIBELANG_BINARY" -p "$runtime/tsconfig.json"

(cd "$package" && bun install --frozen-lockfile --no-cache)

# Fixtures resolve effect and the runtime from test/.fixtures/node_modules. The
# plugin's compile backend, vibelang/project, is the vibelang package beside it.
modules=$package/test/.fixtures/node_modules
mkdir -p "$modules"
rm -rf "$modules/effect" "$modules/vibelang-runtime" "$package/node_modules/vibelang"
ln -s "$effect" "$modules/effect"
ln -s "$runtime" "$modules/vibelang-runtime"
ln -s "$repo/packages/compiler" "$package/node_modules/vibelang"

cd "$package"
exec node --test "$@" test/*.test.mjs
