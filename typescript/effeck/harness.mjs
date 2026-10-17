// Compiles and runs the Effeck acceptance cases in effeck/cases.
//
//   node effeck/harness.mjs [--status] [--verbose] [--jobs N] [case...]
//
// smoke.sh builds tsc and the runtime, then runs this script; run smoke.sh,
// not this file. A case argument selects the case with that name, or else
// every case whose name contains it. Each case is a directory with a
// tsconfig.json, and its kind comes from its sources:
//
//   - A failing case has a source whose first line is `// expect: EK2001 ...`.
//     It passes when the error codes tsc reports are exactly those codes.
//   - A runnable case has a source with a `// expect-result: <json>` line.
//     That source must export main(), a function that returns an Effect. The
//     harness writes out/run.mjs, which runs main() and prints the JSON of its
//     success value on a line that starts with @@result, and the case passes
//     when that line holds <json>. What the program logs does not count.
//   - Any other case runs <case>/run.mjs, or else out/main.js, and compares
//     what it prints with expected/stdout.txt.
//
// Every case except a failing one must compile without errors. Every file in
// expected/ other than stdout.txt and diagnostics.txt must equal the file at
// the same path under out/. A runnable case with expected/stdout.txt compares
// with that file. A case of any kind with expected/diagnostics.txt passes only
// when tsc reports exactly the diagnostics it lists, one per line, in tsc's
// order and without message text: `main.effeck(5,23): error EK1001`. A case with
// expected/messages.txt passes only when each of its lines appears in what
// tsc printed, which checks a diagnostic's text.
//
// A case is pending when it has a file named `pending` whose first word names
// the lane that owns the missing feature and whose rest says what is missing.
// A pending case runs but never fails the suite; when it passes, the harness
// reports XPASS, and its owner deletes the file in the commit that fixed it.
//
// Each case gets node_modules/effect from $EFFECT_NODE_MODULES and
// node_modules/@effeck/runtime from $EFFECK_RUNTIME.
import { spawn } from "node:child_process"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import { fileURLToPath } from "node:url"

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const casesDir = path.join(root, "effeck", "cases")
const tsc = path.join(root, "built", "local", "tsc")
const effectModules = process.env.EFFECT_NODE_MODULES
const runtimeDir = process.env.EFFECK_RUNTIME
if (!effectModules || !runtimeDir) {
  console.error("harness.mjs: set EFFECT_NODE_MODULES and EFFECK_RUNTIME, or run effeck/smoke.sh")
  process.exit(2)
}

const args = process.argv.slice(2)
const statusOnly = args.includes("--status")
const verbose = args.includes("--verbose") || args.includes("-v")
const jobsAt = args.indexOf("--jobs")
const jobs = jobsAt >= 0 ? Number(args[jobsAt + 1]) : Math.max(2, Math.min(8, os.availableParallelism() >> 1))
const selectors = args.filter((a, i) => !a.startsWith("-") && !(jobsAt >= 0 && i === jobsAt + 1))

const allCases = fs
  .readdirSync(casesDir)
  .filter((name) => fs.existsSync(path.join(casesDir, name, "tsconfig.json")))
  .sort()
const selected = selectors.length === 0 ? allCases : [...new Set(selectors.flatMap(select))]

function select(selector) {
  if (allCases.includes(selector)) return [selector]
  const matches = allCases.filter((name) => name.includes(selector))
  if (matches.length === 0) {
    console.error(`harness.mjs: no case matches ${selector}`)
    process.exit(2)
  }
  return matches
}

// run spawns a command and resolves with its exit code and output.
function run(command, argv, options) {
  return new Promise((resolve) => {
    const child = spawn(command, argv, { ...options, stdio: ["ignore", "pipe", "pipe"] })
    let stdout = ""
    let stderr = ""
    child.stdout.on("data", (d) => (stdout += d))
    child.stderr.on("data", (d) => (stderr += d))
    const timer = setTimeout(() => child.kill("SIGKILL"), options.timeout)
    child.on("close", (code, signal) => {
      clearTimeout(timer)
      resolve({ code: signal ? `killed by ${signal}` : code, stdout, stderr })
    })
  })
}

// Diagnostics as `tsc --pretty false` prints them: `file(line,col): error EK2001: message`,
// or without the location for diagnostics that have no file.
const diagnosticLine = /^(?:(.+)\((\d+),(\d+)\): )?(error|warning|message|suggestion) ([A-Za-z]+\d+): /

function parseDiagnostics(text) {
  const diagnostics = []
  for (const line of text.split("\n")) {
    const m = diagnosticLine.exec(line)
    if (m) diagnostics.push({ category: m[4], code: m[5], text: line })
    else if (diagnostics.length > 0 && line.startsWith(" ")) diagnostics.at(-1).text += `\n${line}`
  }
  return diagnostics
}

function link(target, at) {
  try {
    if (fs.readlinkSync(at) === target) return
    fs.unlinkSync(at)
  } catch {}
  fs.symlinkSync(target, at)
}

function linkNodeModules(dir) {
  const modules = path.join(dir, "node_modules")
  const stat = fs.lstatSync(modules, { throwIfNoEntry: false })
  if (stat && !stat.isDirectory()) fs.unlinkSync(modules) // an older smoke.sh linked the whole directory
  fs.mkdirSync(path.join(modules, "@effeck"), { recursive: true })
  link(path.join(effectModules, "effect"), path.join(modules, "effect"))
  link(runtimeDir, path.join(modules, "@effeck", "runtime"))
}

// readHeaders finds the expected codes and the expected result in a case's sources.
function readHeaders(dir) {
  const headers = { codes: undefined, result: undefined, entry: undefined }
  for (const name of fs.readdirSync(dir).sort()) {
    if (!/\.(es|ts|mts|cts)$/.test(name) || name.endsWith(".d.ts")) continue
    const lines = fs.readFileSync(path.join(dir, name), "utf8").split("\n")
    const codes = /^\/\/ expect:\s*(.*)$/.exec(lines[0])
    if (codes) headers.codes = [...(headers.codes ?? []), ...codes[1].split(/\s+/).filter(Boolean)]
    const result = lines.find((line) => line.startsWith("// expect-result:"))
    if (result) {
      headers.result = JSON.stringify(JSON.parse(result.slice("// expect-result:".length)))
      headers.entry = name.replace(/\.(es|ts|mts|cts)$/, ".js")
    }
  }
  return headers
}

// resultMarker starts the line on which out/run.mjs prints main()'s result.
const resultMarker = "@@result "

function runner(entry) {
  return `// Generated by effeck/harness.mjs: runs main() and prints its result.
import { Cause, Effect, Exit } from "effect"
import { main } from "./${entry}"

const timeout = new Promise((resolve) => setTimeout(resolve, 10_000, "timeout"))
const exit = await Promise.race([Effect.runPromiseExit(main()), timeout])
if (exit === "timeout") console.log("${resultMarker}timed out after 10 s")
else if (Exit.isSuccess(exit)) console.log("${resultMarker}" + JSON.stringify(exit.value))
else {
  console.log("${resultMarker}failed: " + Cause.pretty(exit.cause).split("\\n")[0])
  console.error(Cause.pretty(exit.cause))
}
process.exit(0)
`
}

function readPending(dir) {
  const file = path.join(dir, "pending")
  if (!fs.existsSync(file)) return undefined
  const text = fs.readFileSync(file, "utf8").trim()
  const space = text.indexOf(" ")
  return space < 0 ? { owner: text, reason: "" } : { owner: text.slice(0, space), reason: text.slice(space + 1) }
}

const sameSet = (a, b) => a.size === b.size && [...a].every((x) => b.has(x))

// check compiles and runs one case. It resolves with { ok, detail, log }.
async function check(name) {
  const dir = path.join(casesDir, name)
  const headers = readHeaders(dir)
  fs.rmSync(path.join(dir, "out"), { recursive: true, force: true })
  linkNodeModules(dir)

  const compiled = await run(tsc, ["-p", ".", "--pretty", "false"], { cwd: dir, timeout: 120_000 })
  const output = compiled.stdout + compiled.stderr
  const diagnostics = parseDiagnostics(output)
  const errors = diagnostics.filter((d) => d.category === "error")
  if (compiled.code !== 0 && compiled.code !== 1 && compiled.code !== 2) {
    return { ok: false, detail: `tsc exited with ${compiled.code}`, log: output }
  }

  const diagnosticsFile = path.join(dir, "expected", "diagnostics.txt")
  if (fs.existsSync(diagnosticsFile)) {
    const want = fs.readFileSync(diagnosticsFile, "utf8").trim().split("\n").map((line) => line.trim()).filter(Boolean)
    const got = diagnostics.map((d) => d.text.split("\n")[0].replace(/^(.*?: (?:error|warning|message|suggestion) [A-Za-z]+\d+):.*$/, "$1"))
    if (want.join("\n") !== got.join("\n")) {
      return { ok: false, detail: "diagnostics differ from expected/diagnostics.txt", log: [`want:\n  ${want.join("\n  ")}`, `got:\n  ${got.join("\n  ")}`, output].join("\n") }
    }
  }

  const messagesFile = path.join(dir, "expected", "messages.txt")
  if (fs.existsSync(messagesFile)) {
    const lines = fs.readFileSync(messagesFile, "utf8").split("\n").map((line) => line.trim()).filter(Boolean)
    const missing = lines.filter((line) => !output.includes(line))
    if (missing.length > 0) {
      return { ok: false, detail: "tsc did not print every line of expected/messages.txt", log: [`missing:\n  ${missing.join("\n  ")}`, output].join("\n") }
    }
  }

  if (headers.codes) {
    const want = new Set(headers.codes)
    const got = new Set(errors.map((d) => d.code))
    if (sameSet(want, got)) return { ok: true, detail: `reported ${[...want].join(" ")}`, log: output }
    const missing = [...want].filter((c) => !got.has(c))
    const extra = [...got].filter((c) => !want.has(c))
    const parts = [missing.length > 0 && `missing ${missing.join(" ")}`, extra.length > 0 && `unexpected ${extra.join(" ")}`]
    return { ok: false, detail: parts.filter(Boolean).join("; "), log: output }
  }

  if (errors.length > 0) {
    return { ok: false, detail: `tsc reported ${errors.length} error(s): ${[...new Set(errors.map((d) => d.code))].join(" ")}`, log: output }
  }

  const expectedDir = path.join(dir, "expected")
  const expectedFiles = fs.existsSync(expectedDir) ? fs.readdirSync(expectedDir, { recursive: true }) : []
  for (const file of expectedFiles.map(String).sort()) {
    const want = path.join(expectedDir, file)
    if (file === "stdout.txt" || file === "diagnostics.txt" || file === "messages.txt" || fs.statSync(want).isDirectory()) continue
    const got = path.join(dir, "out", file)
    if (!fs.existsSync(got)) return { ok: false, detail: `out/${file} is missing`, log: output }
    if (fs.readFileSync(want, "utf8") !== fs.readFileSync(got, "utf8")) {
      const diff = await run("diff", ["-u", want, got], { timeout: 10_000 })
      return { ok: false, detail: `out/${file} differs from expected/${file}`, log: diff.stdout }
    }
  }

  let entry = path.join(dir, "out", "main.js")
  if (fs.existsSync(path.join(dir, "run.mjs"))) entry = path.join(dir, "run.mjs")
  else if (headers.entry) {
    entry = path.join(dir, "out", "run.mjs")
    fs.writeFileSync(entry, runner(headers.entry))
  }
  const stdoutFile = path.join(expectedDir, "stdout.txt")
  const ran = await run("node", [entry], { cwd: dir, timeout: 60_000 })
  let want, got
  if (fs.existsSync(stdoutFile)) {
    want = fs.readFileSync(stdoutFile, "utf8")
    got = ran.stdout
  } else if (headers.result !== undefined) {
    want = headers.result
    got = ran.stdout.split("\n").findLast((line) => line.startsWith(resultMarker))?.slice(resultMarker.length)
  } else {
    return { ok: false, detail: "nothing to compare: no expected/stdout.txt and no expect-result line", log: "" }
  }
  if (got !== want) {
    const detail = got?.trim().split("\n")[0] || `printed no result (exit ${ran.code})`
    const log = [`want: ${want.trimEnd()}`, `stdout:\n${ran.stdout.trimEnd()}`, ran.stderr.trimEnd()].filter(Boolean).join("\n")
    return { ok: false, detail: `${path.relative(dir, entry)} printed ${detail.slice(0, 160)}`, log }
  }
  return { ok: true, detail: "", log: output }
}

async function main() {
  const results = new Array(selected.length)
  let next = 0
  let printed = 0
  const flush = () => {
    while (printed < selected.length && results[printed]) report(results[printed++])
  }
  const worker = async () => {
    while (next < selected.length) {
      const index = next++
      const name = selected[index]
      const pending = readPending(path.join(casesDir, name))
      const started = Date.now()
      let result
      try {
        result = await check(name)
      } catch (e) {
        result = { ok: false, detail: `harness error: ${e.message}`, log: e.stack }
      }
      const status = pending ? (result.ok ? "XPASS" : "pending") : result.ok ? "ok" : "FAIL"
      results[index] = { name, pending, status, ms: Date.now() - started, ...result }
      flush()
    }
  }
  await Promise.all(Array.from({ length: jobs }, worker))

  const count = (status) => results.filter((r) => r.status === status).length
  console.log(
    `\n${results.length} cases: ${count("ok")} ok, ${count("FAIL")} FAIL, ${count("pending")} pending, ${count("XPASS")} XPASS`,
  )
  const xpass = results.filter((r) => r.status === "XPASS")
  if (xpass.length > 0) console.log(`XPASS: delete the pending file of ${xpass.map((r) => r.name).join(", ")}`)
  process.exit(count("FAIL") > 0 ? 1 : 0)
}

function report(r) {
  const owner = r.pending?.owner ?? "-"
  if (statusOnly) {
    console.log(`${r.name.padEnd(36)} ${owner.padEnd(5)} ${r.status}`)
    return
  }
  const time = verbose ? ` (${(r.ms / 1000).toFixed(1)} s)` : ""
  switch (r.status) {
    case "ok":
      console.log(`ok      ${r.name}${time}`)
      break
    case "XPASS":
      console.log(`XPASS   ${r.name} (pending on ${owner}; delete its pending file)${time}`)
      break
    case "pending":
      console.log(`pending ${r.name} (${owner}: ${r.pending.reason}): ${r.detail}${time}`)
      break
    default:
      console.log(`FAIL    ${r.name}: ${r.detail}${time}`)
  }
  if (r.status === "FAIL" || (verbose && r.log)) {
    for (const line of r.log.trimEnd().split("\n").slice(0, verbose ? 200 : 40)) console.log(`        ${line}`)
  }
}

await main()
