// Q8: JS finally / try-catch / using inside Effect.gen, Effect.fn, fnUntraced, fnUntracedEager.
import { Effect, Exit, Fiber } from "effect"

type Body = (log: Array<string>) => Generator<any, any, any>
const makers: Record<string, (body: Body) => (log: Array<string>) => Effect.Effect<any, any>> = {
  "Effect.gen": (body) => (log) => Effect.gen(() => body(log)),
  "Effect.fn(name)": (body) => Effect.fn("q8")(body),
  "Effect.fnUntraced": (body) => Effect.fnUntraced(body),
  "Effect.fnUntracedEager": (body) => Effect.fnUntracedEager(body)
}

const disposable = (log: Array<string>, name: string) => ({
  [Symbol.dispose]() {
    log.push(`${name} disposed`)
  }
})

const cases: Record<string, Body> = {
  "(a) yielded effect fails": function*(log) {
    try {
      yield* Effect.fail("boom")
      log.push("after fail (unexpected)")
    } finally {
      log.push("finally ran")
    }
  },
  "(c) try/catch around failing yield*": function*(log) {
    try {
      yield* Effect.fail("boom")
    } catch (e) {
      log.push(`catch caught ${String(e)}`)
    }
    log.push("continued after catch")
  },
  "(c2) try/catch around a dying yield*": function*(log) {
    try {
      yield* Effect.die("defect")
    } catch (e) {
      log.push(`catch caught ${String(e)}`)
    }
  },
  "(d) using + failing yield*": function*(log) {
    using _r = disposable(log, "resource")
    yield* Effect.fail("boom")
  },
  "JS throw inside body (control)": function*(log) {
    try {
      yield* Effect.void
      throw new Error("js-throw")
    } finally {
      log.push("finally ran")
    }
  },
  "normal completion with using (control)": function*(log) {
    using _r = disposable(log, "resource")
    yield* Effect.void
    return "done"
  }
}

const summarize = (exit: Exit.Exit<unknown, unknown>) =>
  Exit.isSuccess(exit)
    ? `Success(${JSON.stringify(exit.value)})`
    : `Failure[${exit.cause.reasons.map((r) => r._tag === "Fail" ? `Fail(${String(r.error)})` : r._tag === "Die" ? `Die(${String((r as any).defect).split("\n")[0]})` : "Interrupt").join(",")}]`

for (const [maker, make] of Object.entries(makers)) {
  console.log(`--- ${maker}`)
  for (const [name, body] of Object.entries(cases)) {
    const log: Array<string> = []
    const exit = await Effect.runPromiseExit(make(body)(log))
    console.log(`${name.padEnd(42)} log=[${log.join("; ")}] exit=${summarize(exit)}`)
  }
  // (b) interrupted while suspended inside the generator
  {
    const log: Array<string> = []
    const body: Body = function*(log) {
      using _r = disposable(log, "resource")
      try {
        yield* Effect.never
      } finally {
        log.push("finally ran")
      }
    }
    const exit = await Effect.runPromise(Effect.gen(function*() {
      const f = yield* Effect.forkChild(make(body)(log))
      yield* Effect.yieldNow
      yield* Fiber.interrupt(f)
      return f.pollUnsafe()!
    }))
    console.log(`${"(b) interrupted while suspended".padEnd(42)} log=[${log.join("; ")}] exit=${summarize(exit)}`)
  }
}

// how is the generator left? (state after an Effect failure)
{
  let gen: Generator<any, any, any> | undefined
  const exit = await Effect.runPromiseExit(Effect.suspend(() => {
    gen = (function*() {
      yield* Effect.fail("boom")
      return 1
    })()
    // Re-use the public Effect.gen path: hand the live iterator to the runtime.
    return Effect.gen(function*() {
      return yield* { [Symbol.iterator]: () => gen! } as any
    })
  }))
  // After the fiber ended, the generator is still suspended at the yield.
  const probe = gen!.return(undefined as any)
  console.log("generator after Effect failure: exit=", summarize(exit), "| gen.return() on it ->", JSON.stringify(probe))
}
