#!/bin/sh
# Builds the vibelang compiler and vibelang-runtime from this checkout, then
# bundles test/fixture with webpack, Rspack, Rollup, Rolldown, esbuild, Bun,
# and Vite through vibelang-unplugin and checks each bundle, and bundles the
# compiler's output of test/entry with each of them (entry.test.mjs).
#
# Usage: sh packages/unplugin/test/run.sh [node --test options]
#
# Environment:
#   VIBELANG_BINARY        a native vibelang compiler; skips the Go build
#   EFFECT_NODE_MODULES  a node_modules directory that holds effect 4.0.6
#                        (default: experiments/node_modules)
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
effect=$(cd "$effect_modules/effect" && pwd -P)

# The runtime's dist/ is not checked in. Build it with this compiler against
# the same effect the fixtures use, so a bundle holds one copy of effect.
mkdir -p "$runtime/node_modules"
rm -rf "$runtime/node_modules/effect"
ln -s "$effect" "$runtime/node_modules/effect"
echo "build: vibelang -p packages/runtime/tsconfig.json"
"$VIBELANG_BINARY" -p "$runtime/tsconfig.json"

# Every fixture copy lives under test/.fixtures and resolves packages here.
modules=$package/test/.fixtures/node_modules
mkdir -p "$modules"
rm -rf "$modules/effect" "$modules/vibelang-runtime" "$modules/vibelang" "$modules/vibelang-unplugin"
ln -s "$effect" "$modules/effect"
ln -s "$runtime" "$modules/vibelang-runtime"
# The plugin's compile backend is the project's vibelang package (vibelang/project).
ln -s "$repo/packages/compiler" "$modules/vibelang"
# A fixture program that imports vibelang-unplugin/deferred resolves this package.
ln -s "$package" "$modules/vibelang-unplugin"

# bunfig.toml's minimumReleaseAge applies only to fresh manifests; a warm
# manifest cache skips it (https://github.com/oven-sh/bun/issues/45831).
# Keep --no-cache until that is fixed.
(cd "$package" && bun install --frozen-lockfile --no-cache)

cd "$package"
exec node --test "$@" test/bundlers.test.mjs test/deferred.test.mjs test/deferred-types.test.mjs test/entry.test.mjs
