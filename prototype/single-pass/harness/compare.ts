// Compare the single-pass decisions with the prototype's final decisions over the corpus.
//
//   bun prototype/single-pass/harness/compare.ts [filter] [--verbose]
//
// Per file:
//   1. P: the prototype's loop (replica.ts), run to convergence; its final facts and output.
//   2. S: one neutral emission, one check with the patched checker (single-pass.ts).
//   3. Every decision of S against P: call lowering, lambda mode, conversion, loop, D10, D8, wire.
//   4. The prototype's own emitter driven by S, compared byte for byte with P's final output.
//   5. Fixpoint: the prototype's own pass (emit with S, check, collectFacts) applied to S once.
//      If it returns S, S is a fixed point of the prototype's iteration.
// Writes results.json next to this file's directory.
import fs from "node:fs"
import path from "node:path"
import ts from "typescript"
import { compilerOptions } from "../../esc/src/compiler.ts"
import { FileEmitter } from "../../esc/src/emit.ts"
import { collectFacts, fingerprint, type Unit } from "../../esc/src/facts.ts"
import type { EsFile, Facts } from "../../esc/src/model.ts"
import { type Decisions, decisionsOf, ESC, lineOf, makeHost, relImport, RUNTIME, rewriterFor, runReplica } from "./replica.ts"
import { runSinglePass } from "./single-pass.ts"

const args = process.argv.slice(2)
const dirArg = args.indexOf("--dir")
const dir = dirArg >= 0 ? path.resolve(args[dirArg + 1]) : path.join(ESC, "examples")
const verbose = args.includes("--verbose")
const filter = args.find((a, i) => !a.startsWith("--") && args[i - 1] !== "--dir")
const names = fs.readdirSync(dir).filter((f) => f.endsWith(".es") && (!filter || f.includes(filter))).sort()

const emitWith = (files: Map<string, EsFile>, facts: Facts): Unit[] => {
  const rewrite = rewriterFor(files)
  return [...files.values()].map((file) => ({
    file,
    genPath: file.outPath,
    emitted: new FileEmitter({ file, files, facts, runtimeImport: relImport(file.outPath, RUNTIME), rewriteImport: (s) => rewrite(file, s), check: true }).emitFile()
  }))
}

const src = (file: EsFile, node: ts.Node) => node.getText(file.sf).replace(/\s+/g, " ").slice(0, 70)

interface Disagreement {
  what: "call" | "lambda" | "conv" | "loop" | "direct" | "pipe" | "wire" | "rec"
  line?: number
  node: string
  prototype: string
  singlePass: string
}

const diffDecisions = (p: Decisions, s: Decisions): Disagreement[] => {
  const out: Disagreement[] = []
  for (const [k, v] of p.calls) {
    const u = s.calls.get(k)
    if (!u || u.emit !== v.emit) out.push({ what: "call", line: lineOf(v.file, v.node), node: src(v.file, v.node), prototype: v.emit, singlePass: u?.emit ?? "-" })
  }
  for (const [k, v] of p.lambdas) {
    const u = s.lambdas.get(k)
    if (!u || u.mode !== v.mode) out.push({ what: "lambda", line: lineOf(v.file, v.node), node: src(v.file, v.node), prototype: `${v.mode} (ctx ${v.ctx})`, singlePass: u ? `${u.mode} (ctx ${u.ctx})` : "-" })
  }
  for (const k of new Set([...p.convs.keys(), ...s.convs.keys()])) {
    const a = p.convs.get(k)
    const b = s.convs.get(k)
    if (a?.conv !== b?.conv) {
      const r = (a ?? b)!
      out.push({ what: "conv", line: lineOf(r.file, r.node), node: src(r.file, r.node), prototype: a?.conv || "none", singlePass: b?.conv || "none" })
    }
  }
  for (const k of new Set([...p.loops.keys(), ...s.loops.keys()])) {
    const a = p.loops.get(k)
    const b = s.loops.get(k)
    if (a?.kind !== b?.kind) {
      const r = (a ?? b)!
      out.push({ what: "loop", line: lineOf(r.file, r.node), node: src(r.file, r.node), prototype: a?.kind ?? "plain", singlePass: b?.kind ?? "plain" })
    }
  }
  for (const d of new Set([...p.direct, ...s.direct])) {
    if (p.direct.has(d) !== s.direct.has(d)) out.push({ what: "direct", node: d.split("#")[1], prototype: String(p.direct.has(d)), singlePass: String(s.direct.has(d)) })
  }
  for (const k of new Set([...p.pipes, ...s.pipes])) {
    if (p.pipes.has(k) !== s.pipes.has(k)) out.push({ what: "pipe", node: k, prototype: String(p.pipes.has(k)), singlePass: String(s.pipes.has(k)) })
  }
  for (const k of new Set([...p.wire.keys(), ...s.wire.keys()])) {
    if (p.wire.get(k) !== s.wire.get(k)) out.push({ what: "wire", node: k, prototype: p.wire.get(k) ?? "-", singlePass: s.wire.get(k) ?? "-" })
  }
  for (const k of new Set([...p.rec.keys(), ...s.rec.keys()])) {
    if (p.rec.get(k) !== s.rec.get(k)) out.push({ what: "rec", node: k, prototype: p.rec.get(k) ?? "-", singlePass: s.rec.get(k) ?? "-" })
  }
  return out
}

/** One pass of the prototype's loop, started from `facts`: emit, check, collect. */
const prototypePass = (files: Map<string, EsFile>, facts: Facts) => {
  const units = emitWith(files, facts)
  const virtual = new Map(units.map((u) => [u.genPath, u.emitted.text] as const))
  const program = ts.createProgram({ rootNames: [...virtual.keys()], options: compilerOptions, host: makeHost(ts, compilerOptions, virtual, new Map()) })
  const next = collectFacts(program, units)
  return { units, next, same: fingerprint(next, units) === fingerprint(facts, units) }
}

interface Row {
  file: string
  prototypePasses: number
  calls: number
  lambdas: number
  convs: number
  loops: number
  disagreements: Disagreement[]
  /** Disagreements once recursive functions' forward bindings are excluded (finding 7). */
  textSame: boolean
  fixpoint: boolean
  emissionFixpoint: boolean
  fixpointChanges: Disagreement[]
  /** Fingerprint lines (the prototype's convergence test) present only in S, or only in P. */
  rawOnlySinglePass: string[]
  rawOnlyPrototype: string[]
  anomalies: Array<{ kind: string; detail: string }>
  selections: { candidatesTried: number; viaNeeds: number; viaPipe: number; overloaded: number; split: number; splits: string[] }
  checkMs: number
  tsErrors: number
}

const rows: Row[] = []
for (const name of names) {
  const file = path.join(dir, name)
  const P = runReplica([file])
  const S = runSinglePass([file])
  const unitsS = emitWith(S.files, S.facts)
  const dS = decisionsOf(unitsS, S.facts)
  const dP = P.final
  const disagreements = diffDecisions(dP, dS)
  // The prototype's emitter driven by single-pass facts vs the prototype's final text.
  let textSame = true
  for (const u of unitsS) {
    const want = P.passes[P.passes.length - 1].text.get(u.file.path)
    if (u.emitted.text.slice(0, u.emitted.bodyEnd) !== want) textSame = false
  }
  // Fixpoint under the prototype's own rules.
  const fp = prototypePass(S.files, S.facts)
  const unitsNext = emitWith(S.files, fp.next)
  const fixpointChanges = fp.same ? [] : diffDecisions(dS, decisionsOf(unitsNext, fp.next))
  // The prototype stops when its facts repeat; a lowering fixpoint is weaker: one more pass of the
  // prototype from S would emit the same text.
  const emissionFixpoint = unitsS.every((u, i) => u.emitted.text.slice(0, u.emitted.bodyEnd) === unitsNext[i].emitted.text.slice(0, unitsNext[i].emitted.bodyEnd))
  // Raw facts that differ from the prototype's final facts (including ones no emission reads).
  const lines = (f: Facts, us: Unit[]) => new Set(fingerprint(f, us).split("\n"))
  const fS = lines(S.facts, unitsS)
  const fP = lines(P.facts, P.units)
  const readable = (l: string) => {
    const m = /^([a-z])(\/[^:]+\.es):(\d+)(.*)$/.exec(l)
    if (!m) return l
    const f = S.files.get(m[2])
    const line = f ? f.sf.getLineAndCharacterOfPosition(f.sf.getFullText().slice(+m[3]).search(/\S/) + +m[3]).line + 1 : 0
    return `${m[1]} ${path.basename(m[2])}:L${line}${m[4]}`
  }
  const rawOnlyS = [...fS].filter((l) => !fP.has(l)).map(readable)
  const rawOnlyP = [...fP].filter((l) => !fS.has(l)).map(readable)
  // Consistency: the mode the checker selected is the mode the prototype's lambdaKind derives from S.
  const anomalies = [...S.anomalies]
  for (const u of unitsS) {
    for (const [fn, mode] of u.emitted.lambdaModes) {
      const m = S.modes.get(fn)
      const expect = m?.mode === "program" && ts.isMethodDeclaration(fn) ? "program" : m?.mode
      if (m && expect !== mode) anomalies.push({ kind: "mode-mismatch", detail: `${src(u.file, fn)}: checker ${m.mode}, lambdaKind ${mode}` })
    }
  }
  let candidatesTried = 0, viaNeeds = 0, viaPipe = 0, overloaded = 0, split = 0
  const splits: string[] = []
  for (const [fn, m] of S.modes) {
    candidatesTried += m.candidates
    if (m.via === "needs") viaNeeds++
    if (m.via === "pipe") viaPipe++
    if (m.candidates > 1) overloaded++
    // Candidates that classified the same lambda differently: one fixed class could not serve them all.
    if (new Set(m.perCandidate).size > 1) {
      split++
      const f = [...S.files.values()].find((x) => x.sf === fn.getSourceFile())!
      splits.push(`L${lineOf(f, fn)} ${src(f, fn)}: ${m.perCandidate.join(", ")} -> ${m.mode}`)
    }
  }
  const row: Row = {
    file: name,
    prototypePasses: P.passes.length,
    calls: dP.calls.size,
    lambdas: dP.lambdas.size,
    convs: [...dP.convs.values()].length,
    loops: [...S.units.flatMap((u) => u.emitted.loops.map((l) => l.es))].filter((v, i, a) => a.indexOf(v) === i).length,
    disagreements,
    textSame,
    fixpoint: fp.same,
    emissionFixpoint,
    fixpointChanges,
    rawOnlySinglePass: rawOnlyS,
    rawOnlyPrototype: rawOnlyP,
    anomalies,
    selections: { candidatesTried, viaNeeds, viaPipe, overloaded, split, splits },
    checkMs: S.checkMs,
    tsErrors: S.tsErrors
  }
  rows.push(row)
  const tag = disagreements.length === 0 ? (textSame ? "SAME" : "same decisions, text differs") : `${disagreements.length} disagreement(s)`
  console.log(`${name.padEnd(40)} P=${row.prototypePasses}  ${tag}${fp.same ? "" : emissionFixpoint ? "  facts not a fixpoint (lowering is)" : "  NOT a fixpoint"}${anomalies.length ? `  ${anomalies.length} anomaly(ies)` : ""}`)
  if (verbose || disagreements.length > 0) for (const d of disagreements) console.log(`    ${d.what.padEnd(6)} ${d.line ? `L${d.line} ` : ""}${d.node}: prototype ${d.prototype}, single pass ${d.singlePass}`)
  if (verbose || !fp.same) for (const d of fixpointChanges) console.log(`    fixpoint ${d.what} ${d.line ? `L${d.line} ` : ""}${d.node}: ${d.prototype} -> ${d.singlePass}`)
  if (verbose || rawOnlyS.length + rawOnlyP.length > 0) {
    for (const l of rawOnlyP) console.log(`    raw prototype   ${l}`)
    for (const l of rawOnlyS) console.log(`    raw single pass ${l}`)
  }
  if (verbose || anomalies.length > 0) for (const a of anomalies) console.log(`    anomaly ${a.kind}: ${a.detail}`)
  for (const s of splits) console.log(`    split  ${s}`)
}

const total = (f: (r: Row) => number) => rows.reduce((n, r) => n + f(r), 0)
console.log(`\n${rows.length} files; ${total((r) => r.calls)} calls, ${total((r) => r.lambdas)} lambdas/methods`)
console.log(`decisions identical: ${rows.filter((r) => r.disagreements.length === 0).length}/${rows.length}; text identical: ${rows.filter((r) => r.textSame).length}/${rows.length}; fact fixpoints: ${rows.filter((r) => r.fixpoint).length}/${rows.length}; lowering fixpoints: ${rows.filter((r) => r.emissionFixpoint).length}/${rows.length}`)
console.log(`disagreements: ${total((r) => r.disagreements.length)}; anomalies: ${total((r) => r.anomalies.length)}`)
console.log(`lambdas classified inside overload resolution: ${total((r) => r.selections.overloaded)} with 2+ candidates seen; ${total((r) => r.selections.split)} where candidates disagreed; ${total((r) => r.selections.viaNeeds)} decided by body (no expected type); ${total((r) => r.selections.viaPipe)} by D8`)
console.log(`single check: ${total((r) => r.checkMs).toFixed(0)} ms total; TypeScript diagnostics in neutral text: ${total((r) => r.tsErrors)}`)
if (!filter) fs.writeFileSync(path.resolve(import.meta.dir, dirArg >= 0 ? `../results-${path.basename(dir)}.json` : "../results.json"), JSON.stringify(rows, null, 2))
