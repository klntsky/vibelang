# Independent design: one-shot delimited control (Codex, GPT-6.1 Sol)

Written 2026-10-15 by Codex (`gpt-6.1-sol`) from the shared brief, without access to the main design. Kept verbatim as the independent second design; its runnable machine is `experiments/reference-machine/`. Paths below that say `kernel/` refer to that directory.

# One-shot delimited control as the computation model

**PROPOSED — Design decision.** A computation is interpreted by a fiber with explicit frames. An operation moves the frames up to its handler prompt into a one-shot continuation. Resumption moves those same frames back. The language uses deep, dynamically selected handlers, immutable Context snapshots, region-owned resources and children, and fiber-local continuation ownership. The continuation contract is foundational; the prototype's `bind` nodes are one representation of that contract, not its definition.

**VERIFIED — Evidence convention.** `VERIFIED` identifies inspected Effect 4.0.5 source, directly inspected prototype code, or the experiments in `kernel/kernel.test.ts`. `PROPOSED` identifies a language rule or implementation recommendation. `UNTESTED` identifies an implementation claim needing further experiment. Each paragraph, list item, table row, and code block below carries one of those labels; a label covers the statements within that unit. Verified finite experiments do not establish a universal correctness proof.

**VERIFIED — Source identity.** Effect citations such as `internal/effect.ts:625–679` refer to `./experiments/node_modules/effect/src/`. The prototype installs exactly `effect@4.0.5`; `kernel/node_modules/effect/package.json:4` reports that version. The installed and pinned `src/internal/effect.ts` have identical SHA-256 `29200eee28c991b77b9f62317831332d4330179d8ab654fec8dea4472732d0af`. No other designer's artifacts or docs were read.

## 1. Definitions and execution boundaries

| Status | Term | Definition |
| --- | --- | --- |
| PROPOSED | Computation | An evaluation with successful result A, expected failures E, service requirements R, and observable timing, mutation, scheduling, cancellation and resource behavior. It may return, fail, perform an operation or suspend. Every language function participates in this model. |
| PROPOSED | Cold program value | A reusable description/factory for starting a computation. Constructing it does not run the described body. It owns no suspended stack, fiber or acquired resource merely by existing. Captured ordinary objects remain shared references unless the program explicitly allocates fresh ones on execution. |
| PROPOSED | Fiber | One schedulable execution, with identity, pending interruption, an active frame stack and registry of owned suspended segments. It can have several parked segments but only one executing control path at a time. |
| PROPOSED | Frame | A reified piece of future control or dynamic restoration. Frames belong to exactly one active stack or suspended segment. A frame is moved, never cloned. |
| PROPOSED | Delimited continuation | A region-bound, one-shot capability for the remaining computation at an operation's return point, stopping at a selected prompt. It includes the prompt for deep resumption, all intervening frames, and the dynamic snapshot at capture. It excludes the caller of the handle expression and the prompt's owner fence. |
| PROPOSED | Prompt/handler | A prompt is a runtime delimiter with operation clauses, entry dynamic state, and an owner. A handler is the clauses plus their lexical closures. Selection of the handler is dynamic; clause variables still have lexical scope. |
| PROPOSED | Execution boundary | A boundary that limits capture, intercepts an outcome, restores dynamic state, or changes ownership. These are separate properties; encountering a scope does not imply that an operation cannot pass through it. |

| Status | Boundary/frame kind | Capture and exit behavior |
| --- | --- | --- |
| PROPOSED | Call/return (`Bind`, local activation) | Capturable. Stores return address, live locals and pending expressions; unwinding skips success-only work. |
| PROPOSED | Fiber root | Hard capture limit and ultimate outcome observer. Unhandled operations are defects after static checking has failed to prevent them. Root completion drains owned regions and children. |
| PROPOSED | Handler prompt (`Prompt`) | Matching operations stop here; nonmatching prompts move into the captured segment. On deep resume the matching prompt is reinstalled. |
| PROPOSED | Prompt owner fence (`Owner`) | Sits below its prompt and remains while a clause runs. Before the handle expression completes, revokes and disposes every unused continuation it owns. Inner owner fences may themselves be captured by an outer prompt. |
| PROPOSED | Typed catch | An abortive boundary for matching expected failure, but transparent to operation capture. Its frame moves with the segment if inside the delimiter. |
| PROPOSED | Scope/finally | A restoration and cleanup boundary. Capturing does not close it. Returning, failing, interrupting or abandoning its region closes it once. A language `finally` is an explicit cleanup frame, not a foreign JS `finally`. |
| PROPOSED | Provide/mask/span | Restoration boundaries, transparent to capture. Suspension detaches their frames without running their exit behavior; activation changes to the prompt's entry state. |
| PROPOSED | Resume-return | Delimits a resumed computation and restores the waiting clause's dynamic state when it completes. It marks the consumed token completed even when that computation fails. |
| PROPOSED | Foreign JS frames | Hard barrier for untransformed synchronous frames. The compiler must not attempt to save a JS activation that can return only on the current JS stack. An explicit Effect bridge can suspend through its own runtime, but is opaque to this public-API kernel. |
| PROPOSED | Fork | Hard control and ownership boundary. A child gets a new root, identity and continuation registry. A parent continuation cannot be resumed there. Context values can be inherited without inheriting the parent's control stack. |

**PROPOSED — Uniform functions and explicit contracts.** Ordinary calls compose computations and expose successful A values without per-call `await` or `yield*`. `function f(): Effect<A,E,R> { ... }` constrains the complete computation; its `return` expression is checked against A. Inference or explicit declarations must account for every channel, including callback and service-method contracts. E and R equal to `never` establish neither purity nor synchronous completion. Program construction is a distinct expression form, whose spelling can remain undecided without postponing its coldness or execution semantics.

**PROPOSED — Operation contracts.** An operation has a nominal identity, argument type and successful reply type; its clause receives a continuation from that reply type to the handled body's result type, bound to the owner's lifetime. Unhandled user operations contribute nominal `Handler<Operation>` capability requirements to R; installing their prompt discharges those requirements for the body, while clause requirements remain. Runtime-provided operations such as scheduling and asynchronous waits are discharged at the fiber root. A handler capability is opaque, owner- and fiber-bound evidence, not a service instance that can be freely forged or copied across fork. Foreign Effect adapters must validate or explicitly install the required prompts rather than treating an arbitrary Context entry as sufficient. This preserves the chosen three-channel Effect contract without interpreting E/R as a purity analysis.

**UNTESTED — Operation typing.** The IR prototype uses string operation names and erased types, so nominal operation identity, R inference for handler capabilities, lifetime checking, fork rejection of inherited control evidence and generated foreign adapters need compiler experiments. Their representation in declarations is part of that work, not a feature proven by the runtime tests.

## 2. Abstract machine

**PROPOSED — Configuration.** A global runtime has runnable fibers and event queues. One fiber is `M = ⟨fid, c, K, δ, O, L, I, Q⟩`: identity; current instruction or outcome; active frames K ordered bottom to top; current dynamic state δ; owner registry O; linear continuation registry L; pending interruption I; and scheduler state Q. `δ = ⟨Context, currentRegion, interruptibility, span⟩`. Handler lookup is derived from K, not an independent stale stack. A region holds a Scope, owned child set, and its cleanup registrations. Tracing spans and current Scope can physically live in Context; the tuple makes their semantics explicit.

**PROPOSED — Outcomes.** Outcomes are `Success(v)`, `Fail(e)`, `Defect(d)`, `Interrupt(cause)` and internal `Abandon(owner,reason)` for disposing an unused segment. Composite causes preserve body failure and cleanup defects. Abandonment is a cleanup reason, not a new user E alternative. The compiler may erase this distinction at the Effect boundary into interruption with diagnostic annotations, but must retain it for language-level diagnostics.

**PROPOSED — Instruction rules.** The following pseudocode is a specification; operations on K, δ, O and L within one machine transition are atomic with respect to scheduling and cancellation acceptance.

```text
PROPOSED
call f(args):
    push Call(returnPC, liveLocals); enter f(args)
return v:
    pop frames until a success continuation or delimiter consumes Success(v)
    restore dynamic bindings as their frames exit

handle body with H:
    allocate active owner o
    push Owner(o); push Prompt(o,H,δentry=δ); enter body

perform op(args):
    find nearest Prompt p in K with an op clause; do not cross a root/foreign barrier
    split K = Kouter ++ [p] ++ Kinside
    allocate k = (fid,p.owner,Fresh,frames=[p]++Kinside,δcapture=δ)
    register k with p.owner; K := Kouter; δ := p.δentry
    enter p.H[op](args,k) outside p, with its owner fence still active

resume k(v):
    validate same fid, active owner, Fresh; atomically claim Fresh -> Running
    move k.frames into Kclause ++ [ResumeReturn(k,δclause)] ++ k.frames
    clear k.frames; δ := k.δcapture; deliver Success(v) at perform's return point

discontinue k(e):
    same claim/move/restore as resume; deliver Fail(e) instead of Success(v)

abort/abandon k(reason):
    claim Fresh -> Disposing(reason); dispose its frames without ordinary code
    await cleanup; clear frames; record terminal Abandoned or Canceled

Owner(o) exits with outcome x:
    deactivate o; dispose all its Fresh segments in reverse creation order
    await disposal; propagate x combined with any cleanup defects

ResumeReturn(k,δclause) exits with x:
    record k completed with x; restore δclause; deliver x to the resume expression
```

**PROPOSED — Calls and return values.** A call can suspend anywhere its inferred contract allows an operation, regardless of the caller's source syntax. The activation's live locals and pending expression belong to its frame. Resuming supplies the successful result of the operation, not of the entire function. The resumed computation's final result becomes the result of `resume`, which the clause can transform before returning from the handle expression. Capturing does not execute cleanup or ordinary return paths.

**PROPOSED — Failure and defects.** `throw e` in language code constructs Fail(e); a typed catch handles only matching expected failures after unwinding intervening regions. An explicit E bounds what escapes. Defects bypass expected-error catches and retain diagnostic causes; a privileged cause handler can observe them after cleanup. Unexpected exceptions in foreign JS and kernel implementation callbacks are defects. A resumed segment can catch a failure injected by `discontinue`; `abort` instead disposes without running its catch clauses. Failures raised by a clause are outside the removed matching prompt and cannot accidentally reenter that prompt.

**PROPOSED — Asynchronous interruption.** A request adds to I; it does not copy, rewind, or restore a previous I. Acceptance occurs at an interruptible machine boundary, including after unmasking and before the next user instruction. It changes control to interruption, cancels outstanding asynchronous registration, and disposes the active path and owned detached paths. A mask defers acceptance, not the recording of a request. Cleanup runs masked, may suspend, and is awaited. An uninterruptible operation that never completes can prevent cancellation from completing; the semantics do not promise forced termination of arbitrary JS or uncooperative foreign work.

**PROPOSED — Fork and join.** Fork starts a cold program with a fresh K/O/L and identity, a snapshot of Context, an owned child root scope, and interruptibility enabled by default. It is recorded in the forking region before resources can escape cancellation. Parent masks do not accidentally make a child permanently uninterruptible. Join awaits the child's terminal outcome, including finalization, and propagates that outcome without replacing the joining fiber's Context or identity. Interrupting a join removes its wait registration; enclosing region cleanup also interrupts and awaits its owned child. Child failure is observed at join; region ownership alone does not imply fail-fast sibling cancellation. Race/all are explicit supervisors with their own failure policies.

## 3. One-shot ownership, abandonment and races

| Status | From | Event | To and observer |
| --- | --- | --- | --- |
| PROPOSED | Fresh | Valid resume/discontinue | Running; exclusive ownership of frames transfers to the active path. The caller of resume waits for its outcome. |
| PROPOSED | Fresh | Explicit abandonment or successful owner exit | Disposing(Abandon) then Abandoned; disposer waits for cleanup. The owner cannot return first. |
| PROPOSED | Fresh | Accepted cancellation or failed owner exit | Disposing(Cancel/cause) then Canceled; owner/fiber observer sees the original cause plus cleanup defects. |
| PROPOSED | Running | Segment returns or fails | Completed(exit); caller of resume observes that exit, subject to any subsequently accepted interruption. |
| PROPOSED | Running | Accepted fiber cancellation | CancelingActive then Completed(interruption); frames are already on the active stack, so no detached copy is disposed. |
| PROPOSED | Running, Completed, Disposing, Abandoned or Canceled | Resume/discontinue again | ContinuationUseError defect for the attempted user, without running the continuation again. A diagnostic includes owner, token, current state and first consuming action. |
| PROPOSED | Any | Another fiber attempts consumption | WrongFiber defect; no migration or state transfer. The API for external completions is a delivery registration, not unrestricted access to k. |

**PROPOSED — Lifetime.** A token may be parked while its clause waits asynchronously, but cannot outlive its owner. Returning k, putting it in a global, or closing over it does not extend that lifetime. A lifetime-aware checker should reject such escapes; runtime revocation remains mandatory for untyped interop. At owner exit an escaped unused token is abandoned and its resources close before the owner returns. Dropping the last JS reference is not the cleanup mechanism. Prompt ownership rather than garbage collection makes the timing deterministic.

**PROPOSED — Cancellation versus claim.** Cancellation requests enqueue onto the owning fiber; a foreign thread may not directly edit its K. If cancellation is accepted before claim, k becomes nonresumable and the resume attempt reports canceled/unavailable. If claim occurs first, the frames move exactly once, and later cancellation unwinds those active frames. It may still prevent any user code after the operation from running. A pending request inside a mask does not win merely by being recorded: acceptance is deferred until the mask permits it. Capture, claim and owner deactivation cannot be interrupted halfway through. A runtime with multiple JS workers needs an atomic gate or serialized owner mailbox, not just a non-atomic boolean.

**PROPOSED — A delivered value is an ownership transfer.** Async resource acquisition has a separate `Waiting -> Delivered | Canceled` gate. On Delivered, the resource is transferred into the consumer's Scope with release registered before interruption can be accepted. On Canceled, the consumer never runs, and the producer/registration retains the release obligation. A bare number needs no release; a socket does. Silently ignoring a late callback whose resume effect would have installed release is a leak. The losing producer must receive rejection or run an explicit disposer, not have its cleanup effect silently dropped.

**PROPOSED — Shutdown protocol.** The resource adapter must return a cancel-and-join action whose completion guarantees no future delivery. Cancellation first seals the consumer gate, asks the producer to stop, drains any in-flight delivery and releases it, then acknowledges cleanup. If a producer can deliver after that acknowledgement, its supervisor must own a durable late-release path; such a producer does not satisfy this adapter's contract. There is no finite-time promise that a dead fiber will clean up a resource delivered arbitrarily far in the future without a living owner.

**VERIFIED — Prototype realization and scope of evidence.** `kernel/kernel.ts` serializes frame claims in one fiber; its Fresh/Running/Completed/Abandoned/Canceled states compress the specification's intermediate cleanup states. It records cancellation of an already running segment as Canceled rather than retaining a separate Completed(interruption) exit record. Detached frames are cleared on consumption/disposal. `handoff` models a one-delivery producer and a cancel-and-join action. Test (vi) parks a real continuation in a handler clause, deterministically accepts cancellation, then delivers a resource before producer join completes; release occurs exactly once before the interrupting waiter completes and the captured token is Canceled. Its opposite-winner test delivers the resource, resumes that token to completion, and verifies normal acquisition release. These test a real Effect-backed resource-delivery gate with continuation ownership, not cross-thread mutation of a raw continuation.

**UNTESTED — Race coverage.** Exhaustive interleaving enumeration, multiple workers, reentrant hostile producers, duplicate delivery after cancellation, a cancel-and-join action that defects or lies about quiescence, and arbitrary callbacks arriving after shutdown are not verified. The adapter assumes one delivery and a successful quiescence handshake. The language contract requires rejecting or safely supervising adapters that cannot meet it.

## 4. Saved and restored state; Context decision

| Status | State | On capture and handler entry | On resumption / clause return |
| --- | --- | --- | --- |
| PROPOSED | Control frames and locals | Move all frames through the matching prompt, including nonmatching prompts, typed catches and restoration frames; leave owner fence and outer frames active. | Move segment back above a ResumeReturn frame. Restore waiting clause state when that segment finishes. |
| PROPOSED | Handler stack | Derived from the moved frames; the selected prompt and all inner prompts are absent in its clause. Outer prompts remain available. | Restore the selected and inner prompts exactly. Deep handling makes later operations see the selected handler again. |
| PROPOSED | Context | Save the entire immutable Context pointer at perform. Activate the prompt's entry Context for its clause. Service objects are shared references, not copied values. | Restore the capture pointer, including outer bindings as they existed then. The clause's later providers are not grafted into the continuation. Restore the clause pointer on resume-return. |
| PROPOSED | Current resource Scope | Save region/Scope identity and retain its open state. Activate entry region in clause. Detaching does not close scopes. | Restore the same region; do not allocate a replacement. Its finalizers and acquired resources remain the same registrations. |
| PROPOSED | Interruptibility | Save logical mask state plus restoration frames; activate entry mask. | Restore captured mask; honor any pending interruption when enabled. Then restore clause mask. |
| PROPOSED | Pending interruption | Remains in the fiber; never saved as a restorable historical snapshot. | Never erase a newer request by resuming an older state. |
| PROPOSED | Tracing | Save current span binding and span-exit frame; activate prompt's entry span in clause. | Restore the same span object; its elapsed time includes suspension. Do not close/restart it at perform. Emit capture/claim/dispose events with shared correlation IDs. |
| PROPOSED | Fiber identity | Unchanged for clause and resumed code. | Unchanged. Observers, external interrupt authority and root identity are never snapshotted into a clone. |
| PROPOSED | Children | Retain region-owned child sets by identity; existing children continue while their parent segment is suspended. The fiber's real child registry remains authoritative. | Restore ownership references, without copying, canceling or restarting children merely because of capture. Dispose the owning region by interrupting and awaiting children before releasing its resources. |
| PROPOSED | Scheduler and runtime bookkeeping | Context-local scheduler bindings follow Context, but run-loop budget, current JS stack, observer list, `_yielded`, deferred interruption and dispatcher state are not frozen into k. | Resume through the runtime's scheduling protocol; refresh caches when activating Context. Never replay an old callback registration or overwrite a newer runtime event. |

**PROPOSED — Choose snapshot semantics.** A service lookup after resume resolves under the captured immutable Context, not under the clause's current ambient Context with selected internal overrides replayed onto it. An inner `provide(Users, testUsers)` stays visible in resumed code; its clause sees the Users binding at prompt entry. A different Database provided by the clause cannot silently change a partially executed service method. This preserves the meaning of already-selected dependencies and makes the residual R of that computation stable across suspension.

**PROPOSED — Why not delimited dynamic rebinding.** An alternative would retain only internal binding deltas and rebase them onto the resumer's Context. That makes outer service resolution depend on when and where a token is resumed. An acquired Users instance might still close over Database A while its later residual Database lookup suddenly sees B. Such rebinding can be an explicit operation or a new cold program run under another provider; it is not the default meaning of resume. Snapshotting does not freeze mutation inside service instances or create a transaction.

**VERIFIED — Relation to existing Effect.** `internal/effect.ts:2186–2211` saves `prevContext`, installs the updated Context with `fiber.setContext`, and registers exit restoration of that exact saved pointer. `scoped` similarly saves/restores Context and closes its new Scope (`4109–4119`). `forkUnsafe` initializes a child from the parent's Context (`5527–5549`). Effect's fiber cache reads `Tracer.ParentSpanKey` from Context (`745–760`), and `setContext` refreshes cached runtime services (`724–738`). These support snapshot-like dynamic restoration; they do not implement first-class delimited capture by themselves.

**VERIFIED — Prototype Context evidence.** Test (i) observes `outer` in the clause, `inner:42` in nested resumed code, and one fiber ID for all three locations. Scope tests verify retained resource lifetime, not just a service string surviving a lexical closure.

**UNTESTED — Broader state evidence.** Span-exit frames, custom scheduler Context overrides, all ambient Context.Reference interactions, pending interruption during every capture/restore edge, and reentry from an already masked foreign Effect are not fully tested. The prototype's logical interruptibility is relative to its entry `uninterruptibleMask` restore function; it does not inspect private Effect interruptibility and cannot claim a complete foreign-mask interop contract. Its dynamic snapshots cover Context and region identity, while explicit span instructions remain compiler/runtime work.

## 5. Finalization ordering

**PROPOSED — Region discipline.** On normal return, expected failure, defect, interruption and abandonment, unwind the affected frames from innermost to outermost. Before a region releases its own resources, interrupt and await all children that borrow them. Run the region's registered finalizers sequentially in reverse registration order, masked, under the Context captured when the finalizer was registered. Restore enclosing bindings according to their frames. Cleanup is exactly once even if cancellation and owner exit both try to initiate it.

**PROPOSED — Detached paths.** Normal handling leaves the captured region open. If its clause does not resume, close any currently active clause-local regions first, then dispose unused captured segments in reverse creation order, then let the owner complete. Within a segment, nested regions close inside-out. On owner interruption the order is async registration cancel-and-join, active clause frames, owned detached segments, outer regions, root outcome publication. This orders sibling detached segments explicitly rather than pretending they have a single lexical nesting order.

**PROPOSED — Expected failure and finally.** A catch outside a region observes failure only after that region's finalizers complete. A catch inside an outer still-live region does not close that outer region. `discontinue` can reach captured catches; abandonment runs cleanup frames only. Language `finally` code must be compiled into those cleanup frames and may suspend under a mask. It cannot be implemented by relying on JavaScript `finally` around a yielded `Effect.gen` body.

**PROPOSED — Cleanup failures.** Attempt all finalizers despite defects in earlier finalizers. Retain the body cause and append cleanup causes in observed cleanup order. Cleanup defects turn an otherwise successful handle into a defect; they never become a successful abandoned result. A user cleanup that throws an expected error must handle it or explicitly widen the owning computation's contract. The primitive resource-release channel is infallible in E, but defects remain possible. Cancellation waiters and fiber observers are notified only after required cleanup; a hanging masked finalizer means they continue waiting.

**VERIFIED — Effect foundation.** Scope closure marks its state Closed before running finalizers and is idempotent (`internal/effect.ts:3939–3963`). Sequential finalization iterates registrations in reverse and collects exits (`3980–4005`). OnExit runs cleanup uninterruptibly and combines failure causes (`4173–4214`). `acquireRelease` masks acquisition/registration and captures Context for release; `addFinalizer` captures Context too (`4143–4171`). The async-finalizer path aborts the signal and runs cancellation cleanup on interruption (`1141–1207`).

**VERIFIED — Prototype order.** Test (iv) records `open:abandoned, close:abandoned, returned:short`. Test (v) records outer/captured/clause acquisition followed by `cancel:wait, close:clause, close:captured, close:outer`. The child-ownership test records `close:child` before `close:parent`. The finalizer-defect test retains both expected failure and defect and still closes the outer region. These are actual kernel tests; the kernel uses Effect.gen only for the noncapturable cleanup executor, with explicit Effect exits, not as its continuation implementation.

## 6. Deep/shallow and lexical/dynamic binding

**PROPOSED — Deep by default.** A resumed segment includes the selected prompt, so a later perform finds the same handler. The clause itself runs outside that prompt, avoiding accidental self-recursion when it performs the same operation. Its ordinary variables are lexical closures. The active handler is the nearest matching dynamically installed prompt in the current fiber, not the handler surrounding a function's textual definition.

**PROPOSED — Shallow meaning.** A shallow handler would resume only Kinside, without reinstalling the selected prompt; a second occurrence would reach an outer handler or become unhandled. This is a different semantic operation, not a faster representation of deep handling. The initial language exposes deep handlers only; a later shallow primitive would need explicit contract inference and ownership rules.

**PROPOSED — Fork inheritance.** Fork copies service/reference bindings and tracing parent information as immutable Context references, and creates a new child scope and control root. It copies no live prompt, capture token, caller activation, owner fence or pending interruption. A reusable handler description can be explicitly installed in the child; that creates a new prompt and owner there. Resource services inherited from the parent remain borrowed, which is why the parent region awaits child termination before release.

**VERIFIED — Prototype binding.** The deep-handler test resumes two operations through the same handler. The fork/join test gets an inherited service under a newly installed child handler. A second case forks an operation with only a parent handler: the child's operation remains unhandled and joining exposes the defect. Captures therefore do not silently cross fiber roots in this prototype.

## 7. Deriving language facilities from the core

### Typed errors

**PROPOSED — Derivation.** Expected failure is an abortive operation whose continuation is disposed rather than offered for ordinary resumption. A typed catch is the nearest matching failure prompt. The E contract records failures that reach the computation's boundary after internal catches. Defect and interruption use separate causes and are never silently added to E. An explicit `Effect<A,never,R>` rejects escaping language throws but permits internally handled expected failures, suspension, resource use and mutation.

### Services and dependency injection

**PROPOSED — Identity and requirements.** Service identity maps to Effect Context keys, with canonical identities preserved across compiled modules and TypeScript declarations. Calling `Users.find` requires Users plus the method's residual R. Compiler-generated calls perform lookups under the active snapshot, so inference follows the same model as suspension. Service keys and Layers remain the runtime DI representation; there is no second service container.

**PROPOSED — Acquisition versus method calls.** A provider's acquisition contract and a method's invocation contract are distinct. If UsersLive acquires Database and closes over the concrete database service, constructing UsersLive requires Database and may fail or acquire resources, but invoking its methods need not require Database again. If its method performs a call-time `Database` lookup, that requirement remains in the method signature and caller's R. Providing Users cannot erase that residual requirement. Capturing a whole Context for every method is an explicit closure conversion with lifetime implications, not an inference trick that makes hidden requirements disappear.

**PROPOSED — Example contracts.** The following pseudocode distinguishes binding an acquired object from doing a residual lookup. The code is illustrative source notation, not accepted by the prototype's IR API.

```typescript
// PROPOSED
function acquireUsers(): Effect<UsersImpl, ConnectError, Database | Scope> {
  const db = Database                  // acquire-time service selection
  return { find(id) { return db.find(id) } }
}
// Caller: Users + the actual db.find contract, with no new Database lookup.

function acquireResidualUsers(): Effect<ResidualUsersImpl, never, never> {
  return { find(id): Effect<User, NotFound, Database> {
    return Database.find(id)            // call-time service requirement
  } }
}
// Caller: ResidualUsers | Database. Providing ResidualUsers alone is insufficient.
```

**PROPOSED — Composition, sharing and diagnostics.** Provider graphs lower to Layer composition and scoped memoization. Share one acquired provider instance within its declared build scope; explicit fresh builds create different instances. Acquisition errors contribute to the build computation's E, and unresolved acquisition inputs to its R. A scoped override is a Context restoration frame, making test providers obey exactly the same resume behavior. Statically known missing providers, duplicate/ambiguous providers without explicit override, and provider cycles are compiler diagnostics. Dynamic graphs must detect cycles and duplicate bindings before waiting for acquisitions; diagnostics identify provider keys and graph edges, without logging credentials or service payloads. A missing runtime service is an interop defect unless an explicitly optional lookup returns an expected result. Resource-bearing service instances may not outlive their acquisition scope.

**VERIFIED — DI foundation.** Effect's service-key model is in `Context.ts:103–137,209–431`; Context construction and addition are at `896–1028`. Layer has `build(memoMap, scope): Effect<Context<ROut>,E,RIn>` (`Layer.ts:58`), and its MemoMap memoizes layer construction by layer instance and observes scope ownership (`191–320,418–465`). The prototype uses Effect Context/Scope directly and implements scoped overrides. It does not implement provider graph compilation or static R inference.

**UNTESTED — DI compiler claims.** Separate acquisition/call-time requirement inference, cyclic graph diagnostics, exported declaration checking, scoped service escape rejection and full Layer memoization interop require a compiler and dedicated tests. The current service test establishes dynamic snapshot behavior only.

### Async suspension

**PROPOSED — Derivation.** A wait operation captures the remainder; its clause registers a completion source and parks the owning fiber. Completion queues a delivery to that fiber; it does not execute arbitrary language continuations on the producer's JS stack. The owner fence makes interruption and handler-return cleanup account for the parked remainder. Runtime-internal completion registrations may have their own one-shot gates, but those gates are not user continuation ownership rules. A scheduling trampoline must prevent chains of callbacks from recursively reentering unlimited language frames.

**VERIFIED — Existing substrate.** `Effect.callback` supports async registration, an AbortSignal and an Effect canceler, but accepts only the first resume and silently ignores later calls (`internal/effect.ts:1141–1207`). On asynchronous delivery it calls `fiber.evaluate` synchronously on the resumer's stack (`1162–1164`). The prototype defers its compiler IR through Effect.suspend and uses a bounded machine loop; it still inherits this Effect entry behavior for bridge callbacks.

**UNTESTED — Async stress.** A long chain of distinct fibers synchronously delivering to each other, foreign AsyncLocalStorage restoration under capture, and cross-thread completion routing are untested. The native runtime extension should enqueue reentrant deliveries when necessary; public API wrapping alone does not prove cross-fiber callback stack safety.

### Resources and structured concurrency

**PROPOSED — Derivation.** A resource region is an owned collection of cleanup frames. Capture transfers the region's control ownership into k without releasing its resources; resume returns it to execution; abandonment disposes it. Acquisition masks only the transfer/registration window, with an explicitly interruptible acquisition option and the resource-delivery protocol above. A child borrows region services and belongs to that region until it completes. Cancellation is an abortive unwind plus child cancellation and cleanup, not deletion of a continuation object.

### Generators and streams

**PROPOSED — Derivation.** A generator's `emit(value)` is an operation. Its consumer handles one emission, storing the one-shot remainder in a generator owner region. `next` resumes that remainder until another emission or termination; it receives a fresh one-shot token at each emission, never repeatedly consumes the same token. Consumer return/close abandons the parked remainder and awaits cleanup. Because a token cannot escape its owner, a generator object owns an explicit region whose lifetime encloses iteration; it is not a k returned from a completed ordinary handle. The runtime needs an owner-preserving iteration protocol or runner fiber to keep that owner alive between requests.

**PROPOSED — Streams and backpressure.** A pull stream is a cold program describing such an owned producer, with each downstream demand allowing one resumption or bounded chunk. Bounded buffers add child fibers and explicit ownership of queued values. Early consumer cancellation disposes the producer and buffered resource leases. Fan-out or replay requires fresh producer executions or application-defined buffering/copying; one-shot continuations cannot be cloned into a broadcast stream.

**UNTESTED — Iteration implementation.** The prototype has no generator/stream adapter or detachable owner API. Its ordinary handle correctly revokes an escaped continuation; pretending that such an escaped token is already a usable generator would violate the lifetime rule. The owner-preserving protocol needs an additional experiment.

## 8. Mapping to Effect 4.0.5 and implementation strategies

| Status | Existing mechanism | Evidence and practical limit |
| --- | --- | --- |
| VERIFIED | Reified fiber continuation | FiberImpl has context, `_stack`, interruptibility, pending/deferred interrupts, yielded state, child/parent sets and observers (`internal/effect.ts:487–538`). Public Fiber exposes context/setContext and lifecycle methods but no stack or interruptible field (`Fiber.ts:73–90`). |
| VERIFIED | Trampoline and fairness | `runLoop` consults scheduler.shouldYield (`625–679`); `getCont` pops success/failure/ensure frames (`680–701`); success inlining is bounded by maxInlineSteps 32 (`706–712,774`). |
| VERIFIED | Error/finalizer protocol | Success/failure/ensure frame hooks exist in `internal/core.ts:39–61`; OnExit and Scope provide cleanup and restoration. There is no owner registry for detached prompt segments in the inspected FiberImpl fields. |
| VERIFIED | Async cancellation | Callback gates and asyncFinalizer already coordinate suspension, interruption and a canceler (`internal/effect.ts:1141–1207`). This is not double-resume rejection for an exposed user token. |
| VERIFIED | Generator driver | `fromIteratorUnsafe` calls iter.next, has a success continuation, and no failure continuation invoking iter.throw/return (`1413–1445`). The Effect.gen probe verifies that JS finally is skipped on expected failure. |
| VERIFIED | Masks | Effect changes interruptible and pushes restoration frames; enabling interruptibility checks pending interruption (`4557–4653`). |
| VERIFIED | Fork/join | Child Context inheritance and scheduling are at `5527–5549`; forkChild installs interruption middleware (`5491–5524`, completion invokes middleware at `600–606`); forkIn ties cancellation to scope (`5604–5646`). Join returns the exit or observes it without replacing Context (`844–851`). |
| VERIFIED | Import surface | Installed `kernel/node_modules/effect/package.json:57` maps `./internal/*` to null. Accessing `_stack` structurally through withFiber is private implementation coupling even if its property name is discoverable. |

**PROPOSED — Required native extension.** Introduce supported Prompt, Owner and ResumeReturn frame kinds plus capture/claim/dispose operations. They must detach frames without executing ensure/OnExit hooks, store both capture and prompt-entry dynamic state, register detached ownership for cancellation, reactivate fiber caches with Context, restore masks without erasing pending interruption, and unwind detached frames with the runtime's real finalizer protocol. Associate children with owning regions and await them before resource release. Add correlated capture, claim, delivery-rejection, cleanup-start/end and owner-exit diagnostics. Merely slicing `_stack` and calling a saved success callback does not supply these invariants.

### (a) Effect-native prompt frames slicing fiber._stack

**PROPOSED — Evaluation.** This is the preferred integration endpoint for transparent control through reified Effect computations. Capture slices from the matching runtime prompt; resumption restores the segment on the same fiber above ResumeReturn. It reuses Effect's trampoline, scheduler and cancellation machinery. Compiler-generated language finally uses explicit cleanup frames. Ordinary synchronous JS activations remain uncapturable, even with native prompts.

**UNTESTED — Native risks.** No native prompt extension is implemented here. Detached OnExit frames must run on disposal, their restore order must agree with saved dynamic state, and a suspended segment must remain discoverable when cancellation unwinds only the active stack. Private `_yielded`, contAll mutation, pending interrupts and callback reentry require version-specific validation. Existing foreign generators' JS finally semantics do not improve automatically. A structural private-API prototype could pass simple resume tests while leaking a detached scope; all seven required tests must be rerun against an actual extension before equivalence is claimed.

### (b) One child fiber per prompt using public APIs

**PROPOSED — Evaluation.** A worker child can run the body, send an operation request, await a response, and let the parent execute its clause in the outer Context. The child's suspended native stack keeps its Scope and Context; cancellation can interrupt and await it. Effect provides stack-safe scheduling for both fibers. Channels and request IDs can give safe one-shot response ownership and late-release protocols.

**PROPOSED — Semantic mismatch.** The clause and resumed body now have different fiber identities and child trees, and interruption is relayed instead of being the owning fiber's single event. An operation aimed at an outer prompt must route across intermediate workers. To model `resume` as a nested expression returning a body result while supporting subsequent operations requires a recursive request/response protocol, not one Deferred. It is a possible implementation of a language that deliberately hides those identities, but fails this design's observable same-fiber requirement. Adding a virtual language identity would not make foreign Effect.withFiber observers agree. Cancellation must await the worker before handle returns; otherwise abandonment leaks.

**UNTESTED — Child strategy.** No per-prompt child implementation or benchmark is included. It is not ruled out for an explicitly actor-like handler facility; it is not demonstrated equivalent to the specified continuation core.

### (c) Generator kernel inside one Effect fiber

**PROPOSED — Evaluation.** The compiler can turn each activation into a generator that yields machine instructions. A kernel must explicitly keep an activation stack, find prompts, move generator activations once, restore Context snapshots, and dispose cleanup frames. Language calls must yield Call instructions rather than recursively `yield*` nested generators, or 100000 calls can still overflow the JS stack. Fairness requires a kernel step budget; Effect cannot count an unbounded loop of synchronous `.next` calls hidden inside one primitive.

**PROPOSED — Cleanup constraint.** All language catch/finally paths must lower to explicit machine frames or a verified generator unwinding protocol. `iterator.return()` can itself yield from finally; calling it once is insufficient. `Effect.gen` alone drops iterators on failure, and manually deleting generators on abandonment leaks language cleanup. The compiler also has to arrange that a clause runs outside the captured generators while their resources stay live.

**UNTESTED — Generator strategy.** A complete generator kernel has not been built here. It could implement this semantics with a suitable explicit machine, but ordinary Effect.gen is not evidence that it does. The experiment verifies the native finally limitation, not the impossibility of a custom generator implementation.

### (d) CPS and explicit state machines

**PROPOSED — Evaluation.** Defunctionalize calls and continuations into instructions, live-local records and tagged frames. Dispatch through a bounded trampoline in a single Effect fiber. This can implement the specified same-fiber prompts without private runtime access, provided all capturable language activations are compiled into that machine. Naive recursive CPS callbacks are insufficient: tail calls need a trampoline and dynamic state/cleanup still need explicit representations.

**VERIFIED — Implemented choice.** `kernel/kernel.ts` is this strategy. It moves arrays of explicit frames at perform/resume, installs Effect Context only around bridge effects, makes real public Scope values, masks administrative transitions, and restores interruptibility around work and scheduler checkpoints. It does not slice Effect's private stack. Its 256-step budget and Effect.yieldNow checkpoints allow the recursion test's timer to run. The test makes 100001 calls and 100001 captures/resumptions at nesting depth 100000 without JS stack overflow. The full suite takes 17.95 seconds on the final run; no throughput guarantee follows.

**VERIFIED — Complexity limit.** Capture and reinstatement copy frame-array entries proportional to segment depth. A deeply nested program that repeatedly captures on return can incur quadratic total copying, as this test does. Owner registries retain token metadata until owner exit. The prototype is stack-safe, not an optimized continuation representation.

**PROPOSED — Representation improvement.** Segmented stacks or linked segments with cached prompt links can make detach/reattach cheaper; that is an optimization only if ownership, unwind ordering and scheduler behavior remain identical. The language should use the explicit machine as an executable semantics now and seek supported Effect-native prompts for full interop. It need not postpone the continuation contract until a native extension exists.

### Direct-style JavaScript compilation constraints

**PROPOSED — When direct JS is legal.** A function can compile to a normal synchronous JS call only if the compiler proves that its whole reachable activation cannot suspend, capture, asynchronously fail or be required as a resumable frame. This requires a control/operational analysis distinct from A/E/R, including callbacks and dynamic dispatch. Synchronous mutation alone does not prohibit direct code, but its ordering, exceptions, cancellation checkpoints and cold-program construction behavior must remain equivalent. E=never and R=never are inadequate evidence: sleep and Effect.sync demonstrate the missing operational distinctions.

**PROPOSED — ABI boundary.** A potentially suspending function must compile to kernel instructions, a trampoline/CPS protocol or Effect-native reified control; it cannot return an ordinary A on a synchronous JS stack while later resuming the caller. Compiled callbacks passed into untransformed synchronous Array.map cannot suspend in place. They need a compiler-owned iteration lowering or an explicitly effectful foreign callback ABI. Foreign Effect functions get adapters returning Effect values; those values can be lifted and their outcomes awaited, but this kernel cannot capture their internal frames. Native prompt support could extend capture across reified Effect frames, not arbitrary foreign JS frames.

**PROPOSED — Author-facing API.** Capture/resume can initially be compiler/runtime-only machinery. Users still observe the contract through handlers, errors, resources, async operations and program combinators. A public unrestricted continuation type is not required for the first syntax surface; a public borrowed one-shot capability can be added when lifetime checking and diagnostics exist. Absence of author syntax does not permit weakening one-shot ownership or cleanup semantics.

## 9. Cold programs, retry, race and all

**PROPOSED — Separation.** `Program<A,E,R>` denotes a cold reusable plan; `Continuation` denotes one suspended execution and is affine, fiber-local and region-bound. A language function call executes in the current computation; an explicit program-construction expression packages a thunk/description instead. Calling a function that returns a Program value returns that value; it does not secretly run it. Construction must not execute the packaged body, acquire its resources, or perform provider lookups intended for execution time.

**PROPOSED — Retry.** Retry runs a fresh instantiation for each attempt after the previous attempt's cleanup. It retries selected expected failures according to its policy, not arbitrary defects or cancellation. Fresh frames, prompt owners, acquisition registrations and child roots are created; previously consumed tokens remain consumed. Retrying can repeat external writes and does not roll back shared objects. A captured continuation cannot be converted into a retryable program by putting `resume(k)` inside a thunk.

**PROPOSED — Race.** Race starts fresh executions in sibling fibers supervised by an owning region. Its chosen success/first-exit policy is explicit; after selecting a winner it interrupts and awaits losing executions and releases any undelivered resource values before publishing the result. If the winner returns a resource-bearing value, its lease must be transferred to the caller's region before the winner's temporary scope closes. Returning an ordinary object reference is not enough to extend resource lifetime.

**PROPOSED — All.** All starts fresh executions with a concurrency policy. Under fail-fast semantics one failed child triggers cancellation and joining of the others before the group exits; an accumulating variant keeps independent exits. Input order determines result order, not execution order. Reusing one cold description across children creates multiple executions, while its closed-over mutable objects may still be shared. Scope-safe resource results need explicit lease promotion or a caller-owned acquisition scope.

**VERIFIED — Prototype boundary.** `Program` nodes are reusable descriptions and `run` creates a new machine, root scope and registry on each execution. Fork/join are implemented. The kernel's Program type deliberately erases E/R, and it is not a static effect checker. It has no retry/race/all combinators or resource-lease promotion; those rules are specifications rather than tested features.

## 10. Reproduction, results and falsifiable claims

**VERIFIED — Reproduction.** The following commands run from this design directory; installation was already performed and `kernel/bun.lock` pins the dependency. Bun used for the recorded run is 1.4.1 (4661e494f).

```sh
# VERIFIED: commands used for installation and the test suite
cd kernel
bun install
bun test
```

| Status | Requested experiment | Result and actual observation |
| --- | --- | --- |
| VERIFIED | (i) Nested calls, outer handler, Context | PASS: inner service survives resumption; clause sees outer service; all observations share one fiber ID. |
| VERIFIED | (ii) Scope lifetime | PASS: captured scope remains open in clause; resumed body runs; release occurs once. |
| VERIFIED | (iii) Double resume | PASS: second use produces ContinuationUseError as a defect. |
| VERIFIED | (iv) Handler returns without resume | PASS: captured release completes before post-handle code; escaped token is revoked. |
| VERIFIED | (v) Interruption while captured | PASS: async canceler, clause release, captured release, outer release; token becomes Canceled. |
| VERIFIED | (vi) Cancellation versus delivery | PASS: controlled cancellation-first schedule releases late resource before cancellation completes; delivery-first schedule releases normally once. |
| VERIFIED | (vii) Deep recursion | PASS: 100000 nested calls, 100001 operation captures/resumptions, expected result and a serviced timer. |
| VERIFIED | Additional probes | PASS: deep handling; typed failure/defect/discontinue separation; nested failure cleanup; explicit abandonment; failed-owner cancellation before outer catch; fork Context and handler isolation; child-before-parent cleanup; masked async cancellation; immediate pending-interruption check after unmask; finalizer defect aggregation; native Effect.gen finally behavior; native callback late-resume behavior. |
| VERIFIED | Full recorded suite | 20 pass, 0 fail, 49 assertions, 1 file, 17.95 seconds. Output is saved in `kernel/test-results.txt`. Earlier runs had 17 and 19 passing tests; three added boundary probes and strengthened resource-race scenarios produce this final suite. No failed test run was observed. |

**VERIFIED — Implementation visibility.** The tests execute the proposed kernel, not merely ordinary Effect examples. Only the final two explicitly named Effect behavior probes test native Effect independently. The kernel uses public APIs and imports only from `effect`. Its IR nodes and callback signatures use erased types and host closures; Bun executes them without a static TypeScript checking step. They are a runtime prototype, not a compiler or type-sound implementation.

**UNTESTED — Remaining acceptance scope.** There is no native prompt extension, parser/compiler, lifetime checker, full service graph implementation, foreign-frame capture, generator/stream protocol, retry/race/all supervisor, exhaustive race explorer, adversarial callback protocol test, custom-scheduler/span conformance suite or foreign-mask equivalence test. Passing this suite establishes these 20 observations for this machine and installed Effect version, not equivalence of every strategy above.

| Status | Falsifiable claim | Minimal experiment and falsifier |
| --- | --- | --- |
| VERIFIED | Kernel Context snapshot matches its tested scenario. | Run test (i); an inner result other than `inner:42`, a clause service other than `outer`, or distinct fiber IDs falsifies it. |
| VERIFIED | Capture alone does not finalize the tested inner scope. | Run (ii); any close before resume, or a close count different from one, falsifies it. |
| VERIFIED | The tested token cannot run twice. | Run (iii); successful second resume or repeated body side effects falsifies it. |
| VERIFIED | Owner return disposes its unused segment synchronously in the computation sense. | Run (iv); a post-handle event preceding close, or an escaped Fresh token, falsifies it. |
| VERIFIED | Suspended owner cancellation respects tested cleanup order. | Run (v); wrong event order, missing release, or completion before release falsifies it. |
| VERIFIED | The specified resource adapter releases a one-delivery cancellation loser. | Run both (vi) schedules; zero/two releases, continuation use after canceled delivery, or cancellation completion preceding release falsifies it. |
| VERIFIED | This machine survives depth 100000 with every activation suspending. | Run (vii); RangeError, wrong count/result or timer starvation falsifies it. A slower run alone falsifies no performance bound because none was claimed. |
| VERIFIED | Kernel catches distinguish the tested expected error and defect. | Run the typed-failure probe; catching the defect with the expected-error handler falsifies it. |
| VERIFIED | The tested native Effect.gen JS finally is skipped. | Run its behavior probe; a `JS-finally` event falsifies the claim for this pinned runtime. |
| VERIFIED | A late native callback resume does not evaluate its supplied effect. | Run its behavior probe; `late-effect` appearing falsifies it. |
| VERIFIED | Pending interruption is checked before the tested call after unmasking. | Run the unmask probe; any `after:mask` event falsifies it. |
| VERIFIED | A cleanup defect does not hide the tested body failure or skip outer cleanup. | Run the finalizer-defect probe; absence of either cause or the outer close falsifies it. |
| PROPOSED | Same-fiber prompt ownership is implementable natively. | Add one native Prompt/Owner extension and run tests (i)–(v); any changed identity, premature Scope close or detached-segment leak falsifies that implementation's equivalence. |
| UNTESTED | Snapshot span binding can be restored without ending the span on capture. | Wrap a performed operation inside an inner span, observe outer span in clause and inner span after resume, and count span-end events; early end, wrong span or two ends falsifies the implementation claim. |
| UNTESTED | A native implementation can capture through a TypeScript-authored Effect.flatMap. | Place native perform inside that flatMap under a language prompt and resume once; losing its pending map or Context falsifies transparent Effect interop. The public kernel makes no such claim. |
| UNTESTED | Provider acquisition dependencies can be hidden without hiding residual method requirements. | Compile captured-Database and lookup-Database providers; acceptance of a residual lookup caller lacking Database, or requiring Database for the captured-only caller, falsifies the checker. |
| UNTESTED | Full mask restoration is correct across capture with a pending request. | Capture inside a mask, block the clause, request interruption, and try resume under both entry-mask settings; lost interruption, early masked cancellation or leaked finalizers falsifies it. |
| UNTESTED | A generator owner can survive between pulls and still close promptly. | Emit once with an open resource, request a second value, then close; an invalidated intermediate token or missing/double release falsifies the protocol. |
| UNTESTED | Program retry/race/all allocate fresh continuation ownership. | Run the same cold plan twice and in a race, record owner IDs and releases; token reuse, shared activation locals, an unreleased loser or premature lease close falsifies the implementation. |
| UNTESTED | Reentrant callback completion can be made stack safe across fibers. | Link many callback-waiting fibers so each synchronous delivery wakes the next; a JS stack overflow falsifies the scheduler/adapter's adequacy. The within-fiber recursion test does not test this. |
| PROPOSED | Lifetime rejection and runtime revocation agree. | Compile a handle returning k and reject it; bypass typing and attempt use after handle exit; an accepted typed escape or a live runtime token falsifies the intended lifetime discipline. |
| UNTESTED | Segmented stacks preserve behavior while avoiding quadratic copying. | Replace frame arrays, rerun the suite, then measure two doubled nesting depths with a capture at each return; changed outcomes/cleanup or unchanged quadratic copying falsifies that optimization claim. |

**PROPOSED — Final choice.** Keep the one-shot, same-fiber, snapshot-and-region semantics fixed. Use an explicit machine as the executable reference and early compiler target; pursue a supported Effect-native prompt extension for transparent Effect control. Expose ordinary functions and honest Effect contracts first, with the continuation machinery present from the beginning even if raw tokens remain internal.
