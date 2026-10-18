import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
const js = readFileSync(new URL("./out/a.js", import.meta.url), "utf8")
assert.doesNotMatch(js, /fnUntraced|Effect\.fail|yield\*/)
import { render } from "./out/a.js"
assert.equal(render(true), "valid")
assert.throws(() => render(false), { name: "TypeError", message: "invalid" })
console.log("ok")
