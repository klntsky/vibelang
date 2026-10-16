import { expect, test } from "bun:test"
import { Effect, Exit, Scope } from "effect"
import { abandon, abandonNow, handle, operation, transfer } from "../../kernel/kernel.ts"
import { summary, sync, value } from "./helpers.ts"

test("A01 §1.7 transferred continuation release defect must fail the owning Scope boundary", () => {
  const Ask = operation<void, void>("A01")
  const exit = sync(Effect.scoped(Effect.gen(function*() {
    const scope = yield* Effect.scope
    return yield* handle("h", [Ask], Ask().pipe(Effect.ensuring(Effect.die("release-defect"))), {
      A01: (_, k) => Effect.as(transfer(k, scope), "parked")
    })
  })))
  console.log("A01 observed", JSON.stringify(summary(exit)))
  expect(Exit.isFailure(exit) && exit.cause.reasons.map(r => r._tag === "Die" ? r.defect : r._tag)).toEqual(["release-defect"])
})

test("A02 §1.3 abandon fails its caller with the finalizer failure, without the abort", () => {
  // Resolution of the review finding: abandon reports cleanup failures as a
  // failure of the abandoner; abandonNow (A03) returns the full unwinding Exit.
  const Ask = operation<void, void>("A02")
  const exit = sync(handle("h", [Ask], Ask().pipe(Effect.ensuring(Effect.die("release-defect"))), {
    A02: (_, k) => abandon(k)
  }))
  expect(Exit.isFailure(exit) && exit.cause.reasons.map(r => r._tag === "Die" ? r.defect : r._tag)).toEqual(["release-defect"])
})

test("A03 §1.3 abandonNow exposes release failure and interruption to its caller", () => {
  const Ask = operation<void, void>("A03")
  const exit = sync(handle("h", [Ask], Ask().pipe(Effect.ensuring(Effect.die("release-defect"))), {
    A03: (_, k) => abandonNow(k)
  }))
  expect(Exit.isSuccess(exit)).toBe(true)
  expect(Exit.isFailure(value(exit))).toBe(true)
  expect(Exit.hasInterrupts(value(exit))).toBe(true)
  expect(Exit.isFailure(value(exit)) && value(exit).cause.reasons.some((r: any) => r._tag === "Die" && r.defect === "release-defect")).toBe(true)
})

test("A04 §1.7 transferred wholly masked abort recovery must report AbandonedContinuationContinued", () => {
  const Ask = operation<void, void>("A04")
  const exit = sync(Effect.scoped(Effect.gen(function*() {
    const scope = yield* Effect.scope
    return yield* Effect.uninterruptible(handle("h", [Ask],
      Effect.catchCause(Ask(), () => Effect.void),
      { A04: (_, k) => Effect.as(transfer(k, scope), "parked") }))
  })))
  console.log("A04 observed", JSON.stringify(summary(exit)))
  expect(Exit.isFailure(exit) && exit.cause.reasons.some(r => r._tag === "Die" && (r.defect as any)?._tag === "AbandonedContinuationContinued")).toBe(true)
})
