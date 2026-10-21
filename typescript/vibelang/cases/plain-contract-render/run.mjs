import assert from "node:assert/strict"
import { Effect, Exit } from "effect"
import { render } from "./out/a.js"
assert.equal(Effect.runSync(render(true)), "valid")
const exit = Effect.runSyncExit(render(false))
assert(Exit.isFailure(exit))
assert.deepEqual(exit.cause.reasons.map((r) => r._tag), ["Die"])
assert(exit.cause.reasons[0].defect instanceof TypeError)
console.log("ok")
