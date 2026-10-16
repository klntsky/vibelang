// Passing tests in this file are counterexamples to assertions about Effect,
// rather than conformance assertions that the kernel should make pass.
import { expect, test } from "bun:test"
import { AsyncLocalStorage } from "node:async_hooks"
import { Cause, Context, Effect, Layer } from "effect"
import { handle, operation, resume } from "../../kernel/kernel.ts"
import { defect, run, sync, value } from "./helpers.ts"

test("D01 §2.5.3 false: Effect.callback can resume synchronously inside its own fiber", () => {
  let inside: number | undefined
  const exit = sync(Effect.withFiber(f => Effect.callback<number>(r => {
    inside = f.id
    r(Effect.succeed(f.id))
  })))
  expect(value(exit)).toBe(inside)
  expect(typeof inside).toBe("number")
})

test("D02 §1.5 false: Effect captures AsyncLocalStorage at a delayed fork, before any root wait", async () => {
  const als = new AsyncLocalStorage<string>()
  const tasks: Array<() => void> = []
  // Use the real MixedScheduler path, but control when its initial fork task runs.
  const { Scheduler } = await import("effect")
  const scheduler = new Scheduler.MixedScheduler("async", task => { tasks.push(task); return () => {} })
  const child = als.run("fork-site", () => Effect.runSync(Effect.forkDetach(
    Effect.sync(() => als.getStore())
  ).pipe(Effect.provideService(Scheduler.Scheduler, scheduler))))
  expect(child.pollUnsafe()).toBeUndefined()
  als.run("dispatcher-site", () => { while (tasks.length) tasks.shift()!() })
  // Bun supports the AsyncResource path used by this pinned Effect version.
  expect(value(child.pollUnsafe()!)).toBe("fork-site")
})

test("D03 §5.7 false API shape: zip uses concurrent:true; concurrency:2 does not fork", async () => {
  const Ask = operation<void, number>("D03")
  const clauses = { D03: (_: any, k: any) => resume(k, 42) }
  const numeric = await run(handle("h", [Ask], Effect.zip(Ask(), Effect.succeed(1), { concurrency: 2 } as any), clauses))
  const boolean = await run(handle("h", [Ask], Effect.zip(Ask(), Effect.succeed(1), { concurrent: true }), clauses))
  expect(value(numeric)).toEqual([42, 1])
  expect(defect(boolean)).toBe("HandlerNotOnThisFiber")
})

test("D04 §1.7 overbroad: an Interrupt cause alone does not make Effect skip catchCause", () => {
  const exit = sync(Effect.catchCause(Effect.interrupt, c => Effect.succeed(Cause.hasInterrupts(c))))
  expect(value(exit)).toBe(true)
})

test("D05 §2.3/5.5 overbroad: sibling provides can share a prebuilt ambient MemoMap", () => {
  class Resource extends Context.Service<Resource, number>()("D05Resource") {}
  let builds = 0
  let releases = 0
  const live = Layer.effect(Resource, Effect.acquireRelease(Effect.sync(() => ++builds), () => Effect.sync(() => { releases++ })))
  const exit = sync(Effect.scoped(Effect.gen(function*() {
    const scope = yield* Effect.scope
    const memo = yield* Layer.makeMemoMap
    yield* Layer.buildWithMemoMap(live, memo, scope)
    const results = yield* Effect.all([Effect.provide(Resource, live), Effect.provide(Resource, live)])
      .pipe(Effect.provideService(Layer.CurrentMemoMap, memo))
    return { results, builds, releasesDuringProgram: releases }
  })))
  expect(value(exit)).toEqual({ results: [1, 1], builds: 1, releasesDuringProgram: 0 })
  expect(releases).toBe(1)
})
