import assert from "node:assert/strict"
import * as Effect from "effect/Effect"
import * as programs from "./out/main.js"
let wrapped = 0
const wrap = (body) => Effect.suspend(() => { wrapped++; return body })
const results = []
for (const fn of [programs.bare, programs.called, programs.withPipe]) {
  results.push(await Effect.runPromise(fn(undefined)), await Effect.runPromise(fn(wrap)))
}
for (const fn of [programs.orChoice, programs.nullishChoice]) {
  results.push(await Effect.runPromise(fn(Effect.succeed(42), wrap)), await Effect.runPromise(fn(undefined, wrap)))
}
results.push(await Effect.runPromise(programs.commaChoice(wrap)), await Effect.runPromise(programs.withCondition(wrap)))
results.push(await Effect.runPromise(programs.plainFallback(undefined)), await Effect.runPromise(programs.plainFallback(Effect.succeed(42))))
results.push(await Effect.runPromise(programs.plainConditional(false)), await Effect.runPromise(programs.plainConditional(true)))
assert.deepEqual(results, Array(16).fill(42))
assert.equal(wrapped, 6)
console.log(JSON.stringify({ results, wrapped }))
