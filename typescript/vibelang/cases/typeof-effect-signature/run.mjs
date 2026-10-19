import assert from "node:assert/strict"
import { Effect } from "effect"
import { message } from "./out/a.js"
import { Failure, run } from "./out/chain.js"
assert.equal(message(new Failure({ message: "failed" })), "failed")
assert.equal(Effect.runSync(run(false)), 42)
const error = Effect.runSync(Effect.flip(run(true)))
assert.equal(error._tag, "Failure")
assert.equal(error.message, "failed")
console.log("ok")
