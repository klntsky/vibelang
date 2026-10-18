// Portable worker contracts; real isolation and compilation stay in the Linux suite.
import assert from "node:assert/strict"
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { compile, identity, maxSourceBytes, parseDiagnostics, settings } from "../compile.mjs"

function fixture(t) {
  const project = mkdtempSync(join(tmpdir(), "vibelang-worker-unit-"))
  t.after(() => rmSync(project, { recursive: true, force: true }))
  const binary = join(project, "compiler")
  writeFileSync(binary, '#!/bin/sh\nprintf "VibeLang 0.0.1 (TypeScript 7)\\n"\n', { mode: 0o755 })
  for (const [name, version] of [["vibelang", "0.0.1"], ["vibelang-runtime", "0.0.1"], ["effect", "4.0.5"]]) {
    mkdirSync(join(project, "node_modules", name), { recursive: true })
    writeFileSync(join(project, "node_modules", name, "package.json"), JSON.stringify({ name, version }))
  }
  return { project, binary, timeoutMs: 1000, network: "policy" }
}

test("settings select the platform compiler and deployment overrides", () => {
  const defaults = settings({})
  assert.equal(defaults.timeoutMs, 10_000)
  assert.equal(defaults.network, "unshare")
  assert.equal(settings({ VIBELANG_WORKER_TIMEOUT_MS: "2147483647" }).timeoutMs, 2147483647)
  assert.ok(defaults.binary.includes(`vibelang-${process.platform}-${process.arch}`))
  assert.deepEqual(settings({ VIBELANG_WORKER_PROJECT: "/fixed", VIBELANG_WORKER_BINARY: "/compiler", VIBELANG_WORKER_TIMEOUT_MS: "250", VIBELANG_WORKER_NETWORK: "policy" }), {
    project: "/fixed", binary: "/compiler", timeoutMs: 250, network: "policy"
  })
  assert.throws(() => settings({ VIBELANG_WORKER_NETWORK: "typo" }), /must be unshare or policy/)
})

test("refuses invalid source and oversized UTF-8 before reading project files", async () => {
  for (const input of [undefined, null, 12, {}, Buffer.from("text")]) assert.throws(() => compile(input, {}), /source must be a string/)
  assert.deepEqual(await compile("text\0text", {}), { refusal: "invalid_source" })
  assert.deepEqual(await compile("é".repeat(maxSourceBytes / 2 - 1) + "x\0", {}), { refusal: "invalid_source" })
  for (const source of ["x".repeat(maxSourceBytes + 1), "😀".repeat(maxSourceBytes / 4 + 1), "é".repeat(maxSourceBytes / 2 + 1)]) {
    assert.deepEqual(await compile(source, {}), { refusal: "too_large" })
  }
})

test("diagnostics preserve coordinates and severity, ignoring other files and unlocated messages", () => {
  const output = ["banner", "../main.vibe.d.ts(1,2): error TS1: declaration", "domain.vibe(1,2): error TS1: wrong file",
    "main.vibe(2,7): warning EK2004: unreachable catch", "  details from compiler",
    "/tmp/main.vibe(12,42): error TS2322: Type 'string' is not assignable.", "error TS5058: no config"].join("\n")
  assert.deepEqual(parseDiagnostics(output), [
    { line: 2, column: 7, severity: "warning", code: "EK2004", message: "unreachable catch" },
    { line: 12, column: 42, severity: "error", code: "TS2322", message: "Type 'string' is not assignable." }
  ])
  assert.deepEqual(parseDiagnostics(""), [])
})

test("wide diagnostic streams keep the first 50 records with at most 500 message characters", () => {
  for (const count of [1, 49, 50, 51, 1000]) {
    const records = Array.from({ length: count }, (_, i) => ({ line: i + 1, column: i + 2,
      severity: i % 2 ? "warning" : "error", code: i % 2 ? "EK2004" : "TS2322", message: String(i).padEnd(700, "x") }))
    const output = records.map((d) => `ignored\n/tmp/main.vibe(${d.line},${d.column}): ${d.severity} ${d.code}: ${d.message}`).join("\n")
    assert.deepEqual(parseDiagnostics(output), records.slice(0, 50).map((d) => ({ ...d, message: d.message.slice(0, 500) })))
  }
})

test("identity is deterministic and changes with binaries, dependencies and lockfiles", (t) => {
  const config = fixture(t)
  const first = identity(config)
  assert.deepEqual(first.packages, ["vibelang@0.0.1", "vibelang-runtime@0.0.1", "effect@4.0.5"])
  assert.equal(first.vibelangVersion, "0.0.1")
  assert.match(first.compilerDigest, /^sha256:[a-f0-9]{64}$/)
  assert.match(first.dependenciesDigest, /^sha256:[a-f0-9]{64}$/)
  assert.deepEqual(identity(config), first)
  writeFileSync(config.binary, readFileSync(config.binary, "utf8") + "# changed binary\n")
  const changed = identity(config)
  assert.notEqual(changed.compilerDigest, first.compilerDigest)
  assert.equal(changed.dependenciesDigest, first.dependenciesDigest)
  writeFileSync(join(config.project, "package-lock.json"), "lock one")
  const locked = identity(config)
  assert.notEqual(locked.dependenciesDigest, first.dependenciesDigest)
  writeFileSync(join(config.project, "package-lock.json"), "lock two")
  assert.notEqual(identity(config).dependenciesDigest, locked.dependenciesDigest)
  rmSync(join(config.project, "package-lock.json"))
  writeFileSync(join(config.project, "node_modules/effect/package.json"), '{"version":"4.0.6"}')
  assert.notEqual(identity(config).dependenciesDigest, first.dependenciesDigest)
})

test("startup faults identify missing compiler, invalid version and missing dependency", (t) => {
  const config = fixture(t)
  assert.throws(() => identity({ ...config, binary: join(config.project, "missing") }), /no compiler/)
  writeFileSync(config.binary, '#!/bin/sh\nprintf "not the compiler\\n"\n')
  assert.throws(() => identity(config), /cannot read the compiler version/)
  writeFileSync(config.binary, '#!/bin/sh\nprintf "VibeLang 0.0.1\\n"\n')
  rmSync(join(config.project, "node_modules/vibelang-runtime"), { recursive: true })
  assert.throws(() => identity(config), /missing vibelang-runtime; install it first/)
})

test("rejects invalid deadlines instead of silently changing the wall-clock bound", () => {
  for (const value of ["", "NaN", "Infinity", "-1", "0", "1e100", "not-a-number"]) {
    assert.throws(() => settings({ VIBELANG_WORKER_TIMEOUT_MS: value }), /VIBELANG_WORKER_TIMEOUT_MS/)
  }
})
