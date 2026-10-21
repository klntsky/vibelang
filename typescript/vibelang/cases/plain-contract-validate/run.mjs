import assert from "node:assert/strict"
import { Effect, Exit } from "effect"
import { Empty, validate } from "./out/a.js"
assert.equal(Effect.runSync(validate("x")), "x")
const exit = Effect.runSyncExit(validate(""))
assert(Exit.isFailure(exit))
assert.deepEqual(exit.cause.reasons.map((r) => r._tag), ["Fail"])
assert(exit.cause.reasons[0].error instanceof Empty)
console.log("ok")
