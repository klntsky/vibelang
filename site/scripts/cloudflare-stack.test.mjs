import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { fileURLToPath, pathToFileURL } from "node:url"
import vm from "node:vm"
import { test } from "node:test"
import ts from "typescript"

test("the stack resolves the public domain in its Cloudflare account and disables browser checks", () => {
  const stackPath = process.env.VIBELANG_HYGIENE_ROOT
    ? process.env.VIBELANG_HYGIENE_ROOT + "/site/alchemy.run.ts"
    : fileURLToPath(new URL("../alchemy.run.ts", import.meta.url))
  const source = readFileSync(stackPath, "utf8").replaceAll("import.meta.url", JSON.stringify(pathToFileURL(stackPath).href))
  const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const calls = []
  function value(v) {
    const iterator = (function* () { return yield v })()
    iterator.pipe = (...ops) => ops.reduce((effect, op) => op(effect), iterator)
    return iterator
  }
  const lookup = {
    resolveZoneId(props) {
      calls.push(["lookup", props])
      return value("resolved-zone")
    },
    findZoneByName(props) {
      calls.push(["lookup", props])
      return value({ id: "resolved-zone" })
    },
  }
  const environment = { CloudflareEnvironment: value(value({ accountId: "configured-account" })) }
  const cloudflare = {
    ...environment,
    providers: () => ({}), state: () => ({}),
    Zone: { ...lookup, Setting: (name, props) => { calls.push(["setting", name, props]); return value({}) } },
    Website: { StaticSite: (name, props) => { calls.push(["site", name, props]); return value({ url: "https://vibelang.sh" }) } },
  }
  const alchemy = {
    Stack(_name, _options, program) {
      let step = program.next()
      while (!step.done) step = program.next(step.value)
      return step.value
    },
  }
  const module = { exports: {} }
  vm.runInNewContext(code, {
    exports: module.exports, module, URL,
    require(name) {
      if (name === "alchemy") return alchemy
      if (name === "@alchemy.run/cloudflare") return cloudflare
      if (name === "effect/Effect") return { gen: fn => fn(), orDie: effect => effect }
      if (name === "alchemy/Cloudflare/Zone/lookup") return lookup
      if (name === "alchemy/Cloudflare/CloudflareEnvironment") return environment
      if (name === "node:url") return { fileURLToPath }
      throw new Error("unexpected import: " + name)
    },
  }, { filename: stackPath })
  const lookups = calls.filter(call => call[0] === "lookup")
  assert.equal(lookups.length, 1)
  assert.equal(lookups[0][1].accountId, "configured-account")
  assert.equal(lookups[0][1].hostname ?? lookups[0][1].name, "vibelang.sh")
  const settings = calls.filter(call => call[0] === "setting")
  assert.equal(settings.length, 1)
  assert.equal(settings[0][1], "BrowserCheck")
  assert.equal(settings[0][2].zoneId, "resolved-zone")
  assert.equal(settings[0][2].settingId, "browser_check")
  assert.equal(settings[0][2].value, "off")
  assert.equal(calls.filter(call => call[0] === "site").length, 1)
})
