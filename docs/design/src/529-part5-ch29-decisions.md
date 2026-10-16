## Part V. Decisions

### 29. Decisions

```text
settled before this document      1-7     the owner's original decisions
decisions since                   D1-D56  the owner's calls and the defaults that complete the design
rules of the runtime and compiler P1-P22  adopted (D29), revised where the 2026-10-16 reviews broke them
still the owner's call            O7, O8, O9, and pushing to a remote
```

Every rule in this document traces to one of these. The log with full reasons is `docs/design/DECISIONS.md`; a default is a decision made to complete the design, and the owner can overturn any default.

#### Settled before this document

These come from the earlier conversation and are not reopened.

1. Every function participates in one computation model. No `effect function` keyword and no `effect(...)` wrapper. Ordinary calls compose computations and expose successful results without a marker at each call. A way to construct, store, pass, retry, and concurrently compose programs as values is required (D7 spells it).
2. Explicit contracts are written `Effect<A, E, R>` as a return annotation. The annotation describes the whole computation, and the return expression is checked against A. Most code infers. In the owner's words: "Success, errors, and requirements must each support inference or explicit declaration. We have not finalized the shorthand for partially annotating those channels. Preserve that requirement without inventing a premature spelling or silently confusing inference with `never`. Use the same contract model for function types, callbacks, service methods, and exported declarations."
3. `throw` contributes to the owning computation's inferred E. An explicit E constrains what escapes. `Effect<A, never, R>` may handle expected failures internally but rejects escaping ones. Expected failures, defects, and interruption stay distinct.
4. Optimization preserves computation semantics. E = never and R = never do not prove purity or synchronous completion. Constructing a cold program never executes its work.
5. Dependency injection is core and is designed with the continuation semantics: identity, contracts, inferred and declared requirements, acquisition, composition, missing, ambiguous, and cyclic providers, acquisition failures, residual requirements, scoped sharing and release, test overrides, Context across suspension and children, and interoperability with Effect Context keys and Layers. No second DI container.
6. New syntax must earn its place. Ordinary function syntax and the `Effect<A, E, R>` annotation reuse TypeScript syntax.
7. Broad npm interoperability is not an MVP constraint. Interop with TypeScript code that already uses Effect is.

#### Decisions since

"Will" marks the owner's decisions; every other row is a default.

| # | Decision | Reason and evidence | Chapters |
| --- | --- | --- | --- |
| D1 | Services are abstract classes whose instance members are all abstract; nominal; keys derived and unique; no `extends`, `new`, `instanceof`, or generics; emitted as D45 says | Uses no valid TypeScript meaning (TS2339, TS2348); structurally equal classes would alias (probe); rare in real code | 4 |
| D2 | Control operations are abstract classes that extend the global `Control` | One declaration form for services and operations; no keyword | 9 |
| D3 | Errors use the global `TaggedError` (Will) | Effect's `Data.TaggedError`; `Schema.TaggedError` for serialized errors | 3 |
| D4 | The prelude: `Effect`, `Schema`, `Layer`, `TaggedError`, `Control`, and the built-ins; imports shadow it | What 77-81%, 39-44%, and 34-44% of Effect files import | 1 |
| D5 | Calls to non-Effect code need a declared trust level (Will) | Foreign code may throw; platform APIs are replaced; clock reads go through services | 10 |
| D6 | The standard library is Effect's (Will) | Effect ships the modules and the platform implementations | 11 |
| D7 | Program values are functions with no arguments | No new syntax; `() =>` already means "not yet" | 5, 13 |
| D8 | No `\|>` in v0; the `pipe(program, …stages)` built-in stays | Calls run before a pipe sees them; Stage 2 since 2021; 1.1% and 2.6% of pipes keep two stages | 5 |
| D9 | A function value where a plain function is expected passes its plain-JavaScript form, or ES4301 | Fable review, finding 3 | 13, 16 |
| D10 | Plain JavaScript when a function performs no Effect operation and E is never; source order kept; pure recursion on the JavaScript stack | Codex review R4; settled decision 4; the owner's `add` feedback | 15 |
| D11 | Higher-order functions compile to frames in v0 | Fable review, finding 5 | 15, 16 |
| D12 | A callback type without a contract accepts any contract; listed with O7 | Settled decision 2 forbids reading it as `never` | 16 |
| D13 | `catch` handles a cause only when it holds no defect and no interruption | Codex review R1; K14a-K14c | 3, 17, 26 |
| D14 | Only `using` acquisitions belong to the block | Codex review R8 | 6, 23, 26 |
| D15 | A failure during cleanup is never dropped | Codex review R2b; K12a-K12e | 17, 21 |
| D16 | v0 exposes `handle`, `resume`, `resumeWith`, `abandon`; `transfer` stays in the standard library | Codex review R5; K13 now bounds a transfer by its fiber, and G3 remains | 9, 21 |
| D17 | `resumeWith` runs code whose R is `never` | Codex review R7 | 9, 20 |
| D18 | A clause whose last step is `resume` compiles to the tail-resume shape | Both reviews (R10, Fable 1); K11a-K11d | 20, 26 |
| D19 | `fork` is block-owned | Fable review, finding 9 | 7, 26 |
| D20 | A TypeScript function whose R holds a control operation is ES4202 in v0 | Codex review R6 | 9, 16 |
| D21 | Each `Effect.provide` builds its layers once; provide at `main`; `ManagedRuntime` for several entry points | Fable review, finding 6; conformance test D05 | 4 |
| D22 | Effect's own packages are trusted, with the functions and values they return | D6; `Option` and `Result` are data in Effect 4 | 10 |
| D23 | A trusted contract is the `@effectscript trusted` JSDoc tag, with `@throws` | TypeScript syntax only; the compiler's metadata channel | 10, 27 |
| D24 | A call runs only `Effect` itself; subtypes are data until called; `new` never runs | `Config`, `Exit`, keys, and errors extend `Effect` | 13 |
| D25 | `using` adds no masking policy | K15a-K15c | 6, 21 |
| D26 | Producers: `for...of` by type; a producer is a `Stream` to TypeScript; the adapter runs it on its own fiber behind a zero-capacity handoff | Stream operators pull on other fibers; continuations are fiber-affine; a one-element buffer runs ahead (prototype finding 26) | 8, 26 |
| D27 | `wire` is an intrinsic with a bounded contract (ES3201-ES3204) | Layer types carry outputs and inputs | 4 |
| D28 | A `catch` over several expected failures receives the first | K14a | 3, 17 |
| D29 | P1-P22 are the spec's rules, as revised below | The reviews' verified findings | 29 |
| D30 | Installation: `@effectscript/compiler` (`esc`) and `@effectscript/runtime`; `effect` 4.0.5 exactly | Mirrors `typescript@7` packaging | 28 |
| D31 | Builds: `esc --watch`; `@effectscript/vite` for Vite | One checker for CLI, editor, and bundler | 28 |
| D32 | The editor: `esc --lsp --stdio`; the `effectscript.effectscript` extension | The fork's binary already serves LSP | 28 |
| D33 | v0 editor features: hover, completions, quick fixes, four refactors | The diagnostics with mechanical fixes | 28 |
| D34 | Diagnostics: errors, except the policies ES2004 (a warning) and ES4403; `compilerOptions.effectscript` | Soundness errors cannot be lowered | 28 |
| D35 | One owner per diagnostic: `esc` for types and language, Oxlint for lint | tsgolint embeds TypeScript's checker | 28 |
| D36 | Debugging through composed source maps; spans for logical stacks | The frame invariant keeps callers off the JavaScript stack | 28 |
| D37 | `catch` never sees an unlisted exception; a dead `catch` is the warning ES2004 | Coverage audit X2; prototype finding 16 | 3, 10 |
| D38 | Runners only where no fiber runs the code; ES1005 elsewhere | Coverage audit X4 | 13 |
| D39 | A function-or-Effect parameter: a function if plain, a program if it can pause | Coverage audit X9 | 13 |
| D40 | Service statics: `layer`, `make`; `static readonly default` makes a `Context.Reference` | Coverage audit X12 | 4 |
| D41 | `S.member(…)` always adds `S` to R; service members win over the key's members; ES3006 for a static that shares a member's name | Coverage audit X13; prototype findings 3, 4 | 4, 13 |
| D42 | Workflow bodies are ordinary programs: interrupted on durable suspension, rerun on replay | Coverage audit X17; `workflow/Workflow.ts:695-780, 1031-1036` | 23 |
| D43 | `Effect.callback` is an explicit boundary; its `resume` takes an Effect, and `.es` passes an `Exit` | Coverage audit X11; `Effect.ts` | 6, 10 |
| D44 | "The expected type" is the declared, uninstantiated parameter type of the resolved overload | Prototype finding 1: the contextual type is inferred from the lambda itself | 13 |
| D45 | A service compiles to an interface plus a `Context.Key` class, with an optional brand | Prototype finding 19: Effect's class form breaks `implements` (TS2720) | 4, 27 |
| D46 | `finally` lowers to `Effect.onExit`; `throw` to `return yield* Effect.fail(e)`; E is the type of causes made only of expected failures | Prototype findings 11, 12, 14 | 14, 17, 26 |
| D47 | A block that declares `using` or calls `fork` is a Scope region | Prototype finding 18 | 23, 26 |
| D48 | `Scope` joins the prelude, as module and type | Contracts name it constantly; Effect 4's type is `Scope.Scope` | 1 |
| D49 | Arrays and records of programs convert where Effects are expected | Prototype finding 5 | 5, 13 |
| D50 | `Continuation<V, E, H>`: `resume` returns the `handle`'s result type | Prototype finding 24 | 9 |
| D51 | Plain calls at a module's top level are ordinary calls; `@throws {T}` names a class | Prototype finding 21 | 10 |
| D52 | E and R of recursion by a fixpoint inside the component, never a whole-program pass (ES2102 for A); A widens; definite assignment on the source | Prototype findings 7, 10, 13; the single-pass probe `isEven`/`isOdd` | 14, 26, 27 |
| D53 | Diagnostic codes follow the prototype | Prototype diagnostics | 28 |
| D54 | Inside a plain-JavaScript function, `try`, `catch`, and `finally` are JavaScript's own; `catch` rethrows anything unexpected | D10 with D5: nothing in the function can pause | 15 |
| D55 | A method implementing a plain-function member (`[Equal.symbol]`, `[Hash.symbol]`) compiles to a plain JavaScript method, or ES4301 | The member's declared type is not an Effect | 15 |
| D56 | A lambda's form (plain, Effect function, program) is decided against each overload candidate's declared parameter type, and the first candidate that accepts it wins; never by the lambda's own inferred type. D10 eligibility is computed on demand | Single-pass experiment: 0 disagreements in 46 files; 6 probe lambdas with two self-consistent lowerings | 13, 15, 27 |

#### Rules of the runtime and compiler

Proposed in the first draft as P1-P22 and adopted by D29. A row marked **revised** changed because a review verified a counterexample.

| # | Rule | Evidence | Chapters |
| --- | --- | --- | --- |
| P1 | The machine in Chapter 20 is the specification. Every feature is a rule over it | Chapter 18 | All |
| P2 | Runtime: Effect-native prompts (strategy A). Reference semantics: an explicit machine (strategy B), once G16 runs one law suite on both. **Revised**: the two disagree today on the Context a reinstalled prompt records (R9), so passing tests in both are not agreement | {{testCount}} kernel tests; 20 reference tests; T7a; review probe R9 | 25 |
| P3 | Deep handlers, dynamic selection, clauses run outside their prompt | T9, clause-outside test | 20 |
| P4 | Context is a snapshot across capture; the reinstalled prompt records the resume site's Context | T1, T13, T15 | 22 |
| P5 | Every consumption of a continuation (resume, transfer, abandon) happens on the capturing fiber | T14, T14b, O04, O06 | 21 |
| P6 | A second resume is a defect at the caller; a root wait that loses a race reports it | T2, T5a-T5c | 21 |
| P7 | An owner frame abandons an unresumed continuation. `transfer` (standard library only) moves ownership to the Scope region around the handler, revokes an earlier transfer, and rejects ownership cycles and parallel Scopes. **Revised**: the capturing fiber's exit is a second owner, so a transfer never outlives its fiber (R5) | T3, T4, T10, O01-O08, K13a-K13e | 21 |
| P8 | Abandonment unwinds like interruption and never disturbs real pending interruptions; a masked handler that swallows it is re-raised. **Revised**: every cleanup failure is reported beside pending interruptions, on every abandoning path (R2b) | kernel-edges, A01-A04, I03, I10, I11, K12a-K12e | 20, 23 |
| P9 | Services always use in-place evidence. Control operations use prompt evidence, or in-place evidence when TypeScript supplies an implementation that always resumes; the checker treats them as fiber-bound either way | In-place and override tests; T8 | 4, 9, 16 |
| P10 | Control operations cannot cross fibers. **Revised**: the runtime is the guarantee (`HandlerNotOnThisFiber`); the fiber-behavior table is the diagnostic (ES4201), and TypeScript functions with a control operation in R are rejected (ES4202) | T7b, T8, probes; review probe R6 | 9, 16 |
| P11 | The frame invariant; source regions compile to Effect frames, never JavaScript `try`, in functions that compile to frames | F1-F3 | 26 |
| P12 | **Revised** as D10: plain JavaScript only with no Effect operation and E = never; pure recursion on the JavaScript stack; `pure` permits inlining only | F9, T6a; review probe R4 | 15 |
| P13 | The kernel bounds synchronous wake-up depth | F10a, K10a, K10b | 24 |
| P14 | A callback type without a contract is effect-polymorphic (D12); its spelling is with O7 | Settled decision 2 | 16 |
| P15 | Effect 4 only | Research §10 | 10 |
| P16 | Fork `microsoft/TypeScript` `tsc/` at a published nightly; in-tree package; no shims | Compiler research | 27 |
| P17 | A declaration without a body states its full contract; a bare return type there is an error, never `never` | Settled decision 2 | 14 |
| P18 | **Revised** as D17: `resumeWith` may raise only the operation's declared E, and its R is `never` | Settled decision 3; review probe R7 | 9 |
| P19 | A finalizer's control operations must be handled inside the finalizer | I06, I11 | 17, 23 |
| P20 | Programs start only at runners; calls never create fibers; a module's top level cannot run a program (ES1001). **Revised**: runners are called only where no fiber runs the code (ES1005, D38) | Fable review | 13, 23 |
| P21 | `for...of` over a producer is internal iteration; pulling from outside uses `transfer` | T10, T16 | 8, 26 |
| P22 | **Revised** as D18: a clause whose last action is `resume`, outside any region of the clause, compiles to `return tailResume(k, v)` and iterates in constant stack. The first draft's evidence, T17, covered only a `flatMap`-shaped clause | K11a-K11d; review probe R10 | 20, 26 |

#### Former open questions

| # | Question | Now |
| --- | --- | --- |
| O1 | How are program values spelled? | D7 |
| O2 | Declaration syntax for services and control operations? | D1, D2 |
| O3 | Does v0 expose the handler API? | D16 |
| O4 | Does `function*` mean producer? | D26 |
| O5 | Which owner does `fork` use? | D19 |
| O6 | Is `wire` a compiler intrinsic? | D27 |
| O10 | Does deep pure recursion overflow like JavaScript? | D10 |
| O11 | What does a call to plain TypeScript mean? | D5, D22, D23 |

#### Still the owner's call

| # | Question | Why it waits |
| --- | --- | --- |
| O7 | How are channels partially annotated, and is D12 acceptable until then? | The requirement is settled (decision 2); the spelling is the owner's. Until then, a contract has all three channels or none |
| O8 | File the two Effect issues (G5), and propose a supported prompt primitive to the Effect maintainers (G1)? | Outward-facing |
| O9 | Keep the name? `EffectScript` is taken by an unrelated compile-to-JavaScript language (`github.com/EffectScript/effectscript`, created 2026-02-23) | Must be decided before anything is published; package names (Chapter 28) follow it |
| | Push any repository to a remote? | Outward-facing |
