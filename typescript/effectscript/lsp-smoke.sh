#!/bin/sh
# Builds esc and @effectscript/runtime, then runs the headless language
# server test, effectscript/lsp-smoke.mjs, on effectscript/lsp-fixture.
#
# Usage: effectscript/lsp-smoke.sh
#
# Environment:
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../experiments/node_modules, else the main
#                          checkout's ~/effectscript/experiments/node_modules)
#   EFFECTSCRIPT_RUNTIME   the @effectscript/runtime package directory
#                          (default: ../packages/runtime, built here with esc)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
esc="$root/built/local/esc"
fixture="$root/effectscript/lsp-fixture"
effect_modules=${EFFECT_NODE_MODULES:-}
if [ -z "$effect_modules" ]; then
	effect_modules="$root/../experiments/node_modules"
	[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effectscript/experiments/node_modules"
fi
runtime=${EFFECTSCRIPT_RUNTIME:-$root/../packages/runtime}
runtime=$(cd "$runtime" && pwd)

# The same flags as smoke.sh, so both builds share Go build cache entries.
echo "build: go build -trimpath -gcflags=all=-dwarf=false -o built/local/esc ./cmd/tsc"
(cd "$root/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$esc" ./cmd/tsc)

# The binary names itself, and the TypeScript version it is built on.
version=$("$esc" --version)
title=$("$esc" --help | head -1)
case "$version" in
EffectScript\ *\(TypeScript\ *\)) echo "ok   esc --version: $version" ;;
*) echo "FAIL esc --version: $version"; exit 1 ;;
esac
case "$title" in
"esc: The EffectScript Compiler - Version "*) echo "ok   esc --help: $title" ;;
*) echo "FAIL esc --help: $title"; exit 1 ;;
esac

# The fixture's .es files use the prelude, so the runtime's declarations must
# be built, as smoke.sh builds them, against the same copy of effect.
effect_real=$(cd "$effect_modules/effect" && pwd -P)
if [ "$(cd "$runtime/node_modules/effect" 2>/dev/null && pwd -P)" != "$effect_real" ]; then
	mkdir -p "$runtime/node_modules"
	rm -rf "$runtime/node_modules/effect"
	ln -s "$effect_real" "$runtime/node_modules/effect"
fi
echo "build: esc -p $runtime/tsconfig.json"
"$esc" -p "$runtime/tsconfig.json"

[ -L "$fixture/node_modules" ] && rm "$fixture/node_modules" # an older lsp-smoke.sh linked the whole directory
mkdir -p "$fixture/node_modules/@effectscript"
ln -sfn "$effect_real" "$fixture/node_modules/effect"
ln -sfn "$runtime" "$fixture/node_modules/@effectscript/runtime"
exec node "$root/effectscript/lsp-smoke.mjs" "$esc"
