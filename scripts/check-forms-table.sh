#!/usr/bin/env bash
# Check that the committed forms table, and the decisions it makes, are
# what the generator and the resolver give today.
#
#   scripts/check-forms-table.sh           regenerate both into a temporary directory; exit 1 on any difference
#   scripts/check-forms-table.sh --write   regenerate forms_table.jsonl and forms_decisions.txt
#
# forms_decisions.txt records what the resolver decides at every lambda,
# block, and by-name argument of the repository's .effeck files
# (typescript/effeck, examples, templates). --write refuses a decision that
# moved at a site the snapshot already has unless
# forms_decisions_approved.txt names the move, one line each:
#
#   typescript/effeck/cases/x/main.effeck:12:7 lambda: plain fallback -> effectFn table # why it is right
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
dir="$root/typescript/tsc/internal/effeckmapper"
table="$dir/forms_table.jsonl"
decisions="$dir/forms_decisions.txt"
approved="$dir/forms_decisions_approved.txt"
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

gate=()
if [ "$write" = 1 ]; then
  gate=(-previous "$decisions" -approved "$approved")
fi
(cd "$root/typescript/tsc" && CGO_ENABLED=0 go run -trimpath ${overlay[@]+"${overlay[@]}"} ./cmd/effeck-forms -repo "$root" ${modules[@]+"${modules[@]}"} \
  -o "$tmp/forms_table.jsonl" -decisions "$tmp/forms_decisions.txt" ${gate[@]+"${gate[@]}"})

if [ "$write" = 1 ]; then
  cp "$tmp/forms_table.jsonl" "$table"
  cp "$tmp/forms_decisions.txt" "$decisions"
  echo "check-forms-table: wrote ${table#"$root"/} and ${decisions#"$root"/}"
  exit 0
fi
status=0
if ! cmp -s "$tmp/forms_table.jsonl" "$table"; then
  diff -u "$table" "$tmp/forms_table.jsonl" | head -20 >&2 || true
  echo "check-forms-table: forms_table.jsonl differs from a fresh generation; run scripts/check-forms-table.sh --write" >&2
  status=1
fi
if ! cmp -s "$tmp/forms_decisions.txt" "$decisions"; then
  diff -u "$decisions" "$tmp/forms_decisions.txt" | grep '^[-+][^-+]' | head -20 >&2 || true
  echo "check-forms-table: decisions differ from forms_decisions.txt; run scripts/check-forms-table.sh --write (a moved decision needs a line in forms_decisions_approved.txt)" >&2
  status=1
fi
[ "$status" = 0 ] && echo "check-forms-table: ok, forms_table.jsonl and forms_decisions.txt match a fresh generation"
exit "$status"
