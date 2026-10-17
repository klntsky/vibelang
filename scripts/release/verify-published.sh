#!/bin/sh
# Checks a published Effeck release from the public npm registry: scaffolds an
# app with npm and with Bun, installs, builds, starts, and tests it.
#
#   sh scripts/release/verify-published.sh [version]
#
# The version defaults to the compiler package's version in this checkout.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
version=${1:-$(node -p 'require(process.argv[1]).version' "$root/packages/compiler/package.json")}
sh "$root/scripts/release/check-app.sh" https://registry.npmjs.org/ "$version" npm bun
