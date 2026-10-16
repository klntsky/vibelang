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
| [050-pubsub.md, block 1](../src/concurrency/050-pubsub.md) | `import { PubSub } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Subscription<string>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [050-pubsub.md, block 3](../src/concurrency/050-pubsub.md) | `import { PubSub } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Subscription<string>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [050-pubsub.md, block 4](../src/concurrency/050-pubsub.md) | `import { PubSub } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Subscription<string>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [050-pubsub.md, block 5](../src/concurrency/050-pubsub.md) | `import { PubSub } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Subscription<number>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [050-pubsub.md, block 6](../src/concurrency/050-pubsub.md) | `import { PubSub } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Subscription<string>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [034-tracking-fibers.md, block 1](../src/observability/034-tracking-fibers.md) | `import { Fiber, FiberSet } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'FiberSet<number, unknown>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [034-tracking-fibers.md, block 2](../src/observability/034-tracking-fibers.md) | `import { Fiber, FiberMap } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'FiberMap<string, string, unknown>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [034-tracking-fibers.md, block 3](../src/observability/034-tracking-fibers.md) | `import { FiberSet } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'FiberSet<void, unknown>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [034-tracking-fibers.md, block 4](../src/observability/034-tracking-fibers.md) | `import { FiberSet } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'FiberSet<void, unknown>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [106-file-system.md, block 3](../src/platform/106-file-system.md) | `import { FileSystem } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'File' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [029-introduction.md, block 5](../src/resource-management/029-introduction.md) | `function acquireLease() {` | TS2741: Property '[Symbol.dispose]' is missing in type 'Lease' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [030-scope.md, block 3](../src/resource-management/030-scope.md) | `function openBuffer(label: string) {` | TS2741: Property '[Symbol.dispose]' is missing in type '{ label: string; }' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [030-scope.md, block 4](../src/resource-management/030-scope.md) | `function nestedLifetimes() {` | TS2741: Property '[Symbol.dispose]' is missing in type '{ label: string; }' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [053-introduction.md, block 5](../src/stream/053-introduction.md) | `import { Stream } from "effect"` | TS2739, TS18046, TS2488: Type 'Generator<number, void, unknown>' is missing the following properties from type 'Stream<unknown, unknown, unknown>': channel, pipe, [TypeId] | D26: A producer must adapt to Stream and for-of must consume a Stream. |
| [057-operations.md, block 3](../src/stream/057-operations.md) | `import { Pull, Stream } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type 'Pull<readonly [number, ...number[]], never, void, never>' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [057-operations.md, block 8](../src/stream/057-operations.md) | `import { Filter, Stream } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type '[passes: Stream<number, never, never>, fails: Stream<number, never, never>]' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
| [057-operations.md, block 12](../src/stream/057-operations.md) | `import { Stream } from "effect"` | TS2741: Property '[Symbol.dispose]' is missing in type '[Stream<number, never, never>, Stream<number, never, never>]' but required in type 'Disposable'. | D14/D47: A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value. |
