#!/bin/sh
# Compile all examples, then compare finite programs with their output fixtures.
# VIBELANG=/absolute/path/to/vibelang scripts/check-examples.sh
# Detailed diagnostics and output remain in .cache/check-examples/.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
logs="$root/.cache/check-examples"
mkdir -p "$logs"
compiler=${VIBELANG:-}
if [ -z "$compiler" ]; then
  if command -v vibelang >/dev/null 2>&1; then
    compiler=$(command -v vibelang)
  else
    compiler="$root/typescript/built/local/tsc"
  fi
fi
# Resolve a caller-supplied relative path before changing into an example.
case "$compiler" in
  /*) ;;
  */*) compiler="$(pwd)/$compiler" ;;
esac
failed=0
for name in hello users http-api file-tree resilience approval pipeline; do
  dir="$root/examples/$name"
  log="$logs/$name.log"
  if ! (cd "$dir" && "$compiler" -p .) >"$log" 2>&1; then
    printf 'FAIL %-10s compile (see .cache/check-examples/%s.log)\n' "$name" "$name"
    failed=1
    continue
  fi
  if [ ! -f "$dir/expected-output.txt" ]; then
    printf 'OK   %-10s compile (long-running server; use bun run test)\n' "$name"
    continue
  fi
  # main.js runs its exported main when node starts it (D135).
  entry=dist/main.js
  # Bound execution so an incorrect producer or fiber lifetime cannot hang CI.
  if ! (cd "$dir" && node --input-type=module - "$entry" <<'JS'
import { spawnSync } from "node:child_process"
const result = spawnSync(process.execPath, [process.argv[2]], {
  encoding: "utf8", timeout: 15000, killSignal: "SIGKILL"
})
process.stdout.write(result.stdout ?? "")
process.stderr.write(result.stderr ?? "")
if (result.error) console.error(result.error.message)
process.exit(result.status ?? 1)
JS
  ) >"$log" 2>&1; then
    printf 'FAIL %-10s run (see .cache/check-examples/%s.log)\n' "$name" "$name"
    failed=1
  elif ! diff -u "$dir/expected-output.txt" "$log" >"$logs/$name.diff"; then
    printf 'FAIL %-10s output (see .cache/check-examples/%s.diff)\n' "$name" "$name"
    failed=1
  else
    printf 'OK   %-10s compile and output\n' "$name"
  fi
done
exit "$failed"
