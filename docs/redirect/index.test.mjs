import assert from "node:assert/strict"
import { test } from "node:test"
import worker from "./index.js"
import { readFileSync } from "node:fs"

const config = JSON.parse(readFileSync(new URL("./wrangler.jsonc", import.meta.url), "utf8").replace(/^\/\/.*$/gm, ""))
for (const { pattern: host } of config.routes) {
  test(host + " redirects paths and preserves queries", () => {
    for (const [path, expected] of [
      ["/", "/"], ["/index.html", "/"], ["/setup.html", "/setup"],
      ["/spec/index.html", "/spec/index"], ["/spec/103-errors.html?q=a%20b", "/spec/103-errors?q=a%20b"],
      ["/setup", "/setup"], ["/spec/", "/spec/"], ["/a.html/b", "/a.html/b"],
      ["/a%20b.html", "/a%20b"],
    ]) {
      const response = worker.fetch(new Request("https://" + host + path))
      assert.equal(response.status, 301)
      assert.equal(response.headers.get("location"), "https://vibelang.sh" + expected)
    }
  })
}
test("the retired primary domain is routed", () => {
  assert.ok(config.routes.some(route => route.pattern === "effeck.dev"))
})
