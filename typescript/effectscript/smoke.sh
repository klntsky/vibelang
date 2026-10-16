#!/bin/sh
# Builds tsc, then compiles and runs every case under effectscript/cases.
#
# A case is a directory with a tsconfig.json. The compile must succeed with no
# diagnostics. Every file in <case>/expected/ except stdout.txt must equal the
# file at the same path under <case>/out/. stdout.txt must equal what
# `node <case>/run.mjs` (or `node <case>/out/main.js`) prints.
#
# Cases whose package.json depends on "effect" get a node_modules symlink to
# $EFFECT_NODE_MODULES (default: ../experiments/node_modules, falling back to
# the main checkout's ~/effectscript/experiments/node_modules).
#
# Usage: effectscript/smoke.sh [case-name...]
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
cases="$root/effectscript/cases"
tsc="$root/built/local/tsc"
effect_modules=${EFFECT_NODE_MODULES:-}
if [ -z "$effect_modules" ]; then
	effect_modules="$root/../experiments/node_modules"
	[ -d "$effect_modules" ] || effect_modules="$HOME/effectscript/experiments/node_modules"
fi

# -trimpath keys the Go build cache by module path instead of directory, so
# every jj workspace shares one set of cache entries. Without DWARF the cache
# entries are smaller; panics still print stack traces.
echo "build: go build -trimpath -gcflags=all=-dwarf=false -o built/local/tsc ./cmd/tsc"
(cd "$root/tsc" && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o "$tsc" ./cmd/tsc)

if [ $# -eq 0 ]; then
	set -- $(cd "$cases" && ls -d */tsconfig.json | sed 's,/tsconfig.json$,,')
fi

failed=0
for name in "$@"; do
	dir="$cases/$name"
	log=$(mktemp)
	rm -rf "$dir/out"
	if grep -q '"effect"' "$dir/package.json" 2>/dev/null; then
		ln -sfn "$effect_modules" "$dir/node_modules"
	fi
	if ! "$tsc" -p "$dir" >"$log" 2>&1 || [ -s "$log" ]; then
		echo "FAIL $name: tsc reported diagnostics"
		cat "$log"
		failed=1
		continue
	fi
	for want in $(cd "$dir/expected" && find . -type f ! -name stdout.txt); do
		if ! diff -u "$dir/expected/$want" "$dir/out/$want" >"$log" 2>&1; then
			echo "FAIL $name: out/$want differs from expected/$want"
			cat "$log"
			failed=1
			continue 2
		fi
	done
	entry="$dir/out/main.js"
	[ -f "$dir/run.mjs" ] && entry="$dir/run.mjs"
	if ! node "$entry" >"$log" 2>&1 || ! diff -u "$dir/expected/stdout.txt" "$log"; then
		echo "FAIL $name: node $(basename "$entry") output differs from expected/stdout.txt"
		cat "$log"
		failed=1
		continue
	fi
	rm -f "$log"
	echo "ok   $name"
done
exit $failed
