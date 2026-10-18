#!/usr/bin/env bash
# Check that the committed forms table is what the generator writes today.
#
#   scripts/check-forms-table.sh           regenerate into a temporary file; exit 1 on any difference
#   scripts/check-forms-table.sh --write   regenerate typescript/tsc/internal/effeckmapper/forms_table.jsonl
#
# The table (L1, internal/effeckmapper/forms.go) is generated from the pinned
# declarations of effect, @effect/*, and effeck-runtime by cmd/effeck-forms,
# with the fork's own classifier. It is never edited by hand, so any
# difference means an input changed (a package, the runtime, the classifier)
# or someone edited the file.
#
# Environment:
#   FORMS_NODE_MODULES   effect and @effect/* to read (default docs/tools/node_modules, pinned by docs/tools/bun.lock)
#   EFFECK_FORMS_EXPORT  a file that defines effeck.FormsOfType, mapped into internal/effeck with
#                        go build -overlay, for a tree that does not have it yet
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
table="$root/typescript/tsc/internal/effeckmapper/forms_table.jsonl"
write=0
case "${1:-}" in
  --write) write=1 ;;
  "") ;;
  *) echo "usage: scripts/check-forms-table.sh [--write]" >&2; exit 2 ;;
esac

tmp="$(mktemp -d "${TMPDIR:-/tmp}/effeck-forms.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

overlay=()
if [ -z "${EFFECK_FORMS_EXPORT:-}" ] && ! grep -qs '^func FormsOfType' "$root"/typescript/tsc/internal/effeck/*.go; then
  echo "check-forms-table: internal/effeck does not define FormsOfType, which cmd/effeck-forms needs; set EFFECK_FORMS_EXPORT to a file that does" >&2
  exit 1
fi
if [ -n "${EFFECK_FORMS_EXPORT:-}" ]; then
  printf '{"Replace":{"%s":"%s"}}\n' "$root/typescript/tsc/internal/effeck/formsexport.go" "$EFFECK_FORMS_EXPORT" > "$tmp/overlay.json"
  overlay=(-overlay "$tmp/overlay.json")
fi
modules=()
if [ -n "${FORMS_NODE_MODULES:-}" ]; then
  modules=(-node-modules "$FORMS_NODE_MODULES")
fi

(cd "$root/typescript/tsc" && CGO_ENABLED=0 go run -trimpath ${overlay[@]+"${overlay[@]}"} ./cmd/effeck-forms -repo "$root" ${modules[@]+"${modules[@]}"} -o "$tmp/forms_table.jsonl")

if [ "$write" = 1 ]; then
  cp "$tmp/forms_table.jsonl" "$table"
  echo "check-forms-table: wrote ${table#"$root"/}"
  exit 0
fi
if ! cmp -s "$tmp/forms_table.jsonl" "$table"; then
  diff -u "$table" "$tmp/forms_table.jsonl" | head -20 >&2 || true
  echo "check-forms-table: forms_table.jsonl differs from a fresh generation; run scripts/check-forms-table.sh --write" >&2
  exit 1
fi
echo "check-forms-table: ok, forms_table.jsonl matches a fresh generation"
