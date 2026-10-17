#!/bin/sh
# Checks editors/nvim/effeck.lua in a headless Neovim 0.10 or later.
# It copies typescript/effeck/lsp-fixture to a scratch project that has
# effeck installed with the compiler under test as its native
# binary, then runs test.lua.
#
# Usage: editors/nvim/test.sh
#
# Environment:
#   NVIM                   the Neovim binary (default: nvim)
#   EFFECK                    the effeck binary (default: ../../typescript/built/local/effeck)
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../../experiments/node_modules, else
#                          ~/effectscript/experiments/node_modules)
#   EFFECK_RUNTIME   the built effeck-runtime package directory
#                          (default: ../../packages/runtime)
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
nvim=${NVIM:-nvim}
effeck=${EFFECK:-$repo/typescript/built/local/effeck}
effect_modules=${EFFECT_NODE_MODULES:-$repo/experiments/node_modules}
[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effeck/experiments/node_modules"
runtime=${EFFECK_RUNTIME:-$repo/packages/runtime}

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
project="$scratch/project"
cp -R "$repo/typescript/effeck/lsp-fixture" "$project"
rm -rf "$project/node_modules"
mkdir -p "$project/node_modules/.bin"
ln -s "$(cd "$effect_modules/effect" && pwd -P)" "$project/node_modules/effect"
ln -s "$(cd "$runtime" && pwd -P)" "$project/node_modules/effeck-runtime"
# node_modules/.bin/effeck is effeck's Node launcher, as bun
# install leaves it, and the launcher runs native/effeck next to it.
compiler="$project/node_modules/effeck"
mkdir -p "$compiler/bin" "$compiler/native"
cp "$repo/packages/compiler/package.json" "$compiler/package.json"
cp "$repo/packages/compiler/bin/effeck" "$compiler/bin/effeck"
chmod +x "$compiler/bin/effeck"
ln -s "$effeck" "$compiler/native/effeck"
ln -s ../effeck/bin/effeck "$project/node_modules/.bin/effeck"

echo "nvim: $("$nvim" --version | head -1)"
"$nvim" --headless --clean -l "$here/test.lua" "$project" "$here/effeck.lua"
