import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { Effect, Exit } from "effect"
import { declared, contextual, inferred, callForeign, callTagged, callBuiltin, recover, callContextual, Validator, effectful } from "./out/a.js"
for (const f of [declared, contextual, inferred, callForeign, callTagged]) assert.equal(f("x"), "x")
for (const [f, name, message] of [
  [declared, "Error", "try"], [contextual, "Error", "contextual"],
  [inferred, "Error", "foreign"], [callForeign, "Error", "foreign"],
  [callTagged, "SyntaxError", "tagged"]
]) assert.throws(() => f(""), { name, message })
assert.throws(() => declared("finally"), { message: "finally" })
assert.deepEqual(callBuiltin('{"ok":true}'), { ok: true })
assert.throws(() => callBuiltin("{"), SyntaxError)
assert.equal(recover(), "tagged")
const error = new Error("identity")
assert.throws(() => callContextual(() => { throw error }), (e) => e === error)
assert.throws(() => new Validator().validate(""), { message: "method" })
const exit = Effect.runSyncExit(effectful())
assert(Exit.isFailure(exit))
assert.deepEqual(exit.cause.reasons.map((r) => r._tag), ["Die"])
const dts = readFileSync(new URL("./out/a.d.ts", import.meta.url), "utf8")
assert.match(dts, /effectful\(\): Effect(?:\.Effect)?<string, never, never>/)
assert.doesNotMatch(dts, /<string, Error|<string, SyntaxError/)
console.log("ok")
