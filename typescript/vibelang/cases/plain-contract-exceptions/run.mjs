import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { Effect, Exit } from "effect"
import { Empty, declared, stepsSeen, contextual, inferred, callForeign, callContextual, callBuiltin, recover, Validator, half, effectful } from "./out/a.js"
const tags = (exit) => (assert(Exit.isFailure(exit)), exit.cause.reasons.map((r) => r._tag))
assert.equal(declared("x"), "x")
assert.equal(declared(""), "empty")
assert.deepEqual(stepsSeen(), ["x", ""])
for (const f of [contextual, inferred, callForeign]) {
  assert.equal(f("x"), "x")
  assert.throws(() => f(""), { name: "Error", message: "foreign" })
}
const error = new Error("identity")
assert.throws(() => callContextual(() => { throw error }), (e) => e === error)
assert.deepEqual(callBuiltin('{"ok":true}'), { ok: true })
assert.equal(callBuiltin("{"), "SyntaxError")
assert.equal(recover(), "tagged")
assert.equal(Effect.runSync(new Validator().validate("ok")), "ok")
const invalid = Effect.runSyncExit(new Validator().validate(""))
assert.deepEqual(tags(invalid), ["Fail"])
assert(invalid.cause.reasons[0].error instanceof Empty)
assert.equal(Effect.runSync(half(4)), 2)
const negative = Effect.runSyncExit(half(-1))
assert.deepEqual(tags(negative), ["Fail"])
assert.equal(negative.cause.reasons[0].error, "negative")
assert.deepEqual(tags(Effect.runSyncExit(effectful())), ["Die"])
const dts = readFileSync(new URL("./out/a.d.ts", import.meta.url), "utf8")
assert.match(dts, /declared\(value: string\): string;/)
assert.match(dts, /callBuiltin: \(text: string\) => unknown;/)
assert.match(dts, /effectful\(\): Effect(?:\.Effect)?<string, never, never>/)
console.log("ok")
