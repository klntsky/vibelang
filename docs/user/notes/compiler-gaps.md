# Compiler gaps in user examples

Run `bun docs/tools/check-snippets.ts` from the repository root. The checker
compiles each page once with the native compiler and checks every marked gap.
A gap that no longer produces an unexpected error or misses its expected
error fails the check until its marker is removed. The checker reports Markdown
line numbers and can write full per-block diagnostics with `--json path`.

This inventory records language/compiler disagreements, not new language rules.
The rule references are in `docs/design/DECISIONS.md`. A `// fragment` example
instead omits a fixture or contains an incomplete integration sketch. Type
checking does not prove runtime behavior; the native smoke suite covers that.

## Evidence and scope

Each row identifies the missing contract, resource, producer, or value-call
behavior and the exact diagnostic observed.

The tooling gaps are separate: distribution uses local file dependencies and a
local VSIX, Vite uses compiler subprocesses, declaration emit has no `$direct`
metadata, and the compiler has no custom migration refactors or mechanical ES
code actions. See the Devtools guide and design chapters 27 and 28. No Oxlint
configuration package is shipped. The runtime README also records the producer
adapter's one-element buffer, which differs from D26's zero-capacity handoff.

## Checked blocks

| Page and block | First code line | Diagnostic | Rule and disagreement |
| --- | --- | --- | --- |
| [053-introduction.md, block 5](../src/stream/053-introduction.md) | `import { Stream } from "effect"` | TS2739, TS18046, TS2488: Type 'Generator<number, void, unknown>' is missing the following properties from type 'Stream<unknown, unknown, unknown>': channel, pipe, [TypeId] | D26: A producer must adapt to Stream and for-of must consume a Stream. |
