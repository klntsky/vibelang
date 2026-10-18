#!/bin/sh
# Builds the effeck compiler and effeck-runtime from this checkout, then
# bundles test/fixture with webpack, Rspack, Rollup, Rolldown, esbuild, Bun,
# and Vite through effeck-unplugin and checks each bundle.
#
# Usage: sh packages/unplugin/test/run.sh [node --test options]
#
# Environment:
#   EFFECK_BINARY        a native effeck compiler; skips the Go build
#   EFFECT_NODE_MODULES  a node_modules directory that holds effect 4.0.5
#                        (default: experiments/node_modules, else the main
#                        checkout's ~/effectscript/experiments/node_modules)
set -eu

package=$(cd "$(dirname "$0")/.." && pwd)
repo=$(cd "$package/../.." && pwd)
runtime=$repo/packages/runtime

if [ -z "${EFFECK_BINARY:-}" ]; then
	EFFECK_BINARY=$repo/packages/compiler/native/effeck
	echo "build: go build -o packages/compiler/native/effeck ./cmd/tsc"
	(cd "$repo/typescript/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$EFFECK_BINARY" ./cmd/tsc)
fi
export EFFECK_BINARY

effect_modules=${EFFECT_NODE_MODULES:-$repo/experiments/node_modules}
[ -d "$effect_modules/effect" ] || effect_modules=$HOME/effectscript/experiments/node_modules
effect=$(cd "$effect_modules/effect" && pwd -P)

# The runtime's dist/ is not checked in. Build it with this compiler against
# the same effect the fixtures use, so a bundle holds one copy of effect.
mkdir -p "$runtime/node_modules"
rm -rf "$runtime/node_modules/effect"
ln -s "$effect" "$runtime/node_modules/effect"
echo "build: effeck -p packages/runtime/tsconfig.json"
"$EFFECK_BINARY" -p "$runtime/tsconfig.json"

# Every fixture copy lives under test/.fixtures and resolves packages here.
modules=$package/test/.fixtures/node_modules
mkdir -p "$modules"
rm -rf "$modules/effect" "$modules/effeck-runtime"
ln -s "$effect" "$modules/effect"
ln -s "$runtime" "$modules/effeck-runtime"

# bunfig.toml's minimumReleaseAge applies only to fresh manifests; a warm
# manifest cache skips it (https://github.com/oven-sh/bun/issues/45831).
# Keep --no-cache until that is fixed.
(cd "$package" && bun install --frozen-lockfile --no-cache)

cd "$package"
exec node --test "$@" test/bundlers.test.mjs
