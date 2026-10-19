import assert from "node:assert/strict"
import { Effect } from "effect"
import { interpolatedCall, literal, effectfulOperand, localTemplate } from "./out/main.js"
import { ran } from "./out/sql.js"
await Effect.runPromise(literal())
await Effect.runPromise(interpolatedCall())
await Effect.runPromise(effectfulOperand())
await Effect.runPromise(localTemplate())
assert.deepEqual(ran, ["INSERT INTO literal VALUES (1)", "INSERT INTO interpolated_call VALUES (1)", "INSERT INTO operand VALUES (2)", "INSERT INTO local VALUES (3)"])
console.log(`ran: ${JSON.stringify(ran)}`)
