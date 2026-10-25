#!/bin/sh
# Installs the fixed project's pinned packages from VibeLang's release tarballs (until vibelang and
# vibelang-runtime are on npm) plus effect 4.0.5 from the registry. Linux x64 only; tests use it.
#   sh packages/converter-worker/test/setup-project.sh <dist-npm/tarballs>
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
version=$(node -p 'JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8")).version' "$root/packages/compiler/package.json")
effect=$(node -p 'JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8")).peerDependencies.effect' "$root/packages/runtime/package.json")
tarballs=$(cd "$1" && pwd)
project=$(cd "$(dirname "$0")/../project" && pwd)
cd "$project"
npm install --no-audit --no-fund --no-save --omit=dev \
  "$tarballs/vibelang-$version.tgz" "$tarballs/vibelang-runtime-$version.tgz" "$tarballs/vibelang-linux-x64-$version.tgz" "effect@$effect" >/dev/null
ls node_modules/vibelang-linux-x64/bin/vibelang >/dev/null
echo "setup-project: installed vibelang, vibelang-runtime, vibelang-linux-x64 and effect into $project"
