import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
const js = readFileSync(new URL("./out/a.js", import.meta.url), "utf8")
assert.doesNotMatch(js, /fnUntraced|Effect\.fail|yield\*/)
import { apply, line } from "./out/a.js"
assert.equal(line, "x")
assert.equal(apply((payload) => String(payload.label), { label: " y " }), "y")
const error = new Error("callback")
assert.throws(() => apply(() => { throw error }, {}), (e) => e === error)
console.log("ok")
