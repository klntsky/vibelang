#!/bin/sh
# Builds esc, then runs the headless language server test,
# effectscript/lsp-smoke.mjs, on effectscript/lsp-fixture.
#
# Usage: effectscript/lsp-smoke.sh
#
# Environment:
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../experiments/node_modules, else the main
#                          checkout's ~/effectscript/experiments/node_modules)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
esc="$root/built/local/esc"
effect_modules=${EFFECT_NODE_MODULES:-}
if [ -z "$effect_modules" ]; then
	effect_modules="$root/../experiments/node_modules"
	[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effectscript/experiments/node_modules"
fi

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

ln -sfn "$effect_modules" "$root/effectscript/lsp-fixture/node_modules"
exec node "$root/effectscript/lsp-smoke.mjs" "$esc"
