import assert from "node:assert/strict"
import { Effect } from "effect"
import { one, two, three } from "./out/a.js"
assert.equal(Effect.runSync(one()), 1)
assert.equal(Effect.runSync(two({ value: 2 })), 2)
assert.equal(Effect.runSync(three()), 3)
console.log("ok")
