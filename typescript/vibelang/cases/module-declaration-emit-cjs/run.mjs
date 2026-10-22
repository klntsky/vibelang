import assert from "node:assert/strict"
import { createRequire } from "node:module"
import { Effect } from "effect"

const require = createRequire(import.meta.url)
const exports = require("./out/main.js")
assert.deepEqual(
  ["tidy", "pathJoin", "one", "two", "textNs", "two words", "__proto__", "Counter"].map((name) => typeof exports[name]),
  ["function", "function", "number", "function", "object", "string", "string", "function"],
)
assert.equal(exports.tidy(" ab "), "abab")
assert.equal(exports.pathJoin("a", "b"), "a/b")
assert.equal(exports.one, 1)
assert.equal(exports.two(), 2)
assert.equal(exports.textNs.sep, "/")
assert.equal(exports.textNs.join("a", "b"), "a/b")
assert.equal(exports.textNs.double("x"), "xx")
assert.equal(exports["two words"], "more:")
assert.equal(exports["__proto__"], "more:")
assert.equal(Object.hasOwn(exports, "__proto__"), true)
assert.equal(Object.getPrototypeOf(exports), Object.prototype)
assert.equal(new exports.Counter().increment(), 1)
assert.equal(Object.hasOwn(exports, "default"), false)
await Effect.runPromise(exports.program())
console.log("reexports ok")
