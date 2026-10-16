// Which decisions does the prototype change after pass 1? Runs the replica of
// the prototype's loop over the corpus and prints, per file and pass, every
// decision that differs from the previous pass's emission.
//
//   bun prototype/single-pass/harness/passes.ts [filter]
import fs from "node:fs"
import path from "node:path"
import { type Decisions, ESC, lineOf, runReplica } from "./replica.ts"

const dir = path.join(ESC, "examples")
const filter = process.argv[2]
const files = fs.readdirSync(dir).filter((f) => f.endsWith(".es") && (!filter || f.includes(filter))).sort()

const diff = (a: Decisions, b: Decisions) => {
  const out: string[] = []
  const src = (file: any, node: any) => node.getText(file.sf).replace(/\s+/g, " ").slice(0, 60)
  for (const [k, v] of b.calls) {
    const u = a.calls.get(k)
    if (!u || u.emit !== v.emit) out.push(`call   L${lineOf(v.file, v.node)} ${src(v.file, v.node)}: ${u?.emit ?? "-"} -> ${v.emit}`)
  }
  for (const [k, v] of b.lambdas) {
    const u = a.lambdas.get(k)
    if (!u || u.mode !== v.mode) out.push(`lambda L${lineOf(v.file, v.node)} ${src(v.file, v.node)}: ${u?.mode ?? "-"} -> ${v.mode} (ctx ${v.ctx})`)
  }
  const keys = new Set([...a.convs.keys(), ...b.convs.keys()])
  for (const k of keys) {
    const u = a.convs.get(k)
    const v = b.convs.get(k)
    if (u?.conv !== v?.conv) {
      const r = (v ?? u)!
      out.push(`conv   L${lineOf(r.file, r.node)} ${src(r.file, r.node)}: ${u?.conv || "-"} -> ${v?.conv || "-"}`)
    }
  }
  for (const k of new Set([...a.loops.keys(), ...b.loops.keys()])) {
    const u = a.loops.get(k)
    const v = b.loops.get(k)
    if (u?.kind !== v?.kind) {
      const r = (v ?? u)!
      out.push(`loop   L${lineOf(r.file, r.node)}: ${u?.kind ?? "plain"} -> ${v?.kind ?? "plain"}`)
    }
  }
  for (const d of b.direct) if (!a.direct.has(d)) out.push(`direct ${d.split("#")[1]}: no -> yes`)
  for (const d of a.direct) if (!b.direct.has(d)) out.push(`direct ${d.split("#")[1]}: yes -> no`)
  for (const p of b.pipes) if (!a.pipes.has(p)) out.push(`pipe   ${p}: -> program`)
  for (const [k, v] of b.rec) if (a.rec.get(k) !== v) out.push(`rec    ${k}: ${a.rec.get(k) ?? "(default any/never/never)"} -> ${v}`)
  for (const [k, v] of b.wire) if (a.wire.get(k) !== v) out.push(`wire   ${k}: ${a.wire.get(k) ?? "source"} -> ${v}`)
  return out
}

const summary: string[] = []
for (const name of files) {
  const r = runReplica([path.join(dir, name)])
  const n = r.passes.length
  console.log(`\n=== ${name}: ${n} passes${r.converged ? "" : " (NOT converged)"}`)
  for (let i = 1; i < n; i++) {
    const d = diff(r.passes[i - 1].decisions, r.passes[i].decisions)
    console.log(`  pass ${i} -> ${i + 1}: ${d.length} change(s)`)
    for (const line of d) console.log(`    ${line}`)
  }
  // Sanity: the replica's final emission equals the prototype's committed output.
  for (const [es, text] of r.passes[n - 1].text) {
    const out = path.join(dir, "out", path.basename(es).replace(/\.es$/, ".ts"))
    const same = fs.existsSync(out) && fs.readFileSync(out, "utf8") === text
    if (!same) console.log(`  !! final emission differs from ${path.relative(ESC, out)}`)
  }
  summary.push(`${name.padEnd(40)} ${n}`)
}
console.log(`\n=== passes per file\n${summary.join("\n")}`)
