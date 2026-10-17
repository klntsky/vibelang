# effeck-runtime

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

The Effeck runtime: the one-shot continuation kernel and the helpers that the
[`effeck`](https://www.npmjs.com/package/effeck) compiler emits. It runs on
Effect 4's fibers, scopes, and services.

## Install

```sh
npm install effeck-runtime effect@4.0.5
```

JavaScript compiled by `effeck` imports `effect` and `effeck-runtime`, so an
application lists both as dependencies; apps made with `create-effeck` already
do. The runtime requires **effect@4.0.5 exactly**, because the kernel accesses
that version's fiber continuation stack. Install one copy of `effect`: with two
copies, a Scope can miss the fibers it must stop.

## Build and test

For contributors, from this directory of the Effeck repository:

```sh
bun install
bun run build
bun run test
```

The build uses TypeScript to write ES2022 JavaScript modules, declarations,
JavaScript source maps, and declaration maps to `dist/`. Node 26 and Bun 1.4
load the package's JavaScript export. TypeScript resolves the same export to
`dist/index.d.ts`. The package includes `dist/`, this README, and the license; build it
before packing or installing a local copy. Generated files under `dist/` are ignored.

`bun run test` builds first and then runs behavior tests through the package
export. `bun test` uses the current build. The tests cover each helper module,
resource and interruption behavior, and isolated Node, Bun, and browser bundle
consumers. They also compile a TypeScript consumer against the declarations.

The original suites remain available from the repository root:

```sh
(cd experiments && bun test)                  # 148 kernel law tests
bun prototype/esc/examples/run-all.ts          # 46 compiler examples
```

For example, generated JavaScript can compose helpers directly:

```js
import { Effect, $bind, $program } from "effeck-runtime"

const answer = $program(() => $bind(42))
console.log(await Effect.runPromise(answer))
```

Applications normally write `.effeck` programs and let the compiler select these
helpers. Programs stay lazy: a helper that receives a program does not start a
root fiber. An explicit Effect runner starts execution at the application
boundary.

## Effect version guard

At module load, `version.js` statically imports the public `effect/package.json`
export with the JSON import attribute and checks its version. This works in Node
and Bun, and bundlers resolve the same metadata into their output without a
Node module loader or file-system access. A mismatch throws an error naming the
runtime version, the installed and required Effect versions, and the command
`npm install effect@4.0.5`. Package metadata preserves module side effects so
browser tree shaking cannot drop the guard when a caller imports only a helper.
The mismatch tests exercise both a different patch version and Effect 3.

## Public exports

This table lists every export from the package root. The `$` prefix identifies
compiler helpers. Types have no JavaScript value. Decision references point to
[`docs/design/DECISIONS.md`](../../docs/design/DECISIONS.md).

| Export | What generated code uses it for | Decision |
| --- | --- | --- |
| `$bind` | Run an Effect result or lift an ordinary value during prototype call lowering | D24 |
| `Bind` (type) | Preserve the success, failure, and requirement types of `$bind` | D24 |
| `$program` | Convert a zero-argument function to a lazy, rerunnable Effect | D7 |
| `$programs` | Convert an iterable of zero-argument functions to an array of Effects, or a record of them to a record of Effects | D49 |
| `$normal` | Represent normal completion of a lowered region or loop body | D14, D26 |
| `$catch` | Recover a cause containing expected failures only | D13, D28, D46 |
| `$isFailure` | Test a caught failure's `_tag` against a typed catch clause's tags | D131 |
| `$finally` | Run cleanup on every exit and retain cleanup failures | D15, D46 |
| `$expected` | Mark a value thrown in a plain-JavaScript function as an expected failure | D54 |
| `$caught` | Unwrap an expected failure in a plain-JavaScript `catch`, and rethrow a defect | D13, D54 |
| `$region` | Create a block-owned scope without adding it to the body's Context | D14, D47 |
| `$using` | Run an acquisition with a block's scope, keeping the acquisition's own interruptibility, and register `Symbol.dispose` cleanup with interruption masked | D14, D25 |
| `$fork` | Fork into a block's scope, with `forkIn`'s options, or require a scope from the caller | D19, D47 |
| `fork` | Expose `Effect.forkScoped` to TypeScript and prelude type positions | D4, D19 |
| `$Context` | Expose Effect's Context namespace for generated service references | D1, D40 |
| `$service` | Create the Context key base merged with a generated service interface | D1, D45 |
| `$reference` | Create the `Context.Reference` base of a service class that declares `static readonly default` | D40, D45 |
| `ServiceKey` (type) | Name the static side of a compiled service or control class in its declaration file | D45 |
| `ReferenceKey` (type) | Name the static side of a compiled service class with a static default in its declaration file | D40, D45 |
| `$control` | Register one kernel operation per generated control method | D2 |
| `Control` | Mark an abstract control declaration in the prelude | D2, D4 |
| `wire` | Compose layers in the dependent-first order computed by the compiler | D27 |
| `Continuation` (type) | Type the clause-local, one-shot continuation token: `Continuation<V, E, H>` | D16, D50 |
| `handle` | Install a deep handler and optional return clause; a clause for every operation | D16, D50 |
| `resume` | Resume a captured continuation with a value | D16 |
| `resumeWith` | Resume with an injected Effect that requires no services | D17 |
| `abandon` | Consume a continuation and run its cleanup without resuming its body | D16 |
| `$tailResume` | Return a tail-resume request without adding a clause frame | D18 |
| `$tailResumeWith` | Return a tail-resume request with an injected Effect | D17, D18 |
| `$clause` | Type a handle statement's clause bindings for the checker; compiled code never calls it | D132 |
| `YieldOf` (type) | Type a producer body's yields until a loop or a Stream handles them | D26 |
| `Producer` (type) | A producer: a `Stream` that also carries its body | D26 |
| `$produce` | Turn a generated producer body into a cold `Stream` that hands over one element per pull | D26 |
| `$yield` | Perform the built-in yield operation | D26 |
| `$yieldAll` | Yield every element of another producer, a Stream, or an iterable (`yield*`) | D26 |
| `$forOf` | Iterate a producer under a `Yield` handler, or pull a Stream, and lower loop completion records | D18, D26 |
| `$forStream`, `$toStream` | Kept for the prototype compiler's output: `$forOf`, and the identity | D26 |
| `$trusted` | Run a trusted call: an instance of a listed class is an expected failure, any other exception a defect | D5, D23 |
| `$trustedPlain` | The same in a function that compiles to plain JavaScript: throw a listed exception as `$expected` for its `catch` | D54 |
| `Effect` | Supply the prelude Effect namespace | D4 |
| `Layer` | Supply the prelude Layer namespace | D4 |
| `Schema` | Supply the prelude Schema namespace | D4 |
| `Scope` | Supply the prelude Scope namespace | D48 |
| `TaggedError` | Expose Effect's `Data.TaggedError` | D3 |
| `pipe` | Supply the prelude pipeline function | D8 |
| `Exit` | Expose Effect completion values to TypeScript adapters | D43 |
| `runMain` | Run a prototype entry program and return its Exit | D38 |
| `$runMain` | Run an entry module's `main` as `NodeRuntime.runMain` would, when `@effect/platform-node` is not installed | D135 |
| `suspend` | Bridge callback registration with a resumer that reports a lost race | D43 |
| `Resumer` (type) | Type `resume` and the callback registration state | D43 |
| `handoff` | Bridge sources that acknowledge cancellation before releasing late values | D43 |
| `HandoffProtocolViolation` | Report duplicate delivery or delivery after acknowledged shutdown | D43 |

The kernel's implementation modules are private package files. Their source
remains in `src/kernel/` so the original law and trace suites can test the
continuation machine directly. Generated code uses the package root. Tests and
instrumentation import the kernel module itself from
`effeck-runtime/kernel`, for example its `hooks`, which report each
capture and resumption; the compiler's acceptance case `stack-depth` uses them
to check that loops run in constant stack.

## Prototype compatibility

The extraction preserves the prototype's helper names and execution behavior.
The compiler orders `wire` arguments with dependents before providers; the
runtime itself does not inspect or sort Layer dependencies. `$trusted` tests
`instanceof` against the classes the compiler passes (D51), where the prototype
matched exception tags and constructor names.

`Continuation<V, E, H>` follows D50: `V` is what the perform site receives, `E`
the operation's declared errors, which `resumeWith`'s program may raise, and `H`
what `resume` returns. `handle` takes `H` to be the program's own result, which
is exact when every clause that returns a value of another type does not
resume; with `onReturn`, `H` is `never`, so a clause's tail `resume` adds
nothing to the `handle`'s result (D97).

A producer is a `Stream`. `for...of` over it runs its body under a `Yield`
handler on the loop's fiber and resumes it in tail position, so a long producer
runs in constant stack; a loop that ends early abandons it, which runs its
cleanup. Anything else that runs it as a `Stream` starts its body on a fiber of
its own on the first pull, and the `Yield` clause completes the pending pull and
waits for the next one before it resumes the body: a handoff with no buffer, so
the producer never runs ahead of its consumer (D26; the prototype's adapter used
a one-element buffer). Closing the stream interrupts that fiber, which runs the
producer's cleanup.
