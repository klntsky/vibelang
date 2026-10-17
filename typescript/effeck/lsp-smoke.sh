#!/bin/sh
# Builds effeck and effeck-runtime, then runs the headless language
# server test, effeck/lsp-smoke.mjs, on effeck/lsp-fixture.
#
# Usage: effeck/lsp-smoke.sh
#
# Environment:
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../experiments/node_modules, else the main
#                          checkout's ~/effectscript/experiments/node_modules)
#   EFFECK_RUNTIME   the effeck-runtime package directory
#                          (default: ../packages/runtime, built here with effeck)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
effeck="$root/built/local/effeck"
fixture="$root/effeck/lsp-fixture"
effect_modules=${EFFECT_NODE_MODULES:-}
if [ -z "$effect_modules" ]; then
	effect_modules="$root/../experiments/node_modules"
	[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effeck/experiments/node_modules"
fi
runtime=${EFFECK_RUNTIME:-$root/../packages/runtime}
runtime=$(cd "$runtime" && pwd)

# The same flags as smoke.sh, so both builds share Go build cache entries.
echo "build: go build -trimpath -gcflags=all=-dwarf=false -o built/local/effeck ./cmd/tsc"
(cd "$root/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$effeck" ./cmd/tsc)

# The binary names itself, and the TypeScript version it is built on.
version=$("$effeck" --version)
title=$("$effeck" --help | head -1)
case "$version" in
Effeck\ *\(TypeScript\ *\)) echo "ok   effeck --version: $version" ;;
*) echo "FAIL effeck --version: $version"; exit 1 ;;
esac
case "$title" in
"effeck: The Effeck Compiler - Version "*) echo "ok   effeck --help: $title" ;;
*) echo "FAIL effeck --help: $title"; exit 1 ;;
esac

# The fixture's .effeck files use the prelude, so the runtime's declarations must
# be built, as smoke.sh builds them, against the same copy of effect.
effect_real=$(cd "$effect_modules/effect" && pwd -P)
if [ "$(cd "$runtime/node_modules/effect" 2>/dev/null && pwd -P)" != "$effect_real" ]; then
	mkdir -p "$runtime/node_modules"
	rm -rf "$runtime/node_modules/effect"
	ln -s "$effect_real" "$runtime/node_modules/effect"
fi
echo "build: effeck -p $runtime/tsconfig.json"
"$effeck" -p "$runtime/tsconfig.json"

[ -L "$fixture/node_modules" ] && rm "$fixture/node_modules" # an older lsp-smoke.sh linked the whole directory
mkdir -p "$fixture/node_modules"
ln -sfn "$effect_real" "$fixture/node_modules/effect"
ln -sfn "$runtime" "$fixture/node_modules/effeck-runtime"
exec node "$root/effeck/lsp-smoke.mjs" "$effeck"
