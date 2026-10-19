import assert from "node:assert/strict"
import { Effect, Exit } from "effect"
import { viaTypeQuery, viaSpelledType, viaQualifiedQuery } from "./out/a.js"
for (const parse of [viaTypeQuery, viaSpelledType, viaQualifiedQuery]) {
  assert.equal(Effect.runSync(parse("42")), 42)
  const exit = Effect.runSyncExit(parse("invalid"))
  assert(Exit.isFailure(exit))
  assert.equal(exit.cause.reasons[0].error._tag, "Bad")
}
console.log("ok")
