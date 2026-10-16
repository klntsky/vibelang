> Historical review of an earlier draft. D57-D60 close the former owner calls; the rows below record their outcomes. Kernel fixes are recorded in `experiments/KERNEL-CHANGES.md`.

The proposal does make Effect’s continuation runtime its foundation, and the existing evidence demonstrates real one-shot delimited control. It is not ready to freeze the compiler/checker contract: probes reproduced a stranded resource and lost cleanup defects, while typing and optimization rules remain incomplete. Keep the runtime direction, but resolve the ranked ownership, failure, boundary, and lowering findings before implementing those rules as language guarantees.

Line numbers refer to the 2,732-line `docs/design/effectscript-design.md` reviewed here. Probe identifiers refer to [probes.test.ts](/Users/williamcory/effectscript/experiments/scratch-review-codex/probes.test.ts); these are hand-lowered counterexamples, not tests of an existing compiler.

1. **High: Transfer can leave a continuation permanently unfinalizable.** [Lines 1105, 1997-2000, 2608](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1997). A child can transfer into its inherited parent Scope, finish, and leave a suspended segment whose cleanup cannot run when the parent closes that Scope.

   R5 reproduces this using ordinary `Effect.scoped`, `forkChild`, `transfer`, and `Fiber.join`: the parent gets `ContinuationWrongFiber`, the token remains suspended, and its finalizer never runs. No manual foreign-fiber `Scope.close` or escaping raw `k` is needed. G3 acknowledges cross-fiber closure, but its proposed queue cannot be drained by a capturing fiber that has already exited. This defeats P7’s ownership guarantee.

   **Fix:** For v0, restrict transfer to regions owned and closed by the capturing fiber, and finalize all remaining tokens before that fiber exits. Alternatively keep the capturing fiber alive under an explicit owner until disposal completes. Test inherited scopes, capturer termination, and parallel scope finalizers before exposing unrestricted `Suspended` values. Preserve P5’s affinity rule.

2. **High: Explicit abandonment loses a cleanup defect during interruption.** [Lines 1633-1634, 1926, 2112, 2582](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1926). `abandon(k)` can report only interruption even though the abandoned segment’s finalizer died.

   R2b triggers a real interruption inside a finalizer and then dies with `release-defect`; the result contains only `Interrupt`. The equivalent owner-driven abandonment retains both reasons (R2). In `AbandonCollector.finish`, kernel line 244 returns the pending interruption instead of the collected unwinding result. Existing A02 and I03 test the ingredients separately, missing their composition.

   **Fix:** Preserve and combine the completed cleanup cause with pending real interruptions, removing only the collector’s synthetic abort. Apply the same law to explicit abandon, owner exit, and Scope exit. P8’s “never dropped” claim needs this regression test.

3. **High: The proposed typed catch can swallow defects.** [Lines 431-437, 1623-1634, 1930, 2343-2362](/Users/williamcory/effectscript/docs/design/effectscript-design.md:2343). Lowering source `catch` to `Effect.catch` violates the promise that defects bypass typed recovery when a cause contains both an expected failure and a defect.

   R1 evaluates `Effect.ensuring(Effect.fail("expected"), Effect.die("release-defect"))`: its cause contains `Fail` and `Die`, but wrapping it in `Effect.catch(..., () => Effect.succeed("recovered"))` succeeds. Effect 4’s `findError` selects the first failure; `catchCauseFilter` then replaces the whole cause (`internal/effect.ts:158-165, 2665-2685`).

   **Fix:** Specify compound-cause recovery explicitly. A conservative source catch should pass through any cause containing a defect or interruption; another defensible rule preserves unhandled reasons after recovery. Implement that with cause-aware frames and test mixed causes. This is a language rule requiring extra lowering, not an existing Effect guarantee.

4. **High: `resumeWith` constrains E but leaves R unsound.** [Lines 1064-1075, 1095, 1925, 2040, 2592](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1925). An injected program executes under the capture-time Context, although ordinary requirement inference would discharge its requirements at the resume site.

   For example, inside a clause:

   ```ts
   return Effect.provideService(
     () => resumeWith(k, () => Secret.read()), Secret, secret
   )
   ```

   The surrounding provider cannot satisfy the injected lookup: R7 fails with `Service not found`. This is the requirement-channel counterpart of the previously fixed undeclared-error injection.

   **Fix:** Initially require injected programs to have R = `never`, or permit only requirements declared by the operation and guaranteed at its perform site. Define the complete typing judgment for `handle`, continuation results, clause E/R, `onReturn`, and `resumeWith`; test positive and negative cases in a checker prototype. Revise P18 before coding inference.

5. **High: A combinator table cannot enforce the claimed foreign control boundary.** [Lines 1147-1170, 1591-1599, 1923, 2521, 2607](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1591). An imported function can internally fork a control operation without taking any program argument for the table to inspect.

   R6 wraps an operation in `Effect.timeout` inside a zero-argument TypeScript function; an outer handler gets `HandlerNotOnThisFiber`. Its public contract can still be `() => Effect<number, TimeoutError, Approval>`. Ordinary E/R plus the fallback control class `suspends` does not distinguish this function from a same-fiber operation. Similarly, E/R alone cannot reveal that a foreign resource’s finalizer performs an outward control operation.

   **Fix:** Carry transitive execution-placement and finalizer constraints through signatures and aliases; conservatively reject unknown foreign computations with unresolved control requirements. Parameter-sensitive annotations are needed for dynamic concurrency. Expand G2 beyond testing exports that accept programs, and qualify “seamless” interop accordingly. P10 and P19 need this boundary contract.

6. **High: The direct-call eligibility rule removes observable interruption points.** [Lines 1523-1538, 2586, 2612](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1523). P12 permits finite trusted `Effect.sync` operations to become plain JavaScript even though those operations create machine boundaries.

   R4 compares an Effect operation that requests interruption followed by `events.push("after")` with the fused synchronous form: the framed version never appends, while the direct form does. Both meet the stated no-loop, nonrecursive, E = `never`, synchronous criteria. “Cannot pause” and “contains no observable interruption boundary” are different properties. The `area` output also moves both `double` calls after `Settings.scale()` (1497-1512), despite the evaluation-order promise.

   **Fix:** Limit direct regions to code with no Effect boundaries, or preserve explicit checkpoints and their ordering. Specify semantic scheduling/interruption points before G10’s differential experiment; compare deterministic interruption traces as well as results. Keep the cold exported wrapper.

7. **High: The printed producer lowering leaks stack space.** [Lines 1945, 2286, 2408-2426, 2596](/Users/williamcory/effectscript/docs/design/effectscript-design.md:2408). The generator clause shown in Chapter 24 does not satisfy the kernel’s tail-resume optimization, so its claimed constant-space iteration is false.

   R10 runs that exact `Effect.gen(function* () { return yield* resume(k, undefined) })` shape: 100 elements reach 202 frames; 1,000 reach 2,002. The suspended generator’s Iterator frame sits above `Owner(k)`, preventing kernel lines 377-378 from removing it. T17 instead uses a tail `flatMap` callback, whose frame is already gone.

   **Fix:** Define compiler tail-resume lowering that removes the clause activation before resumption, for example the tested tail `flatMap` shape. Run T17 against actual emitted generator-backend output, including cleanup around a tail resume. Revise P22’s evidence claim; this matters for unbounded producers, not just microbenchmarks.

8. **Medium: `using` and `fork` disagree about what owns a block.** [Lines 815-828, 901, 2384-2397](/Users/williamcory/effectscript/docs/design/effectscript-design.md:2384). Whole-block Scope insertion closes unmarked acquisitions that the resource chapter says belong to the caller, while attaching `fork` to an existing Scope does not itself make every lexical block a lifetime boundary.

   ```ts
   function openAndLog() {
     const connection = Database.connect() // supposedly caller-owned
     using log = Logs.open()
     return connection
   }
   ```

   R8’s documented `Effect.scoped` lowering returns an already-closed connection. Conversely, a nested block containing only `fork` may inherit a Scope outside that block.

   **Fix:** Specify exactly which constructs create regions and which acquisitions receive each Scope. Either bind only `using` acquisitions to the local Scope, or explicitly adopt whole-block ownership and address escaping resources. Include fork-only blocks and mixed marked/unmarked acquisitions in the region-lowering experiment.

9. **Medium: The reference implementation disagrees with P4.** [Lines 2071, 2263, 2576-2578, 2618](/Users/williamcory/effectscript/docs/design/effectscript-design.md:2263). The two passing suites do not currently establish the same dynamic-context semantics.

   R9 resumes under a clause-local provider and performs again: strategy A’s next clause sees `resume-site`, as specified; strategy B sees `handle-site`. B reuses its old prompt instead of recording the resume-site Context. It also lacks the public transfer and `onReturn` operations.

   **Fix:** Promote G16 from future confidence work to an immediate semantic gate. Add this shared law, align B with the chosen P4 rule, and state which laws B cannot yet express. Keep P2 as a target architecture; do not count 128 + 20 passing tests as cross-implementation agreement.

10. **Medium: The cancellation adapter example cannot cancel its wait.** [Lines 837-855, 2016-2021](/Users/williamcory/effectscript/docs/design/effectscript-design.md:837). Default `Effect.acquireRelease` masks the entire `suspend`, so cancellation never reaches the demonstrated cancellation hook while connection establishment is pending.

    R3 records no exit and no canceller invocation after interruption; delivery must occur before the fiber can finish. Effect’s source explicitly restores interruptibility only with the acquisition option (`internal/effect.ts:4143-4155`).

    **Fix:** Show interruptible acquisition, provide `suspend`’s release hook for accepted-but-undelivered values, and define connection-error delivery. Add tests for cancellation before connection and cancellation after deferred acceptance. Distinguish masking registration of release from indefinitely masking an external wait.

11. **Medium: P14 invents the partial annotation the brief reserved.** [Lines 1465-1469, 1561-1582, 2546, 2588](/Users/williamcory/effectscript/docs/design/effectscript-design.md:1561). Treating a bare callback `(a: A) => B` as an A-only annotation with fresh E/R parameters is a new partial-contract convention, not a consequence of settled decision 2.

    The callback-only exception also contradicts the request for one contract model across function types, callbacks, and service methods. It needs an owner decision; describing it as already required obscures that choice.

    **Historical recommendation, resolved by D57:** Illustrate effect polymorphism using the established full form below. D57 now specifies `infer Name` for partial return annotations and retains D12 for callback parameter types:

    ```ts
    function mapAll<A, B, E, R>(
      xs: ReadonlyArray<A>, f: (a: A) => Effect<B, E, R>
    ): Effect<B[], E, R> { /* ordinary loop */ }
    ```

    G9 should check multiple callbacks, returned callbacks, aliases, and declaration round trips; manually reviewing one hover is insufficient.

12. **Low: Two introductory comparisons promise different behavior.** [Lines 8-26, 160, 256, 891-896](/Users/williamcory/effectscript/docs/design/effectscript-design.md:891). The concurrency introduction incorrectly gives `race` fail-fast semantics, and the opening “same thing” comparison omits Effect’s tracing behavior.

    `Effect.race` waits for a success when one branch fails (`internal/effect.ts:1631-1644`); it is not `all`’s failure policy. The opening TypeScript example uses named `Effect.fn` spans, whereas the generated version is untraced.

    **Fix:** Give `all`, `race`, `raceFirst`, and timeout separate exit rules. Compare equally traced examples, or flag the tracing difference immediately. Keep the owner’s requested examples-first reading order; these corrections do not justify moving runtime chapters back to the introduction.

Positions on O1-O11:

| Question | Position and deciding evidence |
| --- | --- |
| O1 | Prefer thunks initially. Before adopting contextual conversion, check generic/overloaded combinators, nested Effect results, functions as data, and declaration round trips. A pipeline operator can later share these same coldness rules; it is not yet a decision. |
| O2 | Support distinct `service` and `control` declarations: they expose a real continuation boundary. Test generated identities and imported Context keys; do not infer identity from spelling alone. |
| O3 | Disagree with exposing the entire API immediately. Start with scoped handlers; keep transfer restricted until findings 1, 4, and 5 close. Internal continuations and producers still satisfy the architectural foundation, the original brief expressly permits this. |
| O4 | Support producers and internal iteration, conditional on finding 7. Complete the Stream cancellation adapter before promising the boundary. |
| O5 | Support lexical block ownership, after defining region insertion and acquisition ownership in finding 8. “Innermost existing Scope” is not an equivalent specification. |
| O6 | Support optional `wire`, with a bounded static graph contract. Specify treatment of dynamic/opaque Layers and residual inputs. Test types that hide outputs and runtime-selected providers; do not promise exhaustive diagnostics from arbitrary Layer values. |
| O7 (closed by D57) | The review recommended keeping the spelling open. D57 now uses `infer Name` slots in return annotations with bodies and retains D12. |
| O8 (closed by D59) | Support upstream discussion with minimal repros, including the new cleanup case. D59 sets status to drafted, not filed, in `docs/upstream/`. |
| O9 (closed by D58) | The review recommended a distinct name because an unrelated language uses it. D58 keeps EffectScript as a controversial default before public release. |
| O10 | Prefer heap frames for non-tail recursion. Define tail-loop checkpoints consistently with finding 6; stack safety and interruptibility are separate guarantees. |
| O11 | Support defects for ordinary synchronous foreign exceptions, but conditionally: invocation remains inside the cold computation, foreign work is not presumed pure, and hidden dependencies/cancellation limits are explicit. `Effect.try` supplies typed failures when wanted. Rejecting `unknown` R merely because it prevents running misses the original direction’s deliberate trust gate; acknowledge this as a policy change, not a technical impossibility. Test coldness, exception classification, and synchronous callbacks before adopting it. |

The seven settled decisions are substantially covered. Decision 2 is reopened by P14, decisions 3-4 have the concrete failures above, and decision 5 needs consistent resource ownership. The new syntax is generally labeled proposed; no rejected `effect function` or `throws` category is reintroduced. Decision 7’s changed foreign-call policy is correctly identified as pending O11. I retain P1, P3-P6, P9, P11, P13, P15-P17, and P19-P21 as directions, subject to the stated boundary/lowering fixes; the findings explain my objections to P2, P7-P8, P10, P12, P14, P18, and P22 as presently justified.

The next useful compiler work is a **small checked-IR experiment**, not a broad implementation. G3 is the most immediate runtime ownership gap; G8/G9 are the principal checker prerequisites, and their proposed experiments are too weak without executable typing judgments. Exercise `resumeWith` requirements, unknown foreign placement, region ownership, compound causes, and generator tail resumption end to end. No feature reviewed requires a second scheduler or DI container, but chapter 18 needs explicit compound-cause, region, and continuation-lifetime rules before its “mechanical” lowering claim is warranted. G1 remains the upstream-support risk; pinning permits experiments while that conversation proceeds.

Evidence run: the original suite passed **128/128**, the independent machine **20/20** in 17.66 seconds, and the new scratch suite **11/11 observational probes**. Passing scratch tests records the counterexamples, not conformance. Both benchmarks ran; deep capture reproduced quadratic copying and took 81.3 ms at depth 8,000. I checked the relevant Effect source, local effect-tsgo type parser, research notes, and current VibeLang Result specification. The maintained compiler baseline is correctly [microsoft/TypeScript, under `tsc/`, requiring Go 1.27](https://github.com/microsoft/TypeScript/blob/main/tsc/go.mod); [typescript-go is archived](https://github.com/microsoft/typescript-go). Only this review and the designated scratch probes were written.
