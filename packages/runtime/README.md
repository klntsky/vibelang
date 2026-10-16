# @effectscript/runtime

The EffectScript runtime contains the one-shot continuation kernel and the
helpers that `esc` emits. It runs on Effect 4's fibers, scopes, and services.
Generated JavaScript imports `effect` and `@effectscript/runtime`; applications
install both packages. The runtime requires **effect@4.0.5 exactly** because the
kernel accesses that version's fiber continuation stack.

## Build and test

From this directory:

```sh
bun install
bun run build
bun run test
```

The build uses TypeScript to write ES2022 JavaScript modules, declarations,
JavaScript source maps, and declaration maps to `dist/`. Node 26 and Bun 1.4
load the package's JavaScript export. TypeScript resolves the same export to
`dist/index.d.ts`. The package includes `dist/` and this README; build it before
packing or installing a local copy. Generated files under `dist/` are ignored.

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
import { Effect, $bind, $program } from "@effectscript/runtime"

const answer = $program(() => $bind(42))
console.log(await Effect.runPromise(answer))
```

Applications normally write `.es` programs and let the compiler select these
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
| `$finally` | Run cleanup on every exit and retain cleanup failures | D15, D46 |
| `$region` | Create a block-owned scope without adding it to the body's Context | D14, D47 |
| `$using` | Provide a scope to an acquisition and register `Symbol.dispose` cleanup | D14, D25 |
| `$fork` | Fork into an explicit block scope, or require a scope from the caller | D19, D47 |
| `fork` | Expose `Effect.forkScoped` to TypeScript and prelude type positions | D4, D19 |
| `$Context` | Expose Effect's Context namespace for generated service references | D1, D40 |
| `$service` | Create the Context key base merged with a generated service interface | D1, D45 |
| `$reference` | Create the `Context.Reference` base of a service class that declares `static readonly default` | D40, D45 |
| `$control` | Register one kernel operation per generated control method | D2 |
| `Control` | Mark an abstract control declaration in the prelude | D2, D4 |
| `wire` | Compose layers in the dependent-first order computed by the compiler | D27 |
| `Continuation` (type) | Type the clause-local, one-shot continuation token | D16, D17 |
| `handle` | Install a deep handler and optional return clause | D16 |
| `resume` | Resume a captured continuation with a value | D16 |
| `resumeWith` | Resume with an injected Effect that requires no services | D17 |
| `abandon` | Consume a continuation and run its cleanup without resuming its body | D16 |
| `$tailResume` | Return a tail-resume request without adding a clause frame | D18 |
| `$tailResumeWith` | Return a tail-resume request with an injected Effect | D17, D18 |
| `YieldOf` (type) | Track a producer's unhandled yield requirement | D26 |
| `Producer` | Hold a cold producer body and its element, failure, and requirement types | D26 |
| `$produce` | Turn a generated producer body into a cold `Producer` | D26 |
| `$yield` | Perform the built-in yield operation | D26 |
| `$forOf` | Handle producer yields and lower loop completion records | D26 |
| `$forStream` | Consume an Effect Stream and stop pulling when the loop exits | D26 |
| `$toStream` | Adapt a producer to a Stream with fiber-owned cleanup | D26 |
| `$trusted` | Turn listed exception tags or class names into expected failures | D23 |
| `Effect` | Supply the prelude Effect namespace | D4 |
| `Layer` | Supply the prelude Layer namespace | D4 |
| `Schema` | Supply the prelude Schema namespace | D4 |
| `Scope` | Supply the prelude Scope namespace | D48 |
| `TaggedError` | Expose Effect's `Data.TaggedError` | D3 |
| `pipe` | Supply the prelude pipeline function | D8 |
| `Exit` | Expose Effect completion values to TypeScript adapters | D43 |
| `runMain` | Run a prototype entry program and return its Exit | D38 |
| `suspend` | Bridge callback registration with a resumer that reports a lost race | D43 |
| `Resumer` (type) | Type `resume` and the callback registration state | D43 |
| `handoff` | Bridge sources that acknowledge cancellation before releasing late values | D43 |
| `HandoffProtocolViolation` | Report duplicate delivery or delivery after acknowledged shutdown | D43 |

The kernel's implementation modules are private package files. Their source
remains in `src/kernel/` so the original law and trace suites can test the
continuation machine directly. Generated code uses the package root. Tests and
instrumentation import the kernel module itself from
`@effectscript/runtime/kernel`, for example its `hooks`, which report each
capture and resumption; the compiler's acceptance case `stack-depth` uses them
to check that loops run in constant stack.

## Prototype compatibility

The extraction preserves the prototype's helper names and execution behavior.
The compiler orders `wire` arguments with dependents before providers; the
runtime itself does not inspect or sort Layer dependencies. `$trusted` currently
accepts exception tag or constructor-name strings; it does not resolve exception
constructors. The existing `Continuation<X, A, E>` type parameters are retained.

The producer-to-Stream adapter retains the prototype's one-element buffer. The
stricter zero-capacity handoff described by D26 remains follow-up work. Direct
`$forOf` consumption uses a tail-resumptive handler and closes the producer when
a completion record exits the loop. Stream closure interrupts its producer
fiber and runs cleanup.
