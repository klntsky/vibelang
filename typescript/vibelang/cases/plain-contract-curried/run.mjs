import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { Effect } from "effect"
import { nested, nestedTyped, nestedMask, currentDepth } from "./out/a.js"
for (const build of [nested, nestedTyped]) {
  const run = build(2)
  assert.equal(typeof run, "function")
  assert.equal(Effect.runSync(run(Effect.succeed(7))), 7)
  assert.equal(Effect.runSync(run(currentDepth())), 2)
  assert.equal(Effect.runSync(run(build(3)(currentDepth()))), 5)
}
assert.equal(Effect.runSync(nestedMask()(Effect.succeed(9))), 9)
const dts = readFileSync(new URL("./out/a.d.ts", import.meta.url), "utf8")
assert.match(dts, /nested: \(step: number\) => <A, E, R>\(effect: Effect.Effect<A, E, R>\) => Effect.Effect<A, E, R>/)
console.log("ok")
