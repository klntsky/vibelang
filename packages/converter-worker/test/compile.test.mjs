// Runs on Linux x64 after test/setup-project.sh. Proves the worker's contract: it compiles, reports
// bounded diagnostics, refuses oversize input, times out, has no network, and never runs the program.
import { strict as assert } from "node:assert"
import { existsSync, readFileSync, rmSync } from "node:fs"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"
import { compile, identity, maxSourceBytes, parseDiagnostics, proveNoEgress, settings } from "../compile.mjs"

const root = fileURLToPath(new URL("../../..", import.meta.url))
const config = settings()
const hero = readFileSync(join(root, ".smithers/flows/convert/golden/01-hero-batch-lookup/expected.vibe"), "utf8")

test("compiles the hero with no diagnostics", async () => {
  const result = await compile(hero, config)
  assert.equal(result.timedOut, false)
  assert.equal(result.exitCode, 0, JSON.stringify(result))
  assert.deepEqual(result.diagnostics.filter((d) => d.severity === "error"), [])
})

test("reports a type error with its position and code", async () => {
  const result = await compile('export function f(): Effect<number> {\n  return "no"\n}\n', config)
  assert.notEqual(result.exitCode, 0)
  assert.ok(result.diagnostics[0].line >= 1)
  assert.match(result.diagnostics[0].code, /^(TS|EK)\d+$/)
})

test("refuses a source over 8 KiB without compiling", async () => {
  assert.deepEqual(await compile("x".repeat(maxSourceBytes + 1), config), { refusal: "too_large" })
})

test("kills a compile that passes its wall-clock limit", async () => {
  const result = await compile(hero, { ...config, timeoutMs: 1 })
  assert.equal(result.timedOut, true)
  assert.notEqual(result.exitCode, 0)
})

test("bounds the diagnostics it returns", async () => {
  const many = Array.from({ length: 200 }, (_, i) => `export const v${i}: number = "s"`).join("\n")
  const result = await compile(many, config)
  assert.ok(result.diagnostics.length <= 50)
  assert.ok(result.diagnostics.length > 0)
})

test("never runs the program", async () => {
  const marker = `/tmp/vibelang-worker-ran-${process.pid}`
  rmSync(marker, { force: true })
  await compile(`export function main() {\n  sync(() => require("fs").writeFileSync(${JSON.stringify(marker)}, "ran"))\n}\n`, config)
  assert.equal(existsSync(marker), false)
})

test("the compile environment has no network", { skip: config.network !== "unshare" && "policy mode: the deployment denies egress" }, () => {
  assert.equal(proveNoEgress(config), true)
})

test("names the compiler and dependencies by digest", () => {
  const id = identity(config)
  assert.equal(id.vibelangVersion, "0.0.1")
  assert.match(id.compilerDigest, /^sha256:[0-9a-f]{64}$/)
  assert.match(id.dependenciesDigest, /^sha256:[0-9a-f]{64}$/)
})

test("parses only the converted file's diagnostics", () => {
  const out = "x/main.vibe(3,5): error TS2322: Type 'string' is not assignable.\n../lib.d.ts(1,1): error TS1: other\n"
  assert.deepEqual(parseDiagnostics(out), [{ line: 3, column: 5, severity: "error", code: "TS2322", message: "Type 'string' is not assignable." }])
})
