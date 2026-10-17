## Part V. Decisions

### 29. Decisions

```text
settled before this document      1-7     the owner's original decisions
decisions since                  D1-D124 the owner's calls and reported build defaults; gaps reserved
rules of the runtime and compiler P1-P22  adopted (D29), revised where the 2026-10-16 reviews broke them
end-to-end build calls          D61-D124 recorded below where reported; unreported numbers stay unassigned
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

D57 supplies the partial-annotation spelling required by decision 2: explicit `infer Name` slots.

#### Decisions since

"Will" marks the owner's decisions; every other row is a default. **Controversial** marks a decision the owner is likely to revisit after using the language.

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
| D9 | A function value where a plain function is expected passes its plain-JavaScript form, or EK4301 | Fable review, finding 3 | 13, 16 |
| D10 | Plain JavaScript when a function performs no Effect operation and E is never; source order kept; pure recursion on the JavaScript stack | Codex review R4; settled decision 4; the owner's `add` feedback | 15 |
| D11 | Higher-order functions compile to frames in v0 | Fable review, finding 5 | 15, 16 |
| D12 | A callback type without a contract accepts any contract; unchanged by D57 | Settled decision 2 forbids reading it as `never` | 16 |
| D13 | `catch` handles a cause only when it holds no defect and no interruption | Codex review R1; K14a-K14c | 3, 17, 26 |
| D14 | Only `using` acquisitions belong to the block | Codex review R8 | 6, 23, 26 |
| D15 | A failure during cleanup is never dropped | Codex review R2b; K12a-K12e | 17, 21 |
| D16 | v0 exposes `handle`, `resume`, `resumeWith`, `abandon`; `transfer` stays in the standard library | Codex review R5; K13 now bounds a transfer by its fiber, and G3 remains | 9, 21 |
| D17 | `resumeWith` runs code whose R is `never` | Codex review R7 | 9, 20 |
| D18 | A clause whose last step is `resume` compiles to the tail-resume shape | Both reviews (R10, Fable 1); K11a-K11d | 20, 26 |
| D19 | `fork` is block-owned | Fable review, finding 9 | 7, 26 |
| D20 | A TypeScript function whose R holds a control operation is EK4202 in v0 | Codex review R6 | 9, 16 |
| D21 | Each `Effect.provide` builds its layers once; provide at `main`; `ManagedRuntime` for several entry points | Fable review, finding 6; conformance test D05 | 4 |
| D22 | Effect's own packages are trusted, with the functions and values they return | D6; `Option` and `Result` are data in Effect 4 | 10 |
| D23 | A trusted contract is the `@effeck trusted` JSDoc tag, with `@throws` | TypeScript syntax only; the compiler's metadata channel | 10, 27 |
| D24 | A call runs only `Effect` itself; subtypes are data until called; `new` never runs | `Config`, `Exit`, keys, and errors extend `Effect` | 13 |
| D25 | `using` adds no masking policy | K15a-K15c | 6, 21 |
| D26 | Producers: `for...of` by type; a producer is a `Stream` to TypeScript; the adapter runs it on its own fiber behind a zero-capacity handoff | Stream operators pull on other fibers; continuations are fiber-affine; a one-element buffer runs ahead (prototype finding 26) | 8, 26 |
| D27 | `wire` is an intrinsic with a bounded contract (EK3201-EK3204) | Layer types carry outputs and inputs | 4 |
| D28 | A `catch` over several expected failures receives the first | K14a | 3, 17 |
| D29 | P1-P22 are the spec's rules, as revised below | The reviews' verified findings | 29 |
| D30 | Installation: `effeck` (`effeck`) and `effeck-runtime`; `effect` 4.0.5 exactly | Mirrors `typescript@7` packaging | 28 |
| D31 | Builds: `effeck --watch`; `effeck-vite` for Vite | One checker for CLI, editor, and bundler | 28 |
| D32 | The editor: `effeck --lsp --stdio`; the `effeck.effeck` extension | The fork's binary already serves LSP | 28 |
| D33 | v0 editor features: hover, completions, quick fixes, four refactors | The diagnostics with mechanical fixes | 28 |
| D34 | Diagnostics: errors, except the policies EK2004 (a warning) and EK4403; `compilerOptions.effeck` | Soundness errors cannot be lowered | 28 |
| D35 | One owner per diagnostic: `effeck` for types and language, Oxlint for lint | tsgolint embeds TypeScript's checker | 28 |
| D36 | Debugging through composed source maps; spans for logical stacks | The frame invariant keeps callers off the JavaScript stack | 28 |
| D37 | `catch` never sees an unlisted exception; a dead `catch` is the warning EK2004 | Coverage audit X2; prototype finding 16 | 3, 10 |
| D38 | Runners only where no fiber runs the code; EK1005 elsewhere | Coverage audit X4 | 13 |
| D39 | A function-or-Effect parameter: a function if plain, a program if it can pause | Coverage audit X9 | 13 |
| D40 | Service statics: `layer`, `make`; `static readonly default` makes a `Context.Reference` | Coverage audit X12 | 4 |
| D41 | `S.member(…)` always adds `S` to R; service members win over the key's members; EK3006 for a static that shares a member's name | Coverage audit X13; prototype findings 3, 4 | 4, 13 |
| D42 | Workflow bodies are ordinary programs: interrupted on durable suspension, rerun on replay | Coverage audit X17; `workflow/Workflow.ts:695-780, 1031-1036` | 23 |
| D43 | `Effect.callback` is an explicit boundary; its `resume` takes an Effect, and `.effeck` passes an `Exit` | Coverage audit X11; `Effect.ts` | 6, 10 |
| D44 | "The expected type" is the declared, uninstantiated parameter type of the resolved overload | Prototype finding 1: the contextual type is inferred from the lambda itself | 13 |
| D45 | A service compiles to an interface plus a `Context.Key` class, with an optional brand | Prototype finding 19: Effect's class form breaks `implements` (TS2720) | 4, 27 |
| D46 | `finally` lowers to `Effect.onExit`; `throw` to `return yield* Effect.fail(e)`; E is the type of causes made only of expected failures | Prototype findings 11, 12, 14 | 14, 17, 26 |
| D47 | A block that declares `using` or calls `fork` is a Scope region | Prototype finding 18 | 23, 26 |
| D48 | `Scope` joins the prelude, as module and type | Contracts name it constantly; Effect 4's type is `Scope.Scope` | 1 |
| D49 | Arrays and records of programs convert where Effects are expected | Prototype finding 5 | 5, 13 |
| D50 | `Continuation<V, E, H>`: `resume` returns the `handle`'s result type | Prototype finding 24 | 9 |
| D51 | Plain calls at a module's top level are ordinary calls; `@throws {T}` names a class | Prototype finding 21 | 10 |
| D52 | E and R of recursion by a fixpoint inside the component, never a whole-program pass (EK2102 for A); A widens; definite assignment on the source | Prototype findings 7, 10, 13; the single-pass probe `isEven`/`isOdd` | 14, 26, 27 |
| D53 | Diagnostic codes follow the prototype | Prototype diagnostics | 28 |
| D54 | Inside a plain-JavaScript function, `try`, `catch`, and `finally` are JavaScript's own; `catch` rethrows anything unexpected | D10 with D5: nothing in the function can pause | 15 |
| D55 | A method implementing a plain-function member (`[Equal.symbol]`, `[Hash.symbol]`) compiles to a plain JavaScript method, or EK4301 | The member's declared type is not an Effect | 15 |
| D56 | A lambda's form (plain, Effect function, program) is decided against each overload candidate's declared parameter type, and the first candidate that accepts it wins; never by the lambda's own inferred type. D10 eligibility is computed on demand | Single-pass experiment: 0 disagreements in 46 files; 6 probe lambdas with two self-consistent lowerings | 13, 15, 27 |
| D57 | **Controversial.** Partial annotations use `infer Name` per channel in a function return annotation with a body; omitted slots are EK2003. D12 stands | TypeScript 7 parses `infer`; explicit slots distinguish inference from `never` | 2, 14, 16, 28 |
| D58 | **Controversial.** Keep EffectScript, `esc`, `.es`, and `@effectscript`; superseded by D125 | Another compile-to-JavaScript language uses the name | 28 |
| D59 | Draft upstream issues and proposals in `docs/upstream/`; status: drafted, not filed | Filing speaks for the owner in a third-party project and needs his go-ahead | 24, 25, 30 |
| D60 | **Will.** One private repository, `github.com/roninjin10/effectscript`; the compiler lives in `typescript/` | Spec and implementations change together; the imported TypeScript tree omits 349 MB of baselines | 27, 28 |

#### Decisions from the end-to-end build (2026-10-16)

The following rows record build defaults, not new language decisions. Lane U recorded D97-D99 from F7's compiler notes because its inbox report was absent. The decision log groups known gaps by area and distinguishes landing snapshots from later fixes. D114-D119 have no reported decisions.

| # | Decision | Reason and evidence | Chapters |
| --- | --- | --- | --- |
| D61 | **Controversial.** An EK diagnostic is a TypeScript diagnostic with source `EK`, on upstream's content-mapper path | One diagnostic path for CLI and LSP | 28 |
| D62 | The prelude is the runtime's exports | Lazy imports preserve files that use no prelude names | 1, 27 |
| D63 | A `.effeck` file imports `./x.effeck` or `./x.ts` by source name with no flag (no TS5097); specifiers become `.js` in JavaScript and `.d.ts` output | Source names for checking, output names for JavaScript | 10, 27, 28 |
| D64 | A failing compiler case passes when its exact set of error codes matches (TypeScript codes count, warnings do not); a runnable case compares only its `@@result` line, so programs may log | Exact codes detect regressions while result markers allow logging | 10, 27, 28 |
| D65 | A call from a `.effeck` file to a plain function runs inline as `f$direct(a)` everywhere, including a module's top level, because it computes a value and runs no program | Plain computation runs no program | 13, 15 |
| D66 | An overload candidate that asks for a plain function rejects an `.effeck` function value that can pause, as it rejects such a lambda (D56 extended to function values); a TypeScript function that returns an Effect passes as data; when every candidate rejects, EK4301 reports against the first candidate's form | Named values and lambdas need the same callback safety | 13, 16 |
| D67 | **Controversial.** A function with a plain return annotation keeps that type and compiles to one plain function; only unannotated or Effect-contract functions get the `$direct` plus `Effect.sync` pair, whose TypeScript type is `(…) => Effect<A, never, never>` (chapter 15's one calling convention) | Plain annotations support JavaScript protocols | 10, 14, 15 |
| D68 | **Controversial.** D12 (a callback parameter without a contract accepts any contract) applies to declarations, methods, and function expressions that are not call arguments; the added type parameters are named E and R (then E2, R2) and appear in `.d.ts` output | Missing callback channels must not mean never | 16, 27 |
| D69 | A program lambda lowers to `Effect.gen(function* …)`, a converted function value to `Effect.suspend(v)`, and an array or record of programs built at run time to `$programs` | Conversions must preserve cold execution | 5, 13, 27 |
| D70 | **Controversial.** A `return`, `break`, or `continue` that leaves a `finally` block is a new error, EK2006; in JavaScript such a jump discards the failure in flight, and compiling `finally` to Effect cleanup cannot honor it | Effect cleanup cannot honor jumps that discard failures | 17, 28 |
| D71 | EK2102 also covers a `catch` variable or return type typed before its cycle's E and R are known; the fix is a written contract | The checker cannot revise cached recursive types | 14, 28 |
| D72 | EK2102 applies only to functions that run Effects; a pure recursive function keeps TypeScript's TS7023, which a plain annotation fixes (consistent with D67) | Pure recursion needs no Effect channel analysis | 14, 15 |
| D73 | A call to a function with a complete written contract adds the contract's E and R, not what its body does | Callers rely on the public contract | 14 |
| D74 | Methods are exempt from EK2003 for a plain annotation, because a method implementing a plain-function member is declared that way (D55) | Plain protocol members need plain annotations | 14, 15 |
| D75 | The `tsc/cmd/tsc` binary identifies as Effeck; there is no separate cmd/effeck | One upstream entry-point seam | 27, 28 |
| D76 | **Controversial.** The editor serves `.ts` only when the built-in TypeScript extension is disabled, or with Cursor's useTsgo | Disabling validation also hides Effeck diagnostics; amends D32 | 28 |
| D77 | **Controversial.** Hover shows a running call's success value and a plain helper's direct and TypeScript-facing signatures; `.ts` hover stays unchanged | Explain both call semantics and helper ABI | 15, 28 |
| D78 | Prelude names and service members complete in scope; local declarations shadow prelude names | Discoverability without explicit imports | 1, 4, 28 |
| D79 | Reuse TypeScript TextMate grammar; resolve effeck from setting, project, then PATH; require workspace trust | Keep theme support and project compiler ownership | 28 |
| D80 | The runtime checks Effect's version by importing effect's package metadata with a static JSON import attribute; works in Node, Bun, and bundlers | JSON attributes work in Node, Bun, and bundlers | 27, 28 |
| D81 | The runtime exports `suspend`, `handoff`, `Resumer`, and `HandoffProtocolViolation` for TypeScript callback adapters (D43) | Resource adapters need cancellation and lost-race reporting | 6, 21 |
| D82 | The extraction kept the prototype helpers' behavior and continuation parameter order; discrepancies with the spec are documented in packages/runtime/README.md, not fixed silently | Extraction must preserve compiler compatibility | 26, 27 |
| D83 | Setup writes absolute `file:` dependencies on the repository's packages into new projects instead of `bun link`; nothing registers globally and nothing is published, so a project keeps working only while the checkout stays where it is | Local setup needs no registration or publication | 28 |
| D84-D89 | No decisions reported in the build inbox | Leave these numbers unassigned | |
| D90 | The member-call rule (D41) is structural: it applies to any Context key whose service type declares the member, which covers `.effeck` services, TypeScript `Context.Service` classes, and Effect's `FileSystem` and `Path` | Context keys and Effeck services share call behavior | 4, 13 |
| D91 | **Controversial.** A control-operation call lowers like a service call, `yield* (yield* Approval).approve(x)`, as the prototype did (amending the original Chapter 13 lowering); `handle` provides an implementation that performs the kernel operations | Preserves the prototype implementation pending F7 | 9, 13 |
| D92 | A method in a `.effeck` file becomes an Effect when its body runs or looks up an Effect, or when its expected type returns one; it stays a method whose body is `return Effect.gen({ self: this }, function* () { … })` | Generators must retain the method receiver | 15, 27 |
| D93 | **Controversial.** Service keys use package, module, and name; defaults require `static readonly`; EK2101 requires all three Effect channels; wire emits consumers first | Stable service identity and bounded provider analysis | 4, 14 |
| D94 | Effect acquisitions receive the block Scope; other initializers need Disposable, null, or undefined; Disposable results also run Symbol.dispose | Effect acquisitions and ordinary Disposables have different ownership | 6 |
| D95 | In an Effect function, `for (using x of values)` makes each iteration's body a Scope region; the value is taken first, and `using` or `fork` inside the body join that region | Cleanup belongs to the iteration that owns it | 6, 7 |
| D96 | **Controversial.** A `fork` belongs to the innermost block around it; an arrow function with an expression body has no block, so `() => fork(p)` is `Effect.forkScoped` and keeps `Scope` in R | An expression body has no owning block | 7 |
| D97 | **Controversial.** Continuation H is the program's A, or never with onReturn; handle unions program and clause results. Recorded from the compiler notes | TypeScript fixes H before checking onReturn; mixed early returns can narrow resume too far | 9 |
| D98 | Producers return lazy Streams; direct loops use the consumer fiber, Stream operators use a separate fiber and a handoff with no buffer. Recorded from the compiler notes | Lazy interoperability, cleanup, and bounded stack without running ahead | 8, 26 |
| D99 | EK4201 checks known fiber boundaries and control-requiring producer arguments; EK4202 checks foreign callees' declared control requirements. Recorded from the compiler notes | Diagnose unreachable handlers; runtime enforces fiber ownership | 9, 16 |
| D100 | Code no fiber runs is plain JavaScript for EK4401 too: a module's top level and the plain callbacks it passes to TypeScript functions (`describe` bodies, route handlers), the same places D38 allows runners | Framework callbacks run outside a fiber | 10, 13 |
| D101 | **Controversial.** Only built-ins that parse outside text declare expected failures (`JSON.parse` with `SyntaxError`, `decodeURI` and `decodeURIComponent` with `URIError`); every other built-in's exceptions are defects | Unlisted built-in exceptions remain defects | 10, 26 |
| D102 | **Controversial.** Original platform-import restriction; import half superseded by D108; globals and allowed web standards retained | Platform work needs typed, replaceable Effect services | 10, 11 |
| D103 | EK1001 covers any top-level call that would run a program, including service member calls and Effect-value calls, reported once at the outermost call | One outer call identifies attempted program execution | 13, 28 |
| D104 | Compiled generators are named after their function, variable, or property (an object method `{ *name() {} }.name` when the body refers to that name); `tracing: "exported"` spans exported functions, `"all"` every named lowered function plus methods through `Effect.withSpan("Class.method")`; anonymous lambdas get no span | Names and spans identify source failures | 19, 26, 28 |
| D105 | Vite compiles a project through one shared effeck subprocess per source revision, with cached output; amends D31 | Whole-project checking keeps caller contracts consistent | 28 |
| D106 | After an edit the plugin invalidates every loaded `.effeck` module and its transitive importers, because callers compile differently when a contract changes | Contract changes can change caller lowering | 28 |
| D107 | **Controversial.** Sources must sit under Vite's root; the plugin overrides emit paths and source roots there so output lookup and source maps are predictable | One source root makes output lookup predictable | 28 |
| D108 | EK4402 reports platform execution, including calls and property reads; imports and passing values are allowed. Supersedes the import half of D102 | Effect adapters must accept host function values | 10, 11, 28 |
| D109 | An expected callback or program asks for the Effect form only when its type is effect's own `Effect` interface, or a type parameter constrained to it; subtypes such as `Config` are data (D24), so `Config.orElse`'s fallback stays plain | D24 keeps structured subtypes as data | 13, 16 |
| D110 | The `pipe` look-ahead (D8) reads the first stage's callee signatures, or a function value's own signatures; it runs only when the first argument can be a program, and never checks the stage early | Reading signatures avoids circular inference | 5, 13 |
| D111 | The D49 conversion covers any iterable (`Set`, a `Map`'s values) through its iterated type; an iterable never converts as a record, matching `Effect.all` at run time | Matches Effect.all at run time | 5, 13 |
| D112 | A callback whose expected return mixes plain and Effect (`boolean \| Effect<…>`) is plain when D10 allows and an Effect function otherwise; the same holds wherever both forms fit | D10 selects the eligible form without Effect work | 13, 16 |
| D113 | A lambda with a contract that takes the plain form has type `(…) => A`; where only a plain function fits, inference from its annotation starts from A | Plain inference needs A rather than its Effect wrapper | 14, 16 |
| D120 | Local runtime dependencies use setup's tarball; refresh all three packages with bun update; examples omit bun.lock | Avoid a second Effect copy that prevents child interruption; amends D83 | 27, 28 |
| D121 | **Controversial.** Effect trust follows re-exports and returned values, including assertions from @effect/vitest | Tests need usable assertions; direct Vitest imports and local TypeScript retain boundary checks | 10, 16 |
| D122 | Every runner checks all services it does not supply; it.effect supplies Scope and its layer services | Diagnose missing services before execution | 4, 28 |
| D123 | Declaration prelude imports use free aliases when names are already bound | Avoid duplicate identifiers in declaration consumers | 27 |
| D124 | Setup relinks effeck launchers pointing into another checkout or at missing files | Keep the launcher usable after workspace changes | 28 |
| D125 | **Controversial.** The language is Effeck: binary `effeck`, files `.effeck`, codes `EK`, unscoped npm packages `effeck`, `effeck-runtime`, `effeck-vite` | The `@effectscript` npm scope belongs to another language | 28 |
| D126 | `error NotFound { id: string }` declares a tagged error class with readonly fields | Fewer tokens for the most common declaration | 3 |
| D127 | **Controversial.** Effect types are written `Result!Error!Dependencies`; an empty slot is never, and left-out dependencies are inferred on a function with a body | Fewer tokens than `Effect<A, E, R>` in every contract | 14 |
| D128 | `service` and `control` declare abstract classes whose bodiless members are abstract; an implemented member is EK2103 | Fewer tokens than `abstract class` and `abstract` on every member | 4, 9 |
| D146 | Every export of `effect`'s root is in scope; JavaScript's own globals win, host globals do not | Imports from `effect` were 5.9% of the docs' tokens | 1, 27 |
| D147 | **Controversial.** A call is passed by name where a parameter expects a program: `fork(fetchReport())` | Thunks around single calls were 2.6% of the docs' tokens | 5, 13 |
| D148 | **Controversial.** `sync`, `try`, and `callback` are boundary hatches whose arguments are plain TypeScript | One visible escape hatch replaces five Effect constructors and their lambda rules | 10, 28 |
| D149 | `try(…)` parses as a call in `.effeck`; Effect's `{ try, catch }` object is EK4404 | A try statement needs a block, so `try (` is unambiguous | 10, 28 |
| D150 | `runPromise`, `runFork`, `runSync`, `runCallback` are in the prelude, for TypeScript and hatch bodies | The way back in; EK1005 still applies where a fiber runs | 10, 13 |
| D151 | `sleep`, `log`, `retry`, `timeout`, `all`, `forEach`, `race`, and seven more Effect functions need no prefix | They were 329 of the 538 `Effect.` member uses in the docs | 1, 27 |

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
| P10 | Control operations cannot cross fibers. **Revised**: the runtime is the guarantee (`HandlerNotOnThisFiber`); the fiber-behavior table is the diagnostic (EK4201), and TypeScript functions with a control operation in R are rejected (EK4202) | T7b, T8, probes; review probe R6 | 9, 16 |
| P11 | The frame invariant; source regions compile to Effect frames, never JavaScript `try`, in functions that compile to frames | F1-F3 | 26 |
| P12 | **Revised** as D10: plain JavaScript only with no Effect operation and E = never; pure recursion on the JavaScript stack; `pure` permits inlining only | F9, T6a; review probe R4 | 15 |
| P13 | The kernel bounds synchronous wake-up depth | F10a, K10a, K10b | 24 |
| P14 | A callback type without a contract is effect-polymorphic (D12); D57 leaves this rule intact | Settled decision 2 | 16 |
| P15 | Effect 4 only | Research §10 | 10 |
| P16 | Fork `microsoft/TypeScript` `tsc/` at a published nightly; in-tree package; no shims | Compiler research | 27 |
| P17 | A declaration without a body states its full contract; a bare return type there is an error, never `never` | Settled decision 2 | 14 |
| P18 | **Revised** as D17: `resumeWith` may raise only the operation's declared E, and its R is `never` | Settled decision 3; review probe R7 | 9 |
| P19 | A finalizer's control operations must be handled inside the finalizer | I06, I11 | 17, 23 |
| P20 | Programs start only at runners; calls never create fibers; a module's top level cannot run a program (EK1001). **Revised**: runners are called only where no fiber runs the code (EK1005, D38) | Fable review | 13, 23 |
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

#### Closed calls for the end-to-end build

| Former item | Decision | Result |
| --- | --- | --- |
| O7 | D57 (**controversial**) | `infer Name` marks an inferred channel; all three return-annotation slots are required; D12 stands |
| O8 | D59 | Upstream issues and proposals are drafted in `docs/upstream/`, not filed |
| O9 | D58, superseded by D125 (**controversial**) | Effeck: `effeck`, `.effeck`, `EK` codes, and unscoped npm packages |
| Remote and repository layout | D60 | The one repository is private at `github.com/roninjin10/effectscript`, with the compiler in `typescript/` |

No language or repository decision waits on the owner. D59's filing boundary remains: these drafts are ready to file, and none is filed without his go-ahead. The build lanes land locally; the orchestrator handles any push.
