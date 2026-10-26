// Count how a real Effect codebase uses pipe, to ground the pipe-syntax design.
// Usage: bun scripts/pipe-census.ts <root>...   (read-only; parses .ts/.tsx that import "effect")
import { readdirSync, readFileSync, statSync } from "node:fs"
import { join } from "node:path"
import ts from "typescript"

const SKIP = new Set(["node_modules", "dist", "build", ".git", ".jj", ".artifacts", "coverage", ".turbo", ".next"])
const files: string[] = []
const walk = (dir: string) => {
  for (const name of readdirSync(dir)) {
    if (SKIP.has(name)) continue
    const path = join(dir, name)
    const s = statSync(path, { throwIfNoEntry: false })
    if (!s) continue
    if (s.isDirectory()) walk(path)
    else if (/\.(ts|tsx)$/.test(name) && !name.endsWith(".d.ts")) files.push(path)
  }
}
for (const root of process.argv.slice(2)) walk(root)

// Effect stages grouped by what VibeLang turns them into.
const STATEMENT = {
  sequencing: ["map", "flatMap", "andThen", "tap", "as", "asVoid", "asSome", "zipRight", "zipLeft", "flatten", "let", "bind", "bindTo", "tapBoth", "when", "whenEffect", "unless", "zipWith"],
  errors: ["catchTag", "catchTags", "catchAll", "catch", "catchIf", "catchSome", "orElse", "orElseSucceed", "orElseFail", "mapError", "tapError", "tapErrorTag", "filterOrFail", "filterOrElse", "mapBoth", "flip"],
  cleanup: ["ensuring", "onExit", "onError", "onInterrupt"]
}
const COMBINATOR = {
  context: ["provide", "provideService", "provideServiceEffect", "provideContext", "updateService", "withConfigProvider", "locally", "provideMerge"],
  observability: ["withSpan", "withLogSpan", "annotateLogs", "annotateSpans", "withSpanScoped", "withTracer", "withParentSpan", "tapErrorCause", "withMetric"],
  policies: ["retry", "repeat", "timeout", "timeoutFail", "timeoutTo", "timeoutOption", "timeoutOrElse", "delay", "cached", "cachedWithTTL", "uninterruptible", "interruptible", "race", "raceFirst", "fork", "forkDaemon", "forkDetach", "forkScoped", "forkChild", "forkIn", "schedule", "withConcurrency", "scoped"],
  exitShape: ["orDie", "orDieWith", "either", "option", "exit", "result", "ignore", "ignoreLogged", "sandbox", "catchCause", "catchAllCause", "catchDefect", "catchAllDefect", "match", "matchEffect", "matchCause", "matchCauseEffect"]
}
const categoryOf = new Map<string, string>()
for (const [group, table] of [["statement", STATEMENT], ["combinator", COMBINATOR]] as const)
  for (const [cat, names] of Object.entries(table)) for (const n of names) categoryOf.set(n, `${group}/${cat}`)

const stageName = (e: ts.Expression): string => {
  if (ts.isCallExpression(e)) return stageName(e.expression)
  if (ts.isPropertyAccessExpression(e)) {
    const left = ts.isIdentifier(e.expression) ? e.expression.text : "?"
    return `${left}.${e.name.text}`
  }
  if (ts.isIdentifier(e)) return e.text
  if (ts.isArrowFunction(e) || ts.isFunctionExpression(e)) return "<lambda>"
  return "<other>"
}
const headKind = (e: ts.Expression): string => {
  if (ts.isCallExpression(e)) {
    const n = stageName(e)
    if (n === "Effect.gen" || n === "Effect.fn" || n === "Effect.fnUntraced") return "Effect.gen(...)"
    if (n.startsWith("Effect.")) return "Effect.* constructor call"
    return "a function call"
  }
  if (ts.isIdentifier(e) || ts.isPropertyAccessExpression(e)) return "a stored value"
  return "other"
}

const pipeKinds = new Map<string, number>()
const effectStages = new Map<string, number>()
const categories = new Map<string, number>()
const heads = new Map<string, number>()
let effectPipes = 0, allPipes = 0, filesWithEffect = 0, stagesPerEffectPipe = 0
// After statements absorb sequencing, error, and cleanup stages, how many stages remain per pipe?
const remaining = new Map<string, number>()
const remainingInTests = new Map<string, number>()
const bump = (m: Map<string, number>, k: string, by = 1) => m.set(k, (m.get(k) ?? 0) + by)

for (const file of files) {
  const text = readFileSync(file, "utf8")
  if (!/from\s+["']effect(\/[\w-]+)?["']/.test(text)) continue
  filesWithEffect++
  const sf = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, file.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
  const visit = (node: ts.Node) => {
    if (ts.isCallExpression(node)) {
      let head: ts.Expression | undefined
      let stages: readonly ts.Expression[] = []
      if (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "pipe" && node.arguments.length > 0) {
        head = node.expression.expression; stages = node.arguments
      } else if (ts.isIdentifier(node.expression) && node.expression.text === "pipe" && node.arguments.length > 1) {
        head = node.arguments[0]; stages = node.arguments.slice(1)
      }
      if (head) {
        allPipes++
        const names = stages.map(stageName)
        const modules = names.map((n) => (n.includes(".") ? n.split(".")[0]! : n))
        const counts = new Map<string, number>()
        for (const m of modules) bump(counts, m)
        const top = [...counts].sort((a, b) => b[1] - a[1])[0]![0]
        const kind = ["Effect", "Stream", "Layer", "Schema", "Option", "Array", "Arr", "Record", "Either", "Result", "Match", "Sink", "Channel", "Schedule", "Duration", "Struct", "String"].includes(top) ? top : "other"
        bump(pipeKinds, kind)
        if (kind === "Effect") {
          effectPipes++
          stagesPerEffectPipe += stages.length
          bump(heads, headKind(head))
          const left = names.filter((n) => !(n.startsWith("Effect.") && (categoryOf.get(n.slice(7)) ?? "").startsWith("statement/"))).length
          const bucket = left >= 3 ? "3+" : String(left)
          bump(remaining, bucket)
          if (/(\.test\.|\.spec\.|\/tests?\/)/.test(file)) bump(remainingInTests, bucket)
          for (const n of names) {
            if (!n.startsWith("Effect.")) continue
            const fn = n.slice("Effect.".length)
            bump(effectStages, fn)
            bump(categories, categoryOf.get(fn) ?? "other")
          }
        }
      }
    }
    ts.forEachChild(node, visit)
  }
  visit(sf)
}

const pct = (n: number, d: number) => `${((100 * n) / d).toFixed(1)}%`
const totalEffectStages = [...effectStages.values()].reduce((a, b) => a + b, 0)
console.log(`files importing effect: ${filesWithEffect}; pipe calls: ${allPipes}; Effect pipes: ${effectPipes} (${(stagesPerEffectPipe / effectPipes).toFixed(2)} stages each)`)
console.log("pipe kind by majority stage module:", Object.fromEntries([...pipeKinds].sort((a, b) => b[1] - a[1])))
console.log("Effect pipe heads:", Object.fromEntries([...heads].sort((a, b) => b[1] - a[1]).map(([k, v]) => [k, `${v} (${pct(v, effectPipes)})`])))
console.log("Effect pipes by stages left once statements absorb sequencing, errors, cleanup:",
  Object.fromEntries(["0", "1", "2", "3+"].map((k) => [k, `${remaining.get(k) ?? 0} (${pct(remaining.get(k) ?? 0, effectPipes)}; in tests ${remainingInTests.get(k) ?? 0})`])))
console.log(`Effect.* stages: ${totalEffectStages}`)
for (const [k, v] of [...categories].sort((a, b) => b[1] - a[1])) console.log(`  ${k.padEnd(26)} ${String(v).padStart(6)}  ${pct(v, totalEffectStages)}`)
console.log("top stages:", [...effectStages].sort((a, b) => b[1] - a[1]).slice(0, 30).map(([k, v]) => `${k} ${v}`).join(", "))
console.log("top unclassified:", [...effectStages].filter(([k]) => !categoryOf.has(k)).sort((a, b) => b[1] - a[1]).slice(0, 15).map(([k, v]) => `${k} ${v}`).join(", "))
