import assert from "node:assert/strict"
import { f, messageOf, prototypeOf, parseOr, primitive, getter, revoked, prototype } from "./out/main.js"
import { failure } from "./out/fixtures.js"
assert.throws(() => f(primitive), (e) => e === failure)
assert.throws(() => messageOf(getter), (e) => e === failure)
assert.throws(() => f(revoked), TypeError)
assert.throws(() => prototypeOf(prototype), (e) => e === failure)
assert.equal(f(1), "1")
assert.equal(parseOr("{"), "syntax")
assert.equal(parseOr("{}"), "parsed")
console.log("ok")
