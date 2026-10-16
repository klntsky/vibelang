import { expect, test } from "bun:test"
import { Cause, Context, Deferred, Fiber, Stream } from "effect"
import {
  $bind, $catch, $control, $finally, $fork, $forOf, $forStream, $normal,
  $produce, $program, $programs, $reference, $region, $service, $tailResume,
  $tailResumeWith, $toStream, $trusted, $using, $yield, abandon, Control,
  Effect, Exit, fork, handle, handoff, Layer, pipe, Producer, resume,
  resumeWith, runMain, Schema, Scope, suspend, TaggedError, wire
} from "@effectscript/runtime"

test("calls: programs are lazy and rerunnable; bind accepts data and Effects", async () => {
  let runs = 0
  const program = $program(() => Effect.sync(() => ++runs))
  const programs = $programs([() => program, () => $bind(9)])
  const record = $programs({ a: () => program, b: () => $bind("b") })
  expect(runs).toBe(0)
  expect(await Effect.runPromise(Effect.all(programs))).toEqual([1, 9])
  expect(await Effect.runPromise(Effect.all(record))).toEqual({ a: 2, b: "b" })
  expect(await Effect.runPromise($bind(program))).toBe(3)
  expect(await runMain(Effect.fail("expected"))).toEqual(Exit.fail("expected"))
})

test("regions: catch handles expected failures and leaves defects and mixed causes alone", async () => {
  let caught = 0
  const recover = (e: string) => Effect.sync(() => { caught++; return `caught:${e}` })
  expect(Effect.runSync($catch(Effect.fail("no"), recover))).toBe("caught:no")
  const defect = new Error("defect")
  const mixed = Cause.combine(Cause.fail("no"), Cause.die(defect))
  for (const cause of [Cause.die(defect), mixed]) {
    const exit = await Effect.runPromiseExit($catch(Effect.failCause(cause), recover))
    expect(Exit.isFailure(exit)).toBe(true)
    if (Exit.isFailure(exit)) expect(exit.cause).toEqual(cause)
  }
  expect(caught).toBe(1)
})

test("regions: finally runs and completes when its body is interrupted", async () => {
  const events: string[] = []
  await Effect.runPromise(Effect.gen(function* () {
    const started = yield* Deferred.make<void>()
    const child = yield* Effect.forkChild($finally(
      Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
      Effect.andThen(Effect.yieldNow, Effect.sync(() => { events.push("finally") }))
    ))
    yield* Deferred.await(started)
    yield* Fiber.interrupt(child)
    events.push("joined")
  }))
  expect(events).toEqual(["finally", "joined"])
})

test("regions: using releases resources and Disposables in reverse acquisition order", () => {
  const events: string[] = []
  const acquire = (name: string) => Effect.acquireRelease(
    Effect.sync(() => { events.push(`acquire:${name}`); return name }),
    () => Effect.sync(() => { events.push(`release:${name}`) })
  )
  Effect.runSync($region(scope => Effect.gen(function* () {
    yield* $using(scope, acquire("outer"))
    yield* $using(scope, Effect.succeed({ [Symbol.dispose]() { events.push("dispose") } }))
    yield* $using(scope, acquire("inner"))
    events.push("body")
  })))
  expect(events).toEqual(["acquire:outer", "acquire:inner", "body", "release:inner", "dispose", "release:outer"])
})

test("fork: exiting a block interrupts its child and waits for cleanup", async () => {
  const events: string[] = []
  await Effect.runPromise($region(scope => Effect.gen(function* () {
    const started = yield* Deferred.make<void>()
    yield* $fork($finally(
      Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
      Effect.sync(() => { events.push("child:closed") })
    ), scope)
    yield* Deferred.await(started)
    events.push("block:exit")
  })))
  expect(events).toEqual(["block:exit", "child:closed"])
  await Effect.runPromise(Effect.scoped(Effect.asVoid(fork(Effect.void))))
})

interface Settings { readonly value: number }
class Settings extends $service<Settings>("runtime-test/Settings") {}
interface Doubled { readonly value: number }
class Doubled extends $service<Doubled>("runtime-test/Doubled") {}

test("services: service keys and dependent-first wire provide implementations", async () => {
  const settings = Layer.succeed(Settings, { value: 21 })
  const doubled = Layer.effect(Doubled, Effect.map(Settings, s => ({ value: s.value * 2 })))
  const program = Effect.map(Doubled, d => d.value)
  expect(await Effect.runPromise(Effect.provide(program, wire(doubled, settings)))).toBe(42)
})

interface Flags { readonly beta: boolean }
class Flags extends $reference<Flags>("runtime-test/Flags", () => Flags.default) {
  static readonly default: Flags = { beta: false }
}

test("services: a reference key reads its class's default until a provider overrides it", async () => {
  const beta = Effect.map(Flags, f => f.beta)
  expect(await Effect.runPromise(beta)).toBe(false)
  expect(await Effect.runPromise(Effect.provideService(beta, Flags, { beta: true }))).toBe(true)
})

interface Ask { answer(question: string): Effect.Effect<number> }
class Ask extends $service<Ask>("runtime-test/Ask") {}
$control(Ask, ["answer"])
const asking = Effect.gen(function* () { return (yield* (yield* Ask).answer("life")) + 1 })

test("handlers: resume reinstalls the continuation and onReturn transforms its result", async () => {
  expect(await Effect.runPromise(handle(asking, Ask, {
    answer(question, k) {
      expect(question).toBe("life")
      return resume(k, 41)
    }
  }))).toBe(42)
  expect(await Effect.runPromise(handle(asking, Ask, {
    answer: (_, k) => resumeWith(k, Effect.succeed(20))
  }, value => Effect.succeed(value * 2)))).toBe(42)
})

test("handlers: tail resumption and abandonment preserve their generated shapes", async () => {
  expect(await Effect.runPromise(handle(asking, Ask, {
    answer: (_, k) => Effect.succeed($tailResume(k, 41))
  }))).toBe(42)
  expect(await Effect.runPromise(handle(asking, Ask, {
    answer: (_, k) => Effect.succeed($tailResumeWith(k, Effect.succeed(41)))
  }))).toBe(42)
  const events: string[] = []
  const program = $finally(asking, Effect.sync(() => { events.push("closed") }))
  const exit = await Effect.runPromiseExit(handle(program, Ask, {
    answer: (_, k) => Effect.andThen(abandon(k), Effect.succeed(0))
  }))
  expect(exit).toEqual(Exit.succeed(0))
  expect(events).toEqual(["closed"])
})

const numbers = () => $produce(() => Effect.gen(function* () {
  yield* $yield(1)
  yield* $yield(2)
  yield* $yield(3)
}))

test("producers: forOf consumes a cold Producer and break closes it", async () => {
  const events: number[] = []
  let closed = false
  const producer = $produce(() => $finally(numbers().body(), Effect.sync(() => { closed = true })))
  expect(producer).toBeInstanceOf(Producer)
  expect(closed).toBe(false)
  await Effect.runPromise($forOf(producer, value => Effect.sync(() => {
    events.push(value)
    return value === 2 ? { kind: "break", label: "" } : $normal
  })))
  expect(events).toEqual([1, 2])
  expect(closed).toBe(true)
})

test("producers: a producer is consumed through Stream and forStream stops at break", async () => {
  expect(await Effect.runPromise(Stream.runCollect($toStream(numbers())))).toEqual([1, 2, 3])
  const seen: number[] = []
  await Effect.runPromise($forStream(Stream.make(1, 2, 3), value => Effect.sync(() => {
    seen.push(value)
    return { kind: "break", label: "" }
  })))
  expect(seen).toEqual([1])
})

test("boundaries: trusted throws become failures only for declared exceptions", () => {
  const trusted = $trusted<SyntaxError>()
  const failure = Effect.runSyncExit(trusted(() => { throw new SyntaxError("bad") }, ["SyntaxError"]))
  expect(Exit.isFailure(failure) && Cause.hasFails(failure.cause)).toBe(true)
  const defect = Effect.runSyncExit(trusted(() => { throw new TypeError("bad") }, ["SyntaxError"]))
  expect(Exit.isFailure(defect) && Cause.hasDies(defect.cause)).toBe(true)
  expect(Effect.runSync(trusted(() => 42, []))).toBe(42)
})

test("prelude: exports Effect's modules, pipe, and TaggedError without wrappers", () => {
  class Bad extends TaggedError("Bad")<{}> {}
  expect(new Bad()._tag).toBe("Bad")
  expect(pipe(20, n => n + 22)).toBe(42)
  expect(typeof Schema.String).toBe("function")
  expect(Scope.make).toBeDefined()
  expect(Control).toBeInstanceOf(Function)
  expect(Context.isKey(Settings)).toBe(true)
})

test("adapters: suspend and handoff deliver a synchronous callback result", async () => {
  expect(await Effect.runPromise(suspend<number>(r => { expect(r.resume(42)).toBe(true) }))).toBe(42)
  expect(await Effect.runPromise(handoff<number>(deliver => {
    deliver(42)
    return Effect.void
  }, () => Effect.void))).toBe(42)
})

test("producers: closing a Stream waits for the producer's finalizer", async () => {
  let closed = false
  const producer = $produce(() => $finally(Effect.gen(function* () {
    let n = 0
    while (true) yield* $yield(++n)
  }), Effect.sync(() => { closed = true })))
  expect(await Effect.runPromise(Stream.runCollect(Stream.take($toStream(producer), 1)))).toEqual([1])
  expect(closed).toBe(true)
})

test("regions: a failing finalizer retains both original and cleanup failures", () => {
  const exit = Effect.runSyncExit($finally(Effect.fail("body"), Effect.fail("cleanup")))
  expect(Exit.isFailure(exit)).toBe(true)
  if (Exit.isFailure(exit)) {
    const failures = exit.cause.reasons.filter(Cause.isFailReason).map(reason => reason.error)
    expect(failures).toEqual(["body", "cleanup"])
  }
})
