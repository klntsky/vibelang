import assert from "node:assert/strict"
import { Effect } from "effect"
import { A, B, Out, S, viaLet, viaDefault, viaBare, viaObject, viaConst, viaVar, partial } from "./out/main.js"

const service = {
  get: (id) => id === "ok" ? Effect.succeed(id)
    : Effect.fail(id === "a" ? new A({ id }) : new B({ code: id }))
}
const run = (f, id) => Effect.runSync(Effect.provideService(Effect.result(f(id)), S, service))
for (const f of [viaLet, viaDefault, viaBare, viaObject, viaConst, viaVar]) {
  assert.equal(run(f, "ok").success, "ok")
  for (const id of ["a", "b"]) {
    const result = run(f, id)
    assert.ok(result.failure instanceof Out)
    assert.equal(result.failure.code, id)
  }
}
assert.ok(run(partial, "only").failure instanceof Out)
assert.ok(run(partial, "other").failure instanceof B)
console.log("extractor error coverage: 20 outcomes")
