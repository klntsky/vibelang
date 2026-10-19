import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import * as vibe from "./out/a.js"
import * as ts from "./out/reference.js"
assert.doesNotMatch(readFileSync(new URL("./out/a.js", import.meta.url), "utf8"), /fnUntraced|yield\*|vibelang-runtime|effect\//)
for (const { consume } of [vibe, ts]) {
  assert.deepEqual(consume(true), { URL: "cache" })
  assert.deepEqual(consume(false), {})
}
console.log("ok")
