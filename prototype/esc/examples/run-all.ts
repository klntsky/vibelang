#!/usr/bin/env bun
// Compile and run every example.
//
//   bun prototype/esc/examples/run-all.ts [filter] [--verbose]
//
// Runnable files export main() and state `// expect-result: <json>`; the
// script runs main() with a timeout and compares. Files named *.fail.es state
// `// expect: CODE ...` on their first line; the script checks the compiler
// reports each code. Contracts are printed with the proposal's claims.
import fs from "node:fs"
import path from "node:path"
import { Cause, Effect, Exit } from "effect"
import { formatContract, formatDiagnostic } from "../src/cli.ts"
import { compile } from "../src/compiler.ts"

const dir = import.meta.dir
const args = process.argv.slice(2)
const verbose = args.includes("--verbose")
const filter = args.find((a) => !a.startsWith("--"))
const files = fs
  .readdirSync(dir)
  .filter((f) => f.endsWith(".es") && (!filter || f.includes(filter)))
  .sort()

interface Row {
  file: string
  kind: "run" | "fail"
  status: "PASS" | "FAIL"
  detail: string
  claims: { ok: number; bad: number }
}
const rows: Row[] = []
const mismatches: string[] = []

for (const name of files) {
  const file = path.join(dir, name)
  const text = fs.readFileSync(file, "utf8")
  const isFail = name.endsWith(".fail.es")
  let result
  try {
    result = compile([file], { outDir: path.join(dir, "out") })
  } catch (e) {
    rows.push({ file: name, kind: isFail ? "fail" : "run", status: "FAIL", detail: `compiler crashed: ${(e as Error).stack?.split("\n").slice(0, 3).join(" | ")}`, claims: { ok: 0, bad: 0 } })
    continue
  }
  const errors = result.diagnostics.filter((d) => d.severity === "error")
  const claims = { ok: 0, bad: 0 }
  for (const c of result.contracts) {
    if (!c.claim || c.claim.status === "text") continue
    if (c.claim.status === "match") claims.ok++
    else {
      claims.bad++
      mismatches.push(`${name}: ${formatContract(c).trim()}`)
    }
  }
  console.log(`\n=== ${name} (${result.passes} passes, ${result.ms.toFixed(0)} ms)`)
  if (verbose || !isFail) for (const c of result.contracts) console.log(formatContract(c))
  if (isFail) {
    const expected = (text.split("\n")[0].match(/expect:\s*(.*)$/)?.[1] ?? "").split(/\s+/).filter(Boolean)
    const got = new Set(result.diagnostics.map((d) => d.code))
    const missing = expected.filter((c) => !got.has(c))
    for (const d of result.diagnostics) console.log(formatDiagnostic(d))
    rows.push({
      file: name,
      kind: "fail",
      status: missing.length === 0 ? "PASS" : "FAIL",
      detail: missing.length === 0 ? `reported ${expected.join(" ")}` : `missing ${missing.join(" ")}; got ${[...got].join(" ") || "nothing"}`,
      claims
    })
    continue
  }
  for (const d of result.diagnostics) console.log(formatDiagnostic(d))
  const expectLine = text.split("\n").find((l) => l.startsWith("// expect-result:"))
  const expected = expectLine ? expectLine.replace("// expect-result:", "").trim() : undefined
  let status: Row["status"] = "FAIL"
  let detail = ""
  try {
    const mod = await import(`${result.files[0].out}?t=${Date.now()}`)
    const exit = (await Promise.race([
      Effect.runPromiseExit(mod.main()),
      new Promise((resolve) => setTimeout(() => resolve("timeout"), 10_000))
    ])) as Exit.Exit<unknown, unknown> | "timeout"
    if (exit === "timeout") detail = "timed out after 10 s"
    else if (Exit.isSuccess(exit)) {
      const got = JSON.stringify(exit.value)
      const want = expected ? JSON.stringify(JSON.parse(expected)) : undefined
      status = want === undefined || got === want ? "PASS" : "FAIL"
      detail = want === undefined || got === want ? got : `got ${got}\n      want ${want}`
    } else detail = `failed: ${Cause.pretty(exit.cause).split("\n").slice(0, 3).join(" | ")}`
  } catch (e) {
    detail = `threw: ${(e as Error).message.split("\n")[0]}`
  }
  if (errors.length > 0) detail = `${errors.length} compile error(s); ${detail}`
  console.log(`  -> ${status}: ${detail}`)
  rows.push({ file: name, kind: "run", status, detail, claims })
}

console.log("\n=== Summary")
for (const r of rows) {
  const c = r.claims.ok + r.claims.bad > 0 ? `  claims ${r.claims.ok}/${r.claims.ok + r.claims.bad}` : ""
  console.log(`${r.status}  ${r.kind.padEnd(4)}  ${r.file.padEnd(40)} ${r.detail.split("\n")[0].slice(0, 110)}${c}`)
}
const pass = rows.filter((r) => r.status === "PASS").length
console.log(`\n${pass}/${rows.length} pass`)
if (mismatches.length > 0) {
  console.log(`\n=== Claimed-contract mismatches (${mismatches.length})`)
  for (const m of mismatches) console.log(m)
}
