import { expect, test } from "bun:test"
import { Context, Effect, Exit } from "effect"
import { handle, operation, resume, resumeWith } from "../../../packages/runtime/src/kernel/kernel.ts"
import { defect, note, sync, value } from "./helpers.ts"

class Local extends Context.Service<Local, string>()("conformance/Local") {}

test("F01 §1.5 OnExit restore closures restore the body and then the resume-site Context", () => {
  const Ask = operation<void, number>("F01")
  const seen: string[] = []
  const log = Effect.flatMap(Local, x => note(seen, x))
  const body = Effect.gen(function*() {
    yield* Effect.andThen(Ask(), log).pipe(Effect.provideService(Local, "inner"))
    yield* log
    return 8
  }).pipe(Effect.provideService(Local, "body"))
  const exit = sync(handle("h", [Ask], body, {
    F01: (_, k) => Effect.gen(function*() {
      yield* log
      const result = yield* Effect.andThen(resume(k, 1), log).pipe(Effect.provideService(Local, "resumer"))
      yield* log
      return result
    })
  }).pipe(Effect.provideService(Local, "outer")))
  expect(value(exit)).toBeUndefined()
  expect(seen).toEqual(["outer", "inner", "body", "resumer", "outer"])
})

test("F02 §1.3/1.5 setInterruptible contAll frames travel with capture", () => {
  const Ask = operation<void, void>("F02")
  const seen: boolean[] = []
  const log = Effect.withFiber(f => note(seen, f.interruptible))
  const body = Effect.gen(function*() {
    yield* Effect.uninterruptible(Effect.andThen(Ask(), log))
    yield* log
  })
  const exit = sync(handle("h", [Ask], body, {
    F02: (_, k) => Effect.andThen(log, Effect.andThen(resume(k, undefined), log))
  }))
  expect(Exit.isSuccess(exit)).toBe(true)
  expect(seen).toEqual([true, false, true, true])
})

test("F03 §1.5 a nested different-handler prompt moves with the segment", () => {
  const A = operation<void, number>("F03A")
  const B = operation<void, number>("F03B")
  const body = handle("B", [B], Effect.gen(function*() {
    return (yield* A()) + (yield* B())
  }), { F03B: (_, k) => resume(k, 20) }, n => Effect.succeed(n * 2))
  const exit = sync(handle("A", [A], body, { F03A: (_, k) => resume(k, 1) }))
  expect(value(exit)).toBe(42)
})

test("F04 §1.5 resuming A from a newly installed B clause does not rebind absent snapshot evidence", () => {
  const A = operation<void, string>("F04A")
  const B = operation<void, string>("F04B")
  const seen: string[] = []
  const log = Effect.flatMap(Local, x => note(seen, x))
  const body = Effect.gen(function*() {
    yield* A()
    yield* log
    return yield* B()
  }).pipe(Effect.provideService(Local, "body"))
  const exit = sync(handle("A", [A], body, {
    F04A: (_, ka) => handle("B", [B], B(), {
      F04B: (_, kb) => Effect.gen(function*() {
        yield* log
        const result = yield* resume(ka, "a")
        yield* log
        return yield* resume(kb, result)
      }).pipe(Effect.provideService(Local, "B-clause"))
    })
  }).pipe(Effect.provideService(Local, "outer")))
  // B at the perform site was absent at capture; the resume-site B is not rebound.
  expect(defect(exit)).toBe("UnhandledOperation")
  expect(seen).toEqual(["B-clause", "body"])
})

test("F05 §1.3/1.7 resumeWith defect discards Iterator and typed catches; Final runs masked", () => {
  const Ask = operation<void, number>("F05")
  const seen: string[] = []
  const body = Effect.gen(function*() {
    const n = yield* Ask()
    yield* note(seen, "continued")
    return n
  }).pipe(
    Effect.catch(() => Effect.as(note(seen, "caught"), -1)),
    Effect.onExit(ex => Effect.withFiber(f => note(seen, `${ex._tag}:${f.interruptible}`)))
  )
  const exit = sync(handle("h", [Ask], body, { F05: (_, k) => resumeWith(k, Effect.die("boom")) }))
  expect(Exit.isFailure(exit) && exit.cause.reasons.map(r => r._tag)).toEqual(["Die"])
  expect(seen).toEqual(["Failure:false"])
})

test("F06 §1.7 abandonment discards Iterator without executing its next statement", () => {
  const Ask = operation<void, void>("F06")
  const seen: string[] = []
  const body = Effect.gen(function*() {
    yield* Ask()
    yield* note(seen, "continued")
  }).pipe(Effect.ensuring(note(seen, "released")))
  const exit = sync(handle("h", [Ask], body, { F06: () => Effect.succeed("stop") }))
  expect(value(exit)).toBe("stop")
  expect(seen).toEqual(["released"])
})

test("F07 §1.3/1.7 perform inside a finalizer resumes and preserves the original Exit", () => {
  const Ask = operation<void, void>("F07")
  const seen: string[] = []
  const body = Effect.succeed(42).pipe(Effect.ensuring(Effect.gen(function*() {
    yield* Ask()
    yield* Effect.withFiber(f => note(seen, `finalizer:${f.interruptible}`))
  })))
  const exit = sync(handle("h", [Ask], body, {
    F07: (_, k) => Effect.andThen(Effect.withFiber(f => note(seen, `clause:${f.interruptible}`)), resume(k, undefined))
  }))
  expect(value(exit)).toBe(42)
  expect(seen).toEqual(["clause:true", "finalizer:false"])
})

test("F08 §1.7 abandonment with a captured inner prompt bypasses onReturn and runs Final", () => {
  const A = operation<void, void>("F08A")
  const B = operation<void, void>("F08B")
  const seen: string[] = []
  const exit = sync(handle("A", [A], handle("B", [B],
    A().pipe(Effect.ensuring(note(seen, "inner-final"))),
    { F08B: (_, k) => resume(k, undefined) }, () => note(seen, "onReturn")),
    { F08A: () => Effect.succeed("stop") }))
  expect(value(exit)).toBe("stop")
  expect(seen).toEqual(["inner-final"])
})

test("F09 §1.3/1.5 resume from an outer handler clause while its segment holds the inner Owner", () => {
  const A = operation<void, number>("F09A")
  const B = operation<void, number>("F09B")
  let ka: any
  const seen: string[] = []
  const body = handle("A", [A], Effect.andThen(A(), Effect.as(note(seen, "A-body"), 42)), {
    F09A: (_, k) => { ka = k; return B().pipe(Effect.ensuring(note(seen, "A-clause-final"))) }
  })
  const exit = sync(handle("B", [B], body, {
    F09B: (_, kb) => Effect.gen(function*() {
      const result = yield* resume(ka, 1)
      yield* note(seen, "B-clause")
      return yield* resume(kb, result)
    })
  }))
  expect(value(exit)).toBe(42)
  expect(seen).toEqual(["A-body", "B-clause", "A-clause-final"])
})
