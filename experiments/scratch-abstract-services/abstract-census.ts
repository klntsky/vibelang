// How are abstract classes used in real TypeScript codebases? (read-only)
import { readdirSync, readFileSync, statSync } from "node:fs"
import { join } from "node:path"
import ts from "typescript"
const SKIP = new Set(["node_modules", "dist", "build", ".git", ".jj", ".artifacts", "vendor", "coverage"])
const files: string[] = []
const walk = (d: string) => { for (const n of readdirSync(d)) { if (SKIP.has(n)) continue; const p = join(d, n); const s = statSync(p, { throwIfNoEntry: false }); if (!s) continue; if (s.isDirectory()) walk(p); else if (/\.tsx?$/.test(n) && !n.endsWith(".d.ts")) files.push(p) } }
for (const r of process.argv.slice(2)) walk(r)
const abstracts: Array<{ name: string; fullyAbstract: boolean; generic: boolean; hasExtends: boolean; file: string }> = []
const extended = new Map<string, number>()
for (const f of files) {
  const sf = ts.createSourceFile(f, readFileSync(f, "utf8"), ts.ScriptTarget.Latest, true)
  const visit = (n: ts.Node) => {
    if (ts.isClassDeclaration(n) || ts.isClassExpression(n)) {
      const heritage = n.heritageClauses?.find((h) => h.token === ts.SyntaxKind.ExtendsKeyword)
      if (heritage) for (const t of heritage.types) if (ts.isIdentifier(t.expression)) extended.set(t.expression.text, (extended.get(t.expression.text) ?? 0) + 1)
      const isAbstract = n.modifiers?.some((m) => m.kind === ts.SyntaxKind.AbstractKeyword)
      if (isAbstract && n.name) {
        const instance = n.members.filter((m) => !m.modifiers?.some((x) => x.kind === ts.SyntaxKind.StaticKeyword) && !ts.isConstructorDeclaration(m))
        const fullyAbstract = instance.length > 0 && instance.every((m) => m.modifiers?.some((x) => x.kind === ts.SyntaxKind.AbstractKeyword))
        abstracts.push({ name: n.name.text, fullyAbstract, generic: !!n.typeParameters?.length, hasExtends: !!heritage, file: f })
      }
    }
    ts.forEachChild(n, visit)
  }
  visit(sf)
}
const fully = abstracts.filter((a) => a.fullyAbstract)
console.log(`files: ${files.length}; abstract classes: ${abstracts.length}; fully abstract (only abstract instance members): ${fully.length}`)
console.log(`fully abstract and subclassed somewhere (by name): ${fully.filter((a) => extended.has(a.name)).length}; generic: ${fully.filter((a) => a.generic).length}; extending something: ${fully.filter((a) => a.hasExtends).length}`)
console.log("examples:", fully.slice(0, 8).map((a) => `${a.name}${extended.has(a.name) ? ` (extended ${extended.get(a.name)}x)` : ""}`).join(", "))
