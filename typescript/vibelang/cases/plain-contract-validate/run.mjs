import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
const js = readFileSync(new URL("./out/a.js", import.meta.url), "utf8")
assert.doesNotMatch(js, /fnUntraced|Effect\.fail|yield\*/)
import { validate } from "./out/a.js"
assert.equal(validate("x"), "x")
assert.throws(() => validate(""), { name: "Error", message: "empty" })
console.log("ok")
