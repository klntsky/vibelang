// Derives the EffectScript TextMate grammar from VS Code's TypeScript grammar.
//
// EffectScript adds no syntax to TypeScript, so the grammar is TypeScript's
// with a new name and root scope, source.effectscript. Inner scopes keep
// their .ts suffix, so every theme rule written for TypeScript colors
// EffectScript the same way. The JSDoc injection is retargeted to the new
// root scope. Both source files are MIT-licensed; see ThirdPartyNotices.txt.
//
// Usage: node scripts/derive-grammar.mjs [typescript-basics directory]
//
// The directory defaults to the typescript-basics extension inside
// /Applications/Visual Studio Code.app.
import { readFileSync, writeFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const here = path.dirname(fileURLToPath(import.meta.url))
const source =
  process.argv[2] ?? "/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/typescript-basics"
const syntaxes = path.join(here, "..", "syntaxes")
const read = (file) => JSON.parse(readFileSync(path.join(source, "syntaxes", file), "utf8"))
const write = (file, json) => writeFileSync(path.join(syntaxes, file), JSON.stringify(json, null, "\t") + "\n")

const note =
  "Derived for EffectScript by editors/vscode/scripts/derive-grammar.mjs from the TypeScript grammar that VS Code ships; only name and scopeName differ."

const grammar = read("TypeScript.tmLanguage.json")
const { information_for_contributors: info = [], version, name, scopeName, patterns, repository, ...other } = grammar
if (name !== "TypeScript" || scopeName !== "source.ts") throw new Error(`unexpected grammar ${name} (${scopeName})`)
if (Object.keys(other).length > 0) throw new Error(`unexpected grammar keys: ${Object.keys(other).join(", ")}`)
write("EffectScript.tmLanguage.json", {
  information_for_contributors: [note, ...info],
  version,
  name: "EffectScript",
  scopeName: "source.effectscript",
  patterns,
  repository
})

const injection = read("jsdoc.ts.injection.tmLanguage.json")
const text = JSON.stringify(injection)
if (!text.includes("source.ts#docblock")) throw new Error("the JSDoc injection no longer includes source.ts#docblock")
write(
  "jsdoc.effectscript.injection.tmLanguage.json",
  JSON.parse(
    text.replaceAll("source.ts#docblock", "source.effectscript#docblock").replace(
      '"scopeName":"documentation.injection.ts"',
      '"scopeName":"documentation.injection.effectscript"'
    )
  )
)
console.log(`wrote ${path.relative(process.cwd(), syntaxes)}/EffectScript.tmLanguage.json (TypeScript-TmLanguage ${version})`)
