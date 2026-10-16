#!/usr/bin/env bun
// esc: the EffectScript prototype compiler.
//
//   bun prototype/esc/src/cli.ts build <file.es>... [--out dir] [--json]
//   bun prototype/esc/src/cli.ts run <file.es> [--out dir]     build, then run the module's main()
import path from "node:path"
import { compile, type CompileResult, type ContractReport } from "./compiler.ts"
import type { Diagnostic } from "./model.ts"

const lineCol = (text: string, pos: number) => {
  const before = text.slice(0, pos)
  const line = before.split("\n").length
  const col = pos - before.lastIndexOf("\n")
  return { line, col }
}

const sources = new Map<string, string>()
const readSource = (file: string) => {
  let s = sources.get(file)
  if (s === undefined) {
    s = require("node:fs").readFileSync(file, "utf8") as string
    sources.set(file, s)
  }
  return s
}

export const formatDiagnostic = (d: Diagnostic) => {
  const text = readSource(d.file)
  const { line, col } = lineCol(text, d.start)
  const src = text.split("\n")[line - 1] ?? ""
  const width = Math.max(1, Math.min(d.end - d.start, src.length - col + 1))
  const where = `${path.relative(process.cwd(), d.file)}:${line}:${col}`
  const lines = [`${d.severity} ${d.code}: ${d.message}`, `  ${where}   ${src.trim()}`]
  const indent = `  ${where}   `.length + (col - 1) - (src.length - src.trimStart().length)
  if (indent > 0 && width > 0) lines.push(`${" ".repeat(Math.max(0, indent))}${"^".repeat(Math.min(width, 60))}`)
  for (const n of d.notes ?? []) lines.push(`  ${n}`)
  return lines.join("\n")
}

export const formatContract = (c: ContractReport) => {
  const tag = c.direct ? " [plain JS]" : c.contract ? " [written]" : ""
  const lines = [`  ${c.name}${tag}: ${c.inferred}`]
  if (c.claim) {
    const status = c.claim.status === "match" ? "matches" : c.claim.status
    lines.push(`    claim (line ${c.claim.line}): ${c.claim.text}  -> ${status}${c.claim.note ? ` (${c.claim.note})` : ""}`)
    for (const ch of c.claim.channels) {
      if (!ch.ok) lines.push(`      ${ch.channel}: inferred ${ch.inferred}  vs claimed ${ch.claimed}`)
    }
  }
  return lines.join("\n")
}

export const report = (r: CompileResult) => {
  const out: string[] = []
  for (const d of r.diagnostics) out.push(formatDiagnostic(d))
  const byFile = new Map<string, ContractReport[]>()
  for (const c of r.contracts) byFile.set(c.file, [...(byFile.get(c.file) ?? []), c])
  for (const [file, cs] of byFile) {
    out.push(`contracts ${path.relative(process.cwd(), file)}`)
    for (const c of cs) out.push(formatContract(c))
  }
  out.push(`${r.files.length} file(s), ${r.passes} pass(es)${r.converged ? "" : " (not converged)"}, ${r.ms.toFixed(0)} ms`)
  return out.join("\n")
}

if (import.meta.main) {
  const args = process.argv.slice(2)
  const cmd = args[0]
  const outIdx = args.indexOf("--out")
  const outDir = outIdx >= 0 ? args[outIdx + 1] : undefined
  const files = args.slice(1).filter((a, i, all) => !a.startsWith("--") && all[i] !== outDir)
  if ((cmd !== "build" && cmd !== "run") || files.length === 0) {
    console.error("usage: esc build <file.es>... [--out dir] [--json] | esc run <file.es> [--out dir]")
    process.exit(2)
  }
  const result = compile(files, { outDir })
  if (args.includes("--json")) console.log(JSON.stringify({ ...result, files: result.files.map((f) => f.out) }, null, 2))
  else console.log(report(result))
  const errors = result.diagnostics.filter((d) => d.severity === "error")
  if (cmd === "run") {
    if (errors.length > 0) console.log(`(running despite ${errors.length} error(s))`)
    const mod = await import(result.files[0].out)
    if (typeof mod.main !== "function") {
      console.error("no exported main()")
      process.exit(1)
    }
    const { Effect, Exit, Cause } = await import("effect")
    const exit = await Effect.runPromiseExit(mod.main())
    if (Exit.isSuccess(exit)) console.log("result:", JSON.stringify(exit.value))
    else console.log("failure:", Cause.pretty(exit.cause))
    process.exit(Exit.isSuccess(exit) ? 0 : 1)
  }
  process.exit(errors.length > 0 ? 1 : 0)
}
