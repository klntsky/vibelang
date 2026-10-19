import assert from "node:assert/strict"
import * as Effect from "effect/Effect"
import { viaOption, viaThunk, viaCatchContinue, typedOption } from "./out/main.js"
const option = await Effect.runPromise(viaOption(0))
assert.deepEqual(option, [0, true])
assert.deepEqual(await Effect.runPromise(viaOption(-1)), [0, false])
const thunk = await Effect.runPromise(viaThunk(0))
assert.deepEqual(thunk, [0, true])
const kept = await Effect.runPromise(viaCatchContinue([-1, 0, 2]))
assert.deepEqual(kept, [0, [0, 2]])
assert.equal(await Effect.runPromise(typedOption()), true)
console.log(JSON.stringify({ option, thunk, kept }))
