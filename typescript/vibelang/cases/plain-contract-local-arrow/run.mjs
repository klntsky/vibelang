import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { total, totalAnnotated, throwSame } from "./out/a.js"
for (const sum of [total, totalAnnotated]) {
  assert.equal(sum([2, 3, 5]), 10)
  assert.throws(() => sum([Infinity]), { name: "TypeError", message: "not finite" })
}
const failure = { reason: "identity" }
assert.throws(() => throwSame(failure), (e) => e === failure)
const js = readFileSync(new URL("./out/a.js", import.meta.url), "utf8")
assert.doesNotMatch(js, /fnUntraced|Effect\.fail|yield\*/)
console.log("ok")
