import { Cause, Context, Effect, Exit, Fiber, Scope } from "effect"

// PROPOSED compiler IR. This deliberately erases E/R; it is not a type checker.
export type Program<A = unknown> = { readonly op: string; readonly [key: string]: any; readonly _A?: A }
export const pure = <A>(value: A): Program<A> => ({ op: "Pure", value })
export const call = <A>(body: () => Program<A>): Program<A> => ({ op: "Call", body })
export const bind = <A, B>(body: Program<A>, next: (a: A) => Program<B>): Program<B> => ({ op: "Bind", body, next })
export const lift = <A, E, R>(effect: Effect.Effect<A, E, R>): Program<A> => ({ op: "Lift", effect })
export const perform = <A = unknown>(key: string, value?: unknown): Program<A> => ({ op: "Perform", key, value })
export type Handler = (value: any, continuation: Continuation) => Program<any>
export const handle = <A>(body: Program<A>, handlers: Record<string, Handler>): Program<any> => ({ op: "Handle", body, handlers })
export const resume = <A>(k: Continuation, value: A): Program<any> => ({ op: "Resume", k, value })
export const discontinue = (k: Continuation, error: unknown): Program<any> => ({ op: "Discontinue", k, error })
export const abandon = (k: Continuation): Program<void> => ({ op: "Abandon", k })
export const fail = (error: unknown): Program<never> => ({ op: "Fail", error })
export const die = (defect: unknown): Program<never> => ({ op: "Die", defect })
export const catching = <A>(body: Program<A>, handler: (error: any) => Program<A>): Program<A> => ({ op: "Catch", body, handler })
export const provide = <A, I, S>(body: Program<A>, key: Context.Key<I, S>, value: S): Program<A> => ({ op: "Provide", body, key, value })
export const scoped = <A>(body: Program<A>): Program<A> => ({ op: "Scoped", body })
export const masked = <A>(body: Program<A>): Program<A> => ({ op: "Mask", body, interruptible: false })
export const interruptible = <A>(body: Program<A>): Program<A> => ({ op: "Mask", body, interruptible: true })
export const fork = <A>(body: Program<A>): Program<Fiber.Fiber<A, any>> => ({ op: "Fork", body })
export const join = <A>(fiber: Fiber.Fiber<A, any>): Program<A> => lift(Fiber.join(fiber))

type Dynamic = { context: Context.Context<any>; interruptible: boolean; region: Region }
type Region = { scope: Scope.Closeable; children: Set<Fiber.Fiber<any, any>>; context: Context.Context<any>; closed: boolean }
type Owner = { active: boolean; continuations: Continuation[] }
type Frame =
  | { tag: "Bind"; next: (a: any) => Program }
  | { tag: "Catch"; handler: (e: any) => Program }
  | { tag: "Restore"; dynamic: Dynamic }
  | { tag: "Region"; region: Region; dynamic: Dynamic }
  | { tag: "Owner"; owner: Owner }
  | { tag: "Prompt"; owner: Owner; handlers: Record<string, Handler>; dynamic: Dynamic }
  | { tag: "ResumeReturn"; k: Continuation; dynamic: Dynamic }

export type ContinuationState = "Fresh" | "Running" | "Completed" | "Abandoned" | "Canceled"
export class Continuation {
  state: ContinuationState = "Fresh"
  constructor(
    readonly id: number,
    readonly machine: Machine,
    readonly owner: Owner,
    public frames: Frame[],
    readonly dynamic: Dynamic
  ) {}
}
export class ContinuationUseError extends Error {
  constructor(readonly state: string) { super(`Continuation unavailable: ${state}`) }
}
export type Trace = { fiber: number; event: string; continuation?: number }

class Machine {
  stack: Frame[] = []
  dynamic!: Dynamic
  node: Program
  outcome: Exit.Exit<any, any> | undefined
  nextId = 0
  readonly regions: Region[] = []
  constructor(readonly id: number, program: Program, context: Context.Context<any>, readonly trace: Trace[]) {
    this.node = program
    const region = this.makeRegion(context)
    this.dynamic = { context: region.context, interruptible: true, region }
    this.stack.push({ tag: "Region", region, dynamic: this.dynamic })
  }
  event(event: string, k?: Continuation) { this.trace.push({ fiber: this.id, event, continuation: k?.id }) }
  makeRegion(context: Context.Context<any>): Region {
    const scope = Scope.makeUnsafe("sequential")
    const region: Region = { scope, children: new Set(), context: Context.add(context, Scope.Scope, scope), closed: false }
    this.regions.push(region)
    return region
  }
  closeRegion(region: Region, exit: Exit.Exit<any, any>): Effect.Effect<void> {
    return Effect.suspend(() => {
      if (region.closed) return Effect.void
      region.closed = true
      this.event("region:close")
      // Children finish cancellation before any resource in this region closes.
      return Effect.flatMap(Fiber.interruptAll(region.children), () => Scope.close(region.scope, exit))
    })
  }
  // Disposal evaluates no user continuation or catch clause. All close effects
  // are attempted even if a previous finalizer defects.
  disposeFrames(frames: Frame[], exit: Exit.Exit<any, any>): Effect.Effect<void> {
    const actions: Effect.Effect<void>[] = []
    for (let i = frames.length - 1; i >= 0; i--) {
      const frame = frames[i]
      if (frame.tag === "Region") actions.push(this.closeRegion(frame.region, exit))
      else if (frame.tag === "Owner") {
        frame.owner.active = false
        for (const k of [...frame.owner.continuations].reverse()) {
          if (k.state === "Fresh") actions.push(this.dispose(k, exit, "Canceled"))
        }
      } else if (frame.tag === "ResumeReturn") frame.k.state = "Canceled"
    }
    return this.allCleanup(actions)
  }
  allCleanup(actions: Effect.Effect<void>[]): Effect.Effect<void> {
    return Effect.gen(function*() {
      let cause: Cause.Cause<any> | undefined
      for (const action of actions) {
        const result = yield* Effect.exit(action)
        if (Exit.isFailure(result)) cause = cause ? Cause.combine(cause, result.cause) : result.cause
      }
      if (cause) return yield* Effect.failCause(cause)
    }) as Effect.Effect<void>
  }
  dispose(k: Continuation, exit: Exit.Exit<any, any>, state: "Canceled" | "Abandoned"): Effect.Effect<void> {
    return Effect.suspend(() => {
      if (k.state !== "Fresh") return Effect.void
      k.state = state
      this.event(`continuation:${state}`, k)
      const frames = k.frames
      k.frames = []
      return this.disposeFrames(frames, exit)
    })
  }
  cleanup(exit: Exit.Exit<any, any>): Effect.Effect<void> {
    this.event("machine:cleanup")
    const frames = this.stack.splice(0)
    return Effect.uninterruptible(this.allCleanup([
      this.disposeFrames(frames, exit),
      // Idempotent fallback also covers a region opened just before a defect.
      ...[...this.regions].reverse().map((region) => this.closeRegion(region, exit))
    ]))
  }
  accept(exit: Exit.Exit<any, any>) { this.outcome = exit }
  claim(k: Continuation) {
    if (k.machine !== this) throw new ContinuationUseError("WrongFiber")
    if (!k.owner.active || k.state !== "Fresh") throw new ContinuationUseError(k.state)
    k.state = "Running"
    this.event("continuation:claimed", k)
    const frames = k.frames
    k.frames = []
    this.stack.push({ tag: "ResumeReturn", k, dynamic: this.dynamic })
    // Avoid the JavaScript argument-count limit at 100000 captured frames.
    for (const frame of frames) this.stack.push(frame)
    this.dynamic = k.dynamic
  }
  run(restore: <A, E, R>(e: Effect.Effect<A, E, R>) => Effect.Effect<A, E, R>): Effect.Effect<any, any> {
    const bridge = (effect: Effect.Effect<any, any, any>, masked = false): Effect.Effect<any, any> => {
      const provided = Effect.provideContext(effect, this.dynamic.context)
      const current = !masked && this.dynamic.interruptible ? restore(provided) : provided
      return Effect.flatMap(Effect.exit(current), (exit) => {
        if (Exit.isFailure(exit) && Cause.hasInterrupts(exit.cause)) return Effect.failCause(exit.cause)
        this.accept(exit)
        return loop()
      })
    }
    const finish = (effect: Effect.Effect<void>, original: Exit.Exit<any, any>): Effect.Effect<any, any> =>
      Effect.flatMap(Effect.exit(effect), (closed) => {
        this.outcome = Exit.isFailure(closed)
          ? Exit.isFailure(original) ? Exit.failCause(Cause.combine(original.cause, closed.cause)) : closed
          : original
        return loop()
      })
    const loop = (): Effect.Effect<any, any> => Effect.suspend(() => {
      // The kernel has its own budget: Effect cannot count these machine steps.
      for (let budget = 0; budget < 256; budget++) {
        try {
          if (this.outcome !== undefined) {
            const result = this.outcome
            const frame = this.stack.pop()
            if (!frame) return result
            switch (frame.tag) {
              case "Bind":
                if (Exit.isSuccess(result)) { this.outcome = undefined; this.node = frame.next(result.value) }
                break
              case "Catch":
                if (Exit.isFailure(result) && result.cause.reasons.length === 1 && result.cause.reasons[0]._tag === "Fail") {
                  this.outcome = undefined; this.node = frame.handler(result.cause.reasons[0].error)
                }
                break
              case "Restore":
              case "Prompt": {
                const wasMasked = !this.dynamic.interruptible
                this.dynamic = frame.dynamic
                if (wasMasked && this.dynamic.interruptible) return Effect.flatMap(restore(Effect.void), loop)
                break
              }
              case "ResumeReturn":
                frame.k.state = "Completed"
                const wasMasked = !this.dynamic.interruptible
                this.dynamic = frame.dynamic
                this.event("continuation:completed", frame.k)
                if (wasMasked && this.dynamic.interruptible) return Effect.flatMap(restore(Effect.void), loop)
                break
              case "Region":
                this.dynamic = frame.dynamic
                return finish(this.closeRegion(frame.region, result), result)
              case "Owner": {
                frame.owner.active = false
                const abandoned = Exit.failCause(Cause.interrupt(this.id))
                return finish(this.allCleanup([...frame.owner.continuations].reverse().map((k) =>
                  this.dispose(k, Exit.isFailure(result) ? result : abandoned, Exit.isFailure(result) ? "Canceled" : "Abandoned")
                )), result)
              }
            }
            continue
          }
          const n = this.node
          switch (n.op) {
            case "Pure": this.accept(Exit.succeed(n.value)); break
            case "Call": this.node = n.body(); break
            case "Bind": this.stack.push({ tag: "Bind", next: n.next }); this.node = n.body; break
            case "Lift": return bridge(n.effect)
            case "Fail": this.accept(Exit.fail(n.error)); break
            case "Die": this.accept(Exit.die(n.defect)); break
            case "Catch": this.stack.push({ tag: "Catch", handler: n.handler }); this.node = n.body; break
            case "Provide":
              this.stack.push({ tag: "Restore", dynamic: this.dynamic })
              this.dynamic = { ...this.dynamic, context: Context.add(this.dynamic.context, n.key, n.value) }
              this.node = n.body
              break
            case "Scoped": {
              const region = this.makeRegion(this.dynamic.context)
              this.stack.push({ tag: "Region", region, dynamic: this.dynamic })
              this.dynamic = { ...this.dynamic, region, context: region.context }
              this.node = n.body
              break
            }
            case "Mask": {
              const wasMasked = !this.dynamic.interruptible
              this.stack.push({ tag: "Restore", dynamic: this.dynamic })
              this.dynamic = { ...this.dynamic, interruptible: n.interruptible }
              this.node = n.body
              if (wasMasked && n.interruptible) return Effect.flatMap(restore(Effect.void), loop)
              break
            }
            case "Handle": {
              const owner: Owner = { active: true, continuations: [] }
              this.stack.push({ tag: "Owner", owner }, { tag: "Prompt", owner, handlers: n.handlers, dynamic: this.dynamic })
              this.node = n.body
              break
            }
            case "Perform": {
              let i = this.stack.length - 1
              for (; i >= 0; i--) {
                const frame = this.stack[i]
                if (frame.tag === "Prompt" && Object.hasOwn(frame.handlers, n.key)) break
              }
              if (i < 0) throw new Error(`Unhandled operation: ${n.key}`)
              const prompt = this.stack[i] as Extract<Frame, { tag: "Prompt" }>
              const k = new Continuation(++this.nextId, this, prompt.owner, this.stack.splice(i), this.dynamic)
              prompt.owner.continuations.push(k)
              const wasMasked = !this.dynamic.interruptible
              this.dynamic = prompt.dynamic
              this.event("continuation:captured", k)
              this.node = call(() => prompt.handlers[n.key](n.value, k))
              if (wasMasked && this.dynamic.interruptible) return Effect.flatMap(restore(Effect.void), loop)
              break
            }
            case "Resume":
            case "Discontinue": {
              const wasMasked = !this.dynamic.interruptible
              this.claim(n.k)
              this.accept(n.op === "Resume" ? Exit.succeed(n.value) : Exit.fail(n.error))
              if (wasMasked && this.dynamic.interruptible) return Effect.flatMap(restore(Effect.void), loop)
              break
            }
            case "Abandon":
              if (n.k.machine !== this || n.k.state !== "Fresh") throw new ContinuationUseError(n.k.state)
              return bridge(this.dispose(n.k, Exit.failCause(Cause.interrupt(this.id)), "Abandoned"), true)
            case "Fork": {
              const region = this.dynamic.region
              return bridge(Effect.map(Effect.forkChild(run(n.body)), (fiber) => {
                region.children.add(fiber)
                fiber.addObserver(() => region.children.delete(fiber))
                return fiber
              }))
            }
            default: throw new Error(`Unknown machine operation: ${n.op}`)
          }
        } catch (defect) { this.accept(Exit.die(defect)) }
      }
      const checkpoint = this.dynamic.interruptible ? restore(Effect.yieldNow) : Effect.yieldNow
      return Effect.flatMap(checkpoint, loop)
    })
    return loop()
  }
}

export const run = <A>(program: Program<A>, trace: Trace[] = []): Effect.Effect<A, any, any> =>
  Effect.withFiber((fiber) => Effect.uninterruptibleMask((restore) => {
    const machine = new Machine(fiber.id, program, fiber.context, trace)
    return Effect.onExit(machine.run(restore), (exit) => machine.cleanup(exit))
  }))

// PROPOSED resource callback protocol: one delivery; cancelAndJoin MUST settle
// only after the producer can no longer deliver. The canceler waits for, and
// releases, a late delivery. Wrap in acquireRelease for successful transfers.
export const handoff = <A>(
  register: (deliver: (a: A) => void) => Effect.Effect<void>,
  releaseLate: (a: A) => Effect.Effect<void>
): Effect.Effect<A> => Effect.callback<A>((resumeEffect) => {
  let state: "Waiting" | "Delivered" | "Canceled" = "Waiting"
  let late: { value: A } | undefined
  const cancelAndJoin = register((value) => {
    if (state === "Canceled") { late = { value }; return }
    if (state !== "Waiting") throw new Error("Producer violated one-delivery protocol")
    state = "Delivered"
    resumeEffect(Effect.succeed(value))
  })
  return Effect.suspend(() => {
    state = "Canceled"
    return Effect.flatMap(cancelAndJoin, () => late ? releaseLate(late.value) : Effect.void)
  })
})
