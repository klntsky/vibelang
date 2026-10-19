import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import * as vibe from "./out/a.js"
import * as ts from "./out/reference.js"
assert.doesNotMatch(readFileSync(new URL("./out/a.js", import.meta.url), "utf8"), /fnUntraced|yield\*|vibelang-runtime|effect\//)
for (const { consume, repositoryCheckEnvironment } of [vibe, ts]) {
  const input = { PATH: "/bin", SMITHERS_CACHE_URL: "url", SMITHERS_CACHE_TOKEN: "token", PRIVATE: "secret" }
  const value = consume(input)
  assert.deepEqual(value, { environment: { PATH: "/bin" }, cache: { SMITHERS_CACHE_URL: "url", SMITHERS_CACHE_TOKEN: "token", SMITHERS_CACHE_READ_TOKEN: "token" } })
  assert.equal(input.SMITHERS_CACHE_URL, undefined)
  assert.equal(input.SMITHERS_CACHE_TOKEN, undefined)
  for (const partial of [{}, { SMITHERS_CACHE_URL: "url" }, { SMITHERS_CACHE_TOKEN: "token" }, { SMITHERS_CACHE_URL: "", SMITHERS_CACHE_TOKEN: "token" }, { SMITHERS_CACHE_URL: "url", SMITHERS_CACHE_TOKEN: "" }]) {
    assert.deepEqual(consume(partial).cache, {})
  }
  assert.equal(repositoryCheckEnvironment({}), undefined)
  const environment = { PATH: "/bin" }
  assert.equal(repositoryCheckEnvironment({ checkEnvironment: environment, cacheEnvironment: {} }), environment)
  assert.deepEqual(repositoryCheckEnvironment({ checkEnvironment: environment, cacheEnvironment: { TOKEN: "read" } }), { PATH: "/bin", TOKEN: "read" })
}
assert.equal(readFileSync(new URL("./out/a.d.ts", import.meta.url), "utf8"), readFileSync(new URL("./out/reference.d.ts", import.meta.url), "utf8"))
console.log("ok")
