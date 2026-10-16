// Run from the repository root: bun docs/tools/check-snippets.ts
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join, relative, resolve } from "node:path"
import { createRequire } from "node:module"

const repo = resolve(import.meta.dir, "../..")
export type Diagnostic = { code: string; message: string; line?: number; column?: number }
export type Block = { number: number; line: number; source: string; filename?: string; firstLine: string; expected: string[]; fragment: boolean; gap?: string }
export type Result = Block & { status: "ok" | "fragment" | "compiler-gap" | "fail"; diagnostics: Diagnostic[]; reason?: string }

export function extract(markdown: string): Block[] {
  const lines = markdown.split(/\r?\n/)
  const blocks: Block[] = []
  let number = 0
  for (let i = 0; i < lines.length; i++) {
    const fence = /^\s{0,3}(`{3,}|~{3,})([^`]*)$/.exec(lines[i])
    if (!fence) continue
    const start = i + 1
    const body: string[] = []
    const close = new RegExp(`^\\s{0,3}${fence[1][0]}{${fence[1].length},}\\s*$`)
    while (++i < lines.length && !close.test(lines[i])) body.push(lines[i])
    if (fence[2].trim().split(/\s+/)[0] !== "ts") continue
    number++
    const filename = body.slice(0, 2).map(line => /^\s*\/\/\s+((?:[\w.-]+\/)*[\w.-]+\.(?:es|ts))\s*$/.exec(line)?.[1]).find(Boolean)
    if (filename?.split("/").some(part => part === "." || part === "..")) throw new Error(`Invalid snippet filename: ${filename}`)
    if (/^\s*\/\/ TypeScript \(\.ts\)/.test(body[0] ?? "") && !filename) continue
    const headers: string[] = []
    for (const line of body) {
      if (!/^\s*\/\//.test(line)) break
      headers.push(line)
    }
    const header = headers.join("\n")
    blocks.push({ number, line: start + 1, source: body.join("\n"), filename,
      firstLine: body.find(l => l.trim() && !/^\s*\/\//.test(l)) ?? body[0] ?? "",
      expected: [...(header.match(/\/\/ expect-error:\s*([^\n]+)/)?.[1] ?? "").matchAll(/(?:ES|TS)\d+/g)].map(m => m[0]),
      fragment: /\/\/ fragment\s*(?:$|\n)/.test(header),
      gap: header.match(/\/\/ compiler-gap:\s*([^\n]+)/)?.[1] })
  }
  return blocks
}

// Repeated captions describe independent examples, so keep their numbered files.
// Unique captions are real source files, including TypeScript companion modules.
export function blockFilenames(blocks: Block[]): Map<number, string> {
  const counts = new Map<string, number>()
  for (const block of blocks) if (block.filename) counts.set(block.filename, (counts.get(block.filename) ?? 0) + 1)
  const used = new Set([...counts].filter(([, count]) => count === 1).map(([name]) => name))
  return new Map(blocks.map(block => {
    if (block.filename && counts.get(block.filename) === 1) return [block.number, block.filename]
    const extension = block.filename?.endsWith(".ts") ? "ts" : "es"
    let filename = `block-${block.number}.${extension}`
    for (let suffix = 2; used.has(filename); suffix++) filename = `block-${block.number}-${suffix}.${extension}`
    used.add(filename)
    return [block.number, filename]
  }))
}

function localImport(from: string, to: string): string {
  const path = relative(dirname(from), to).replaceAll("\\", "/")
  return path.startsWith(".") ? path : "./" + path
}

function missing(d: Diagnostic) {
  return d.code === "TS2304" || d.code === "TS2552" ||
    (d.code === "TS2307" && /module ['"]\.{1,2}\//.test(d.message))
}

export function classify(block: Block, diagnostics: Diagnostic[]): Result {
  const codes = new Set(diagnostics.map(d => d.code))
  const absent = block.expected.filter(c => !codes.has(c))
  const unexpected = diagnostics.filter(d => !block.expected.includes(d.code))
  if (block.fragment) return { ...block, diagnostics, status: "fragment", reason: "Explicit fragment" }
  if (block.gap) {
    if (!absent.length && !unexpected.some(d => !missing(d))) return { ...block, diagnostics, status: "fail", reason: "Stale compiler-gap marker; remove it" }
    return { ...block, diagnostics, status: "compiler-gap", reason: absent.length ? `${block.gap}; missing ${absent.join(", ")}` : block.gap }
  }
  if (absent.length) return { ...block, diagnostics, status: "fail", reason: `Expected ${absent.join(", ")} was not reported` }
  return { ...block, diagnostics, status: unexpected.length === 0 ? "ok" : unexpected.every(missing) ? "fragment" : "fail" }
}

export function parseDiagnostics(output: string): { file?: string; diagnostic: Diagnostic }[] {
  const result: { file?: string; diagnostic: Diagnostic }[] = []
  for (const line of output.split(/\r?\n/)) {
    const m = /^(.*)\((\d+),(\d+)\): error ((?:TS|ES)\d+): (.*)$/.exec(line)
    if (m) result.push({ file: m[1], diagnostic: { code: m[4], message: m[5], line: +m[2], column: +m[3] } })
    else {
      const global = /^error ((?:TS|ES)\d+): (.*)$/.exec(line)
      if (global) result.push({ diagnostic: { code: global[1], message: global[2] } })
    }
  }
  return result
}

function files(path: string): string[] {
  return readdirSync(path, { withFileTypes: true }).flatMap(e => e.isDirectory() ? files(join(path, e.name)) : e.name.endsWith(".md") ? [join(path, e.name)] : []).sort()
}

async function main() {
  const args = process.argv.slice(2)
  function option(name: string, fallback?: string) {
    const i = args.indexOf(name)
    if (i < 0) return fallback
    if (!args[i + 1] || args[i + 1].startsWith("--")) throw new Error(`${name} requires a value`)
    const value = args[i + 1]
    args.splice(i, 2)
    return value
  }
  if (args.includes("--help")) {
    console.log("bun docs/tools/check-snippets.ts [--compiler path] [--root docs/user] [--filter text] [--json path] [--keep]")
    return
  }
  const selectedCompiler = option("--compiler", process.env.ESC_BINARY || (existsSync(join(repo, "typescript/built/local/tsc")) ? join(repo, "typescript/built/local/tsc") : "esc"))!
  const compiler = selectedCompiler.includes("/") ? resolve(selectedCompiler) : selectedCompiler
  const root = resolve(option("--root", join(repo, "docs/user"))!)
  const filter = option("--filter", "")!
  const json = option("--json")
  const keep = args.includes("--keep")
  if (args.some(a => a !== "--keep")) throw new Error(`Unknown argument: ${args.join(" ")}`)
  // The experiments package pins this parser. It only discovers declarations;
  // all language checking belongs to the native EffectScript compiler.
  const require = createRequire(join(repo, "experiments/package.json"))
  const ts = require("typescript") as typeof import("../../experiments/node_modules/typescript")
  const temp = mkdtempSync(join(tmpdir(), "effectscript-docs-"))
  const pages: { page: string; blocks: Result[] }[] = []
  try {
    const modules = join(temp, "node_modules")
    mkdirSync(join(modules, "@effectscript"), { recursive: true })
    const dependencies = [join(repo, "experiments/node_modules"), join(repo, "docs/tools/node_modules")]
    for (const source of dependencies) {
      if (!existsSync(source)) continue
      for (const name of readdirSync(source).filter(n => !n.startsWith("."))) {
        if (name === "@effectscript") continue
        const target = join(modules, name)
        if (!existsSync(target)) symlinkSync(join(source, name), target)
      }
    }
    for (const name of ["runtime", "vite"]) symlinkSync(join(repo, "packages", name), join(modules, "@effectscript", name))
    if (!existsSync(join(modules, "effect/package.json")) || !existsSync(join(repo, "packages/runtime/dist/index.d.ts"))) {
      throw new Error("Install experiments dependencies and build packages/runtime before checking snippets (see docs/tools/README.md)")
    }
    let index = 0
    for (const page of files(root).filter(p => relative(root, p).includes(filter))) {
      const blocks = extract(readFileSync(page, "utf8"))
      if (!blocks.length) continue
      const dir = join(temp, String(++index))
      mkdirSync(dir)
      writeFileSync(join(dir, "package.json"), '{"type":"module","name":"docs-snippets"}')
      const filenames = blockFilenames(blocks)
      const scope = new Map<string, { from: string; value: boolean }>()
      const offsets = new Map<string, number>()
      for (const block of blocks) {
        const filename = filenames.get(block.number)!
        mkdirSync(dirname(join(dir, filename)), { recursive: true })
        if (filename.endsWith(".ts")) {
          offsets.set(filename, 0)
          writeFileSync(join(dir, filename), block.source + "\n")
          continue
        }
        const ast = ts.createSourceFile(filename, block.source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
        const names = new Set<string>()
        const exported = new Set<string>()
        const typeOnly = new Set<string>()
        const localValues = new Set<string>()
        const bindings = (node: any) => {
          if (ts.isIdentifier(node)) names.add(node.text)
          else if (ts.isObjectBindingPattern(node) || ts.isArrayBindingPattern(node)) {
            for (const e of node.elements) if (ts.isBindingElement(e)) bindings(e.name)
          }
        }
        for (const statement of ast.statements) {
          const before = new Set(names)
          if (ts.isImportDeclaration(statement)) {
            const clause = statement.importClause
            if (clause?.name) bindings(clause.name)
            const named = clause?.namedBindings
            if (named && ts.isNamespaceImport(named)) bindings(named.name)
            else if (named) for (const element of named.elements) bindings(element.name)
          } else if (ts.isVariableStatement(statement)) {
            for (const d of statement.declarationList.declarations) {
              bindings(d.name)
              if (ts.isIdentifier(d.name)) localValues.add(d.name.text)
            }
          } else if ("name" in statement && statement.name && ts.isIdentifier(statement.name)) bindings(statement.name)
          if (ts.isTypeAliasDeclaration(statement) || ts.isInterfaceDeclaration(statement)) typeOnly.add(statement.name.text)
          else for (const name of names) if (!before.has(name)) localValues.add(name)
          if (ts.canHaveModifiers(statement) && ts.getModifiers(statement)?.some(m => m.kind === ts.SyntaxKind.ExportKeyword) && !ts.getModifiers(statement)?.some(m => m.kind === ts.SyntaxKind.DefaultKeyword)) {
            for (const name of names) if (!before.has(name)) exported.add(name)
          }
        }
        for (const name of localValues) typeOnly.delete(name)
        const imports = new Map<string, string[]>()
        const values: string[] = []
        for (const [name, previous] of scope) {
          if (names.has(name) && !(typeOnly.has(name) && previous.value)) continue
          const list = imports.get(previous.from) ?? []
          if (typeOnly.has(name)) {
            list.push(`${name} as $doc_${name}`)
            values.push(`const ${name} = $doc_${name}`)
          } else list.push(name)
          imports.set(previous.from, list)
        }
        const prefix = [...[...imports].map(([from, ids]) => `import { ${ids.join(", ")} } from "${localImport(filename, from)}"`), ...values].join("\n") + "\n"
        offsets.set(filename, prefix.split("\n").length - 1)
        writeFileSync(join(dir, filename), `${prefix}${block.source}\nexport { ${[...names].filter(n => !exported.has(n)).join(", ")} }\n`)
        // An example that deliberately fails or opts out cannot supply reliable
        // definitions to later examples. Other blocks share their latest names.
        if (!block.fragment && !block.expected.length) for (const name of names) scope.set(name, { from: filename, value: !typeOnly.has(name) || values.some(v => v.startsWith(`const ${name} =`)) })
      }
      writeFileSync(join(dir, "tsconfig.json"), JSON.stringify({ compilerOptions: {
        strict: true, target: "ESNext", module: "NodeNext", moduleResolution: "NodeNext",
        skipLibCheck: true, noEmit: true, types: [], rewriteRelativeImportExtensions: true
      }, files: [...filenames.values()] }))
      const child = Bun.spawn([compiler, "--noEmit", "--pretty", "false", "-p", join(dir, "tsconfig.json")], { cwd: dir, stdout: "pipe", stderr: "pipe" })
      const [stdout, stderr, code] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
      const output = `${stdout}\n${stderr}`
      if (keep) writeFileSync(join(dir, "compiler.log"), output)
      const parsed = parseDiagnostics(output)
      const infrastructure = parsed.filter(d => !d.file || !offsets.has(relative(dir, resolve(dir, d.file)).replaceAll("\\", "/")))
      if (infrastructure.length || (code !== 0 && !parsed.length)) throw new Error(`Compiler failed for ${relative(repo, page)} (exit ${code}):\n${output}`)
      const results = blocks.map(block => {
        const filename = filenames.get(block.number)!
        const diagnostics = parsed.filter(d => d.file && relative(dir, resolve(dir, d.file)).replaceAll("\\", "/") === filename).map(({ diagnostic }) => ({
          ...diagnostic, line: block.line + diagnostic.line! - offsets.get(filename)! - 1
        }))
        return classify(block, diagnostics)
      })
      pages.push({ page: relative(repo, page), blocks: results })
      console.log(`\n${relative(repo, page)}\n| Block | Line | Status | Diagnostics |\n| --- | --- | --- | --- |`)
      for (const result of results) {
        console.log(`| ${result.number} | ${result.line} | ${result.status} | ${[...new Set(result.diagnostics.map(d => d.code))].join(", ") || "-"} |`)
        if (result.status === "fail") {
          if (result.reason) console.log(`  ${result.reason}`)
          for (const d of result.diagnostics) console.log(`  ${d.line}:${d.column} ${d.code}: ${d.message}`)
        }
      }
    }
    const totals = { pages: pages.length, blocks: 0, ok: 0, fragment: 0, "compiler-gap": 0, fail: 0 }
    for (const page of pages) for (const block of page.blocks) { totals.blocks++; totals[block.status]++ }
    console.log(`\nTotals: ${JSON.stringify(totals)}`)
    if (json) writeFileSync(resolve(json), JSON.stringify({ compiler, totals, pages }, null, 2) + "\n")
    if (totals.fail) process.exitCode = 1
  } finally {
    if (keep) console.log(`Temporary projects: ${temp}`)
    else rmSync(temp, { recursive: true, force: true })
  }
}

if (import.meta.main) main().catch(error => { console.error(error.message); process.exitCode = 2 })
