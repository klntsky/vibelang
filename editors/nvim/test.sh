#!/bin/sh
# Checks editors/nvim/effectscript.lua in a headless Neovim 0.10 or later.
# It copies typescript/effectscript/lsp-fixture to a scratch project that has
# @effectscript/compiler installed with the compiler under test as its native
# binary, then runs test.lua.
#
# Usage: editors/nvim/test.sh
#
# Environment:
#   NVIM                   the Neovim binary (default: nvim)
#   ESC                    the esc binary (default: ../../typescript/built/local/esc)
#   EFFECT_NODE_MODULES    a node_modules directory that holds effect 4.0.5
#                          (default: ../../experiments/node_modules, else
#                          ~/effectscript/experiments/node_modules)
#   EFFECTSCRIPT_RUNTIME   the built @effectscript/runtime package directory
#                          (default: ../../packages/runtime)
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
nvim=${NVIM:-nvim}
esc=${ESC:-$repo/typescript/built/local/esc}
effect_modules=${EFFECT_NODE_MODULES:-$repo/experiments/node_modules}
[ -d "$effect_modules/effect" ] || effect_modules="$HOME/effectscript/experiments/node_modules"
runtime=${EFFECTSCRIPT_RUNTIME:-$repo/packages/runtime}

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
project="$scratch/project"
cp -R "$repo/typescript/effectscript/lsp-fixture" "$project"
rm -rf "$project/node_modules"
mkdir -p "$project/node_modules/.bin" "$project/node_modules/@effectscript"
ln -s "$(cd "$effect_modules/effect" && pwd -P)" "$project/node_modules/effect"
ln -s "$(cd "$runtime" && pwd -P)" "$project/node_modules/@effectscript/runtime"
# node_modules/.bin/esc is @effectscript/compiler's Node launcher, as bun
# install leaves it, and the launcher runs native/esc next to it.
compiler="$project/node_modules/@effectscript/compiler"
mkdir -p "$compiler/bin" "$compiler/native"
cp "$repo/packages/compiler/package.json" "$compiler/package.json"
cp "$repo/packages/compiler/bin/esc" "$compiler/bin/esc"
chmod +x "$compiler/bin/esc"
ln -s "$esc" "$compiler/native/esc"
ln -s ../@effectscript/compiler/bin/esc "$project/node_modules/.bin/esc"

echo "nvim: $("$nvim" --version | head -1)"
"$nvim" --headless --clean -l "$here/test.lua" "$project" "$here/effectscript.lua"
