#!/bin/sh
# Installs the fixed project's pinned packages from VibeLang's release tarballs (until vibelang and
# vibelang-runtime are on npm) plus effect 4.0.5 from the registry. Linux x64 only; tests use it.
#   sh packages/converter-worker/test/setup-project.sh <dist-npm/tarballs>
set -eu
tarballs=$(cd "$1" && pwd)
project=$(cd "$(dirname "$0")/../project" && pwd)
cd "$project"
npm install --no-audit --no-fund --no-save --omit=dev \
  "$tarballs/vibelang-0.0.1.tgz" "$tarballs/vibelang-runtime-0.0.1.tgz" "$tarballs/vibelang-linux-x64-0.0.1.tgz" effect@4.0.5 >/dev/null
ls node_modules/vibelang-linux-x64/bin/vibelang >/dev/null
echo "setup-project: installed vibelang, vibelang-runtime, vibelang-linux-x64 and effect into $project"
