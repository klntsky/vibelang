#!/usr/bin/env bash
# End-to-end check of Effeck's TypeScript 7 content mapper with stock tools:
# stock TypeScript 7 (npm typescript@next, not the Effeck fork) checks a
# project in which main.ts imports ./greet.effeck through effeck-ts-mapper.
#
#   packages/ts-mapper/test/e2e.sh
#
# Steps:
#   1. go test the projector and the protocol server, and check that the
#      generated L1 forms table matches a fresh generation
#      (scripts/check-forms-table.sh; never hand-edit the table).
#   2. Build bin/effeck-mapper.
#   3. Install typescript@next and effect 4.0.5 into test/.cache, then
#      assemble the fixture in a temporary directory whose node_modules hold
#      a copy of this package, effect, and effeck-runtime built from
#      packages/runtime.
#   4. tsc --runExternalCode must print exactly test/expected-tsc.txt: one
#      error in main.ts (a misuse of greet's inferred type) and one in
#      greet.effeck, at its .effeck position. forms-check.ts pins the
#      types each lambda form projects to (forms.effeck): a wrong form
#      adds a TS2322 there.
#   5. The stock language server must answer hovers and diagnostics at
#      .effeck positions (test/lsp-hover.mjs).
#   6. Without effect beside the prelude, the sentinel on each projected
#      file's first line must report TS1360 at line 1, naming the fix.
#
# Environment:
#   TS7_VERSION  typescript version to test (default 7.1.0-dev.20261017.1)
#   EFFECT_DIR   an installed effect 4.0.5 to use instead of the cached one
#   KEEP=1       keep the temporary project and print its path
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package="$(cd "$here/.." && pwd)"
root="$(cd "$package/../.." && pwd)"
ts7_version="${TS7_VERSION:-7.1.0-dev.20261017.1}"

step() { printf '\n== %s\n' "$*"; }

step "go test ./internal/effeckmapper"
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go test -trimpath -gcflags=all=-dwarf=false ./internal/effeckmapper/)

step "forms table matches its generator"
[ -d "$root/docs/tools/node_modules" ] || (cd "$root/docs/tools" && bun install --silent)
bash "$root/scripts/check-forms-table.sh"

step "build bin/effeck-mapper"
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$package/bin/effeck-mapper" ./cmd/effeck-mapper)

step "install typescript@$ts7_version and effect@4.0.5"
cache="$here/.cache/typescript-$ts7_version"
if [ ! -x "$cache/node_modules/.bin/tsc" ] || [ ! -f "$cache/node_modules/effect/package.json" ]; then
  mkdir -p "$cache"
  (cd "$cache" && printf '{"private":true}\n' > package.json && npm install --no-audit --no-fund --silent "typescript@$ts7_version" effect@4.0.5)
fi
tsc="$cache/node_modules/.bin/tsc"
effect_dir="${EFFECT_DIR:-$cache/node_modules/effect}"
"$tsc" --version

step "assemble the fixture"
[ -f "$effect_dir/package.json" ] || { echo "e2e: no effect at $effect_dir; set EFFECT_DIR" >&2; exit 1; }
work="$(mktemp -d "${TMPDIR:-/tmp}/effeck-ts-mapper-e2e.XXXXXX")"
if [ "${KEEP:-}" = 1 ]; then echo "e2e: keeping $work"; else trap 'rm -rf "$work"' EXIT; fi
cp -R "$here/fixture/." "$work/"
mkdir -p "$work/node_modules/effeck-ts-mapper"
cp -R "$package/package.json" "$package/prelude.d.ts" "$package/bin" "$work/node_modules/effeck-ts-mapper/"
ln -s "$effect_dir" "$work/node_modules/effect"
# The prelude re-exports effeck-runtime's prelude names, so the runtime's
# declarations sit beside effect. Build them from a copy of the sources whose
# node_modules holds the same effect, without touching packages/runtime.
runtime="$work/node_modules/effeck-runtime"
mkdir -p "$work/.runtime-build/node_modules" "$runtime"
cp -R "$root/packages/runtime/src" "$root/packages/runtime/tsconfig.json" "$work/.runtime-build/"
ln -s "$effect_dir" "$work/.runtime-build/node_modules/effect"
"$tsc" -p "$work/.runtime-build/tsconfig.json" --outDir "$runtime/dist" --declarationMap false --sourceMap false --inlineSources false
cp "$root/packages/runtime/package.json" "$runtime/"
echo "$work"

step "cross-file impure recursion stays stack-safe (B)"
node "$here/cross-file-stack.mjs" "$package/bin/effeck-mapper" "$work"

step "tsc --runExternalCode"
status=0
(cd "$work" && "$tsc" -p . --pretty false --runExternalCode) > "$work/tsc.txt" 2>&1 || status=$?
cat "$work/tsc.txt"
if [ "$status" -ne 2 ]; then echo "e2e: tsc exited $status, expected 2 (errors reported)" >&2; exit 1; fi
if ! diff -u "$here/expected-tsc.txt" "$work/tsc.txt"; then echo "e2e: tsc output differs from expected-tsc.txt" >&2; exit 1; fi
echo "ok   tsc output matches expected-tsc.txt"

step "tsc --lsp --stdio: hover and diagnostics at .effeck positions"
# bin/tsc is a Node launcher; the language server is the native binary it starts.
native="$(node -e 'const r=require("node:module").createRequire(process.argv[1]);const p=`@typescript/typescript-${process.platform}-${process.arch}`;console.log(require("node:path").join(require("node:path").dirname(r.resolve(p+"/package.json")),"lib","tsc"))' "$cache/node_modules/typescript/package.json")"
node "$here/lsp-hover.mjs" "$native" "$work"

step "the prelude sentinel: no effect beside the prelude fails at line 1"
# skipLibCheck hides prelude.d.ts's own failed imports, which would turn every
# helper into the error type; the sentinel on each file's first line names the fix.
# A directory of its own: inside $work, resolution would find $work/node_modules/effect.
bare="$(mktemp -d "${TMPDIR:-/tmp}/effeck-ts-mapper-no-effect.XXXXXX")"
if [ "${KEEP:-}" = 1 ]; then echo "e2e: keeping $bare"; else trap 'rm -rf "$work" "$bare"' EXIT; fi
mkdir -p "$bare/src" "$bare/node_modules/effeck-ts-mapper"
cp "$here/fixture/tsconfig.json" "$here/fixture/package.json" "$bare/"
cp "$here/fixture/src/greet.effeck" "$bare/src/"
cp -R "$package/package.json" "$package/prelude.d.ts" "$package/bin" "$bare/node_modules/effeck-ts-mapper/"
(cd "$bare" && "$tsc" -p . --pretty false --runExternalCode) > "$bare/tsc.txt" 2>&1 || true
if ! grep -q '^src/greet.effeck(1,[0-9]*): error TS1360: .*the prelude cannot resolve effect from its own location; install effect 4.0.5 beside effeck-ts-mapper' "$bare/tsc.txt"; then
  cat "$bare/tsc.txt"; echo "e2e: the prelude sentinel did not report a missing effect at line 1" >&2; exit 1
fi
echo "ok   TS1360 at greet.effeck line 1 names the fix"

step "e2e: all checks passed"
