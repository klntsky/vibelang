#!/bin/sh
# Creates <dir>/node_modules with the packages the benchmark projects import:
# effect (the runtime's own copy, so one copy loads), effeck-runtime (this
# checkout's packages/runtime), and the rest from experiments/node_modules and
# docs/tools/node_modules, as docs/tools/check-snippets.ts does.
#
#   sh scripts/bench/modules.sh <dir>
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
dir=$1
modules="$dir/node_modules"
mkdir -p "$modules"
[ -f "$root/packages/runtime/dist/index.js" ] || { echo "modules.sh: build packages/runtime first" >&2; exit 1; }
ln -sfn "$(cd "$root/packages/runtime/node_modules/effect" && pwd -P)" "$modules/effect"
ln -sfn "$root/packages/runtime" "$modules/effeck-runtime"
for source in "$root/experiments/node_modules" "$root/docs/tools/node_modules"; do
  [ -d "$source" ] || continue
  for entry in "$source"/* "$source"/@*; do
    [ -e "$entry" ] || continue
    name=$(basename "$entry")
    case $name in effect|effeck-runtime|effeck-vite|.bin) continue ;; esac
    [ -e "$modules/$name" ] || ln -s "$entry" "$modules/$name"
  done
done
[ -f "$dir/package.json" ] || echo '{"type":"module","private":true}' > "$dir/package.json"
