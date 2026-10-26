// Runtime regression probe: does P22 ("a resume in tail position drops its clause's
// owner frame") hold when the clause is a generator, as Chapter 24's for...of
// lowering (design.md lines 2408-2421) and every compiled VibeLang function
// (Effect.fnUntraced) would be?
//
// T17 proves constant stack with a flatMap clause. This probe runs the same
// loop with (a) the doc's generator clause and (b) an Effect.fnUntraced clause.
//
// 2026-10-16: the two growth probes below still hold, by construction: a
// yielded resume keeps the clause's frames alive (now 3 per element: Owner,
// Clause, Iterator). The kernel's answer is a lowering, `return tailResume(k, v)`,
// checked by the last two probes here and by K11a/K11b in tests/review-fixes.test.ts.
import { expect, test } from "bun:test"
import { Effect, Exit } from "effect"
import { type Continuation, handle, hooks, operation, resume, tailResume } from "../../packages/runtime/src/kernel/kernel.ts"

const Yield = operation<number, void>("Yield")
const N = 100_000

const measure = async (clause: (value: number, k: Continuation) => Effect.Effect<any, any, any>) => {
  let maxDepth = 0
  hooks.onResume = (fiber) => {
    maxDepth = Math.max(maxDepth, fiber._stack.length)
  }
  try {
    let sum = 0
    const counter = Effect.gen(function*() {
      for (let i = 0; i < N; i++) yield* Yield(i)
    })
    const exit = await Effect.runPromiseExit(handle("for-of", [Yield], Effect.asVoid(counter), { Yield: clause }))
    expect(Exit.isSuccess(exit)).toBe(true)
    return maxDepth
  } finally {
    hooks.onResume = undefined
  }
}

test("control: T17's flatMap clause keeps the stack flat", async () => {
  const depth = await measure((_value, k) => Effect.flatMap(Effect.void, () => resume(k, undefined)))
  console.log(`flatMap clause: max stack depth ${depth}`)
  expect(depth).toBeLessThan(20)
})

test("Chapter 24's generator clause (`return yield* resume(k, undefined)`) does NOT drop the owner frame", async () => {
  let found = 0
  const depth = await measure((line, k) =>
    Effect.gen(function*() {
      if (line % 2 === 0) found++
      return yield* resume(k, undefined)
    })
  )
  console.log(`generator clause: max stack depth ${depth} (N = ${N})`)
  // Falsifiable claim: the stack grows with N. If this assertion fails, P22
  // holds for generator clauses and this finding is wrong.
  expect(depth).toBeGreaterThan(1000)
})

test("an Effect.fnUntraced clause (what a compiled VibeLang clause would be) also grows", async () => {
  const clauseFn = Effect.fnUntraced(function*(_line: number, k: Continuation) {
    return yield* resume(k, undefined)
  })
  const depth = await measure((line, k) => clauseFn(line, k))
  console.log(`fnUntraced clause: max stack depth ${depth} (N = ${N})`)
  expect(depth).toBeGreaterThan(1000)
})

test("fixed lowering: a generator clause that RETURNS tailResume(k, v) keeps the stack flat", async () => {
  let found = 0
  const depth = await measure((line, k) =>
    Effect.gen(function*() {
      if (line % 2 === 0) found++
      return tailResume(k, undefined)
    })
  )
  expect(found).toBe(N / 2)
  console.log(`generator clause returning tailResume: max stack depth ${depth} (N = ${N})`)
  expect(depth).toBeLessThan(20)
})

test("fixed lowering: an Effect.fnUntraced clause returning tailResume(k, v) keeps the stack flat", async () => {
  const clauseFn = Effect.fnUntraced(function*(_line: number, k: Continuation) {
    return tailResume(k, undefined)
  })
  const depth = await measure((line, k) => clauseFn(line, k))
  console.log(`fnUntraced clause returning tailResume: max stack depth ${depth} (N = ${N})`)
  expect(depth).toBeLessThan(20)
})
