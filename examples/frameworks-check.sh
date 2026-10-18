#!/bin/sh
# Install, type-check, test, and build the React, Vue, and Svelte examples.
#   sh examples/frameworks-check.sh [react-vite vue-vite svelte-vite]
# Run sh scripts/setup.sh first: the examples install this checkout's
# compiler, runtime tarball, and effeck-vite through file: dependencies.
# EFFECK_BINARY=/absolute/path/to/effeck selects the compiler for the effeck
# launcher and effeck-vite; the default is packages/compiler/native/effeck.
# Each example gets one status line. Logs stay in .cache/frameworks-check/.
# bun install --no-cache fetches fresh manifests, so each example's
# bunfig.toml minimumReleaseAge applies; a warm manifest cache skips it.
set -eu
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(dirname -- "$here")
logs="$root/.cache/frameworks-check"
mkdir -p "$logs"
EFFECK_BINARY=${EFFECK_BINARY:-$root/packages/compiler/native/effeck}
case "$EFFECK_BINARY" in
  /*) ;;
  *) EFFECK_BINARY="$(pwd)/$EFFECK_BINARY" ;;
esac
export EFFECK_BINARY
if [ ! -x "$EFFECK_BINARY" ]; then
  echo "frameworks-check: no compiler at $EFFECK_BINARY; run sh scripts/setup.sh" >&2
  exit 1
fi
if [ ! -f "$root/packages/runtime/effeck-runtime.tgz" ]; then
  echo "frameworks-check: no runtime tarball; run sh scripts/setup.sh" >&2
  exit 1
fi
[ "$#" -gt 0 ] || set -- react-vite vue-vite svelte-vite

failed=0
# step NAME COMMAND...: runs COMMAND in the example and appends to its log.
step() {
  label=$1
  shift
  printf '$ %s\n' "$*" >>"$log"
  if (cd "$dir" && "$@") >>"$log" 2>&1; then
    return 0
  fi
  printf 'FAIL %-12s %s (see .cache/frameworks-check/%s.log)\n' "$name" "$label" "$name"
  failed=1
  return 1
}
for name in "$@"; do
  dir="$here/$name"
  log="$logs/$name.log"
  : >"$log"
  step install bun install --no-cache &&
    step typecheck bun run typecheck &&
    step vitest bun x vitest run &&
    step build bun run build &&
    printf 'OK   %-12s install, effeck type check, Vitest, vite build\n' "$name"
done
exit "$failed"
