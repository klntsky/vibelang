import { expect, test } from "bun:test"
import { Cause, Context, Effect, Exit } from "effect"
import type * as Kernel from "../../../packages/runtime/src/kernel/kernel.ts"

// The same behavior laws run against source in experiments and the shipped
// package entry in packages/runtime. The oracle is a finite request list,
// its arithmetic answers, and resource nesting, independent of kernel frames.
export function kernelModelLaws(label: string, K: typeof Kernel) {
  const Local = Context.Reference<string>(`t2/model/${label}`, { defaultValue: () => "outer" })
  for (const mode of ["resume", "tail", "injected failure", "implicit abandon", "explicit abandon"] as const) {
    test(`${label}: generated ${mode} histories preserve Context, resources, results, and replay`, async () => {
      let seed = 0x25c0ffee
      for (let trial = 0; trial < 32; trial++) {
        const payloads = Array.from({ length: trial * 7 % 19 + 1 }, () => {
          seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0
          return seed % 2001 - 1000
        })
        const stop = trial % payloads.length
        const stopping = mode.endsWith("abandon")
        const Op = K.operation<{ index: number; value: number }, number, string>(`model/${label}/${mode}/${trial}`)
        const events: string[] = []
        const body = Effect.gen(function*() {
          const results: number[] = []
          for (const [index, value] of payloads.entries()) {
            const result = yield* Effect.provideService(Effect.scoped(Effect.gen(function*() {
              yield* Effect.acquireRelease(
                Effect.sync(() => { events.push(`open:${index}`) }),
                () => Effect.andThen(Effect.yieldNow, Effect.sync(() => { events.push(`close:${index}`) }))
              )
              if (index % 3 === 0) yield* Effect.yieldNow
              const before = yield* Local
              const answer = yield* K.catchExpected(Op({ index, value }), error => Effect.sync(() => {
                expect(error).toBe(`injected:${value}`)
                return -value
              }))
              events.push(`answer:${index}:${before}:${yield* Local}:${answer}`)
              return answer
            })), Local, `inner:${index}`)
            results.push(result)
          }
          return results
        })
        const program = K.handle("model", [Op], body, {
          [Op.name]: ({ index, value }, k) => Effect.gen(function*() {
            events.push(`clause:${index}:${yield* Local}`)
            yield* Effect.yieldNow
            if (stopping && index === stop) {
              if (mode === "explicit abandon") yield* K.abandon(k)
              return { stopped: index }
            }
            if (mode === "tail") return K.tailResume(k, value * 3 + 1)
            return yield* (mode === "injected failure"
              ? K.resumeWith(k, Effect.fail(`injected:${value}`))
              : K.resume(k, value * 3 + 1))
          })
        }, values => Effect.sync(() => {
          events.push("returned")
          return { values }
        }))
        const expectedEvents: string[] = []
        for (const [index, value] of payloads.entries()) {
          expectedEvents.push(`open:${index}`, `clause:${index}:outer`)
          if (!stopping || index !== stop) {
            const answer = mode === "injected failure" ? -value : value * 3 + 1
            expectedEvents.push(`answer:${index}:inner:${index}:inner:${index}:${answer}`)
          }
          expectedEvents.push(`close:${index}`)
          if (stopping && index === stop) break
        }
        if (!stopping) expectedEvents.push("returned")
        const expected = stopping ? { stopped: stop }
          : { values: payloads.map(value => mode === "injected failure" ? -value : value * 3 + 1) }
        for (let replay = 0; replay < 2; replay++) {
          events.length = 0
          expect(await Effect.runPromise(program)).toEqual(expected)
          expect(events).toEqual(expectedEvents)
        }
      }
    })
  }

  test(`${label}: transfer without an owning scope fails and releases the held segment`, () => {
    const Op = K.operation<void, void>(`model/${label}/no-scope`)
    const events: string[] = []
    const exit = Effect.runSyncExit(K.handle("no-scope", [Op], Effect.ensuring(Op(), Effect.sync(() => { events.push("release") })), {
      [Op.name]: (_, k) => K.transfer(k)
    }))
    expect(Exit.isFailure(exit)).toBe(true)
    if (Exit.isFailure(exit)) {
      const defect = exit.cause.reasons.find(Cause.isDieReason)?.defect
      expect(defect).toBeInstanceOf(K.NoScopeForTransfer)
      expect((defect as Error).message).toContain("transfer needs a Scope region")
    }
    expect(events).toEqual(["release"])
  })
}
