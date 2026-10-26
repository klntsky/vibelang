// Runs inside the VS Code that test/run.mjs starts. It checks the VibeLang
// extension installed from vibelang.vsix on a copy of the language server
// fixture (typescript/vibelang/lsp-fixture), whose node_modules/.bin/vibelang
// is the compiler under test. run() throws when any check fails.
const fs = require("node:fs")
const path = require("node:path")
const vscode = require("vscode")

const workspace = process.env.VIBELANG_TEST_WORKSPACE
const extensionsDir = process.env.VIBELANG_TEST_EXTENSIONS
const timeoutMs = Number(process.env.VIBELANG_TEST_TIMEOUT ?? 90_000)

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

async function waitFor(what, probe) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    const value = await probe()
    if (value) return value
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`)
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
}

async function open(relative) {
  const document = await vscode.workspace.openTextDocument(vscode.Uri.file(path.join(workspace, relative)))
  await vscode.window.showTextDocument(document, { preview: false })
  return document
}

function positionOf(document, needle, offset = 0) {
  const index = document.getText().indexOf(needle)
  if (index < 0) throw new Error(`${path.basename(document.uri.fsPath)} has no ${JSON.stringify(needle)}`)
  return document.positionAt(index + offset)
}

async function hoverText(document, needle) {
  const hovers = await vscode.commands.executeCommand("vscode.executeHoverProvider", document.uri, positionOf(document, needle))
  return hovers.flatMap((hover) => hover.contents.map((part) => (typeof part === "string" ? part : part.value))).join("\n")
}

// The tokens VS Code's grammars give text, written to grammar/<name> in the
// project, outside the program (tsconfig.json includes only src/). Each token
// is its text, c, and its scopes, t, such as "source.vibe meta.import.ts ...".
async function syntaxTokens(name, text) {
  const file = path.join(workspace, "grammar", name)
  fs.mkdirSync(path.dirname(file), { recursive: true })
  fs.writeFileSync(file, text)
  const tokens = await vscode.commands.executeCommand("_workbench.captureSyntaxTokens", vscode.Uri.file(file))
  return tokens.map(({ c, t }) => ({ c, t }))
}

// The first token where a .vibe file's tokens differ from a .ts file's, with
// the root scopes source.vibe and source.ts counted as the same, else "".
function firstDifference(vibe, ts) {
  const index = ts.findIndex((token, i) => !vibe[i] || token.c !== vibe[i].c || token.t !== vibe[i].t.replace(/^source\.vibe\b/, "source.ts"))
  return index < 0 ? "" : `token ${index} is ${JSON.stringify(vibe[index])}, TypeScript's is ${JSON.stringify(ts[index])}`
}

function codeOf(diagnostic) {
  return typeof diagnostic.code === "object" ? diagnostic.code.value : diagnostic.code
}

async function run() {
  let failures = 0
  const check = async (name, body) => {
    try {
      await body()
      console.log(`ok   ${name}`)
    } catch (error) {
      failures++
      console.log(`FAIL ${name}: ${error.message}`)
    }
  }

  await check("extension: vibelang.vibelang comes from the .vsix and activates", async () => {
    const extension = vscode.extensions.getExtension("vibelang.vibelang")
    assert(extension, "the extension is not installed")
    assert(extension.extensionPath.startsWith(extensionsDir), `loaded from ${extension.extensionPath}, not ${extensionsDir}`)
    await extension.activate()
  })

  if (process.env.VIBELANG_TEST_PASS === "typescript-off") {
    // test/run.mjs disabled the built-in TypeScript extension, so only
    // VibeLang's server can report this .ts file's diagnostics.
    await check("typescript: with VS Code's TypeScript off, VibeLang reports .ts diagnostics", async () => {
      assert(vscode.extensions.getExtension("vscode.typescript-language-features") === undefined, "the built-in TypeScript extension is enabled")
      fs.writeFileSync(path.join(workspace, "src", "typed.ts"), 'export const count: number = "three"\n')
      const typed = await open("src/typed.ts")
      await waitFor("TS2322 in typed.ts", () => vscode.languages.getDiagnostics(typed.uri).find((d) => codeOf(d) === 2322))
    })
    if (failures > 0) throw new Error(`${failures} check(s) failed`)
    return
  }

  const main = await open("src/main.vibe")

  await check("language: a .vibe file opens as vibe", () => {
    assert(main.languageId === "vibe", `language ${main.languageId}`)
  })

  await check("grammar: the VibeLang grammar colors .vibe files", async () => {
    const tokens = await vscode.commands.executeCommand("_workbench.captureSyntaxTokens", main.uri)
    const keyword = tokens.find((token) => token.c.trim() === "function")
    assert(keyword, `no function token in ${tokens.length} tokens`)
    assert(keyword.t.startsWith("source.vibe") && keyword.t.includes("storage.type.function"), `scopes ${keyword.t}`)
  })

  await check("grammar: the catch of a catch expression is a keyword", async () => {
    const file = await open("src/catch-expression.vibe")
    const tokens = await vscode.commands.executeCommand("_workbench.captureSyntaxTokens", file.uri)
    const keywords = tokens.filter((token) => token.c.trim() === "catch")
    assert(keywords.length === 2, `catch tokens ${JSON.stringify(keywords)}`)
    for (const keyword of keywords) assert(keyword.t.includes("keyword.control.trycatch"), `scopes ${keyword.t}`)
  })

  await check("grammar: a module declaration's body is colored as TypeScript colors it", async () => {
    // Braces in the body's strings, templates, regular expression and
    // comments do not end it. VibeLang's own forms, such as the error
    // declaration that the compiler rejects there (D283), are TypeScript's.
    const body = [
      '  import { Effect } from "effect"',
      "",
      '  export const close = "}"',
      "  export const braces = /[{}]/g",
      '  export const label = (name: string) => `{${name}${"}"}}`',
      "  export const page = `",
      "    <main>{ } }</main>",
      "  `",
      "  // a } in a comment",
      "  /* a { and",
      "     a } in a comment */",
      "",
      "  export function clean(value: string): string {",
      "    const trimmed = value.trim()",
      '    if (trimmed === "") throw new Error("empty")',
      "    return trimmed",
      "  }",
      "",
      "  export const greet = (name: string) => Effect.succeed({ greeting: `hello, ${name}` })",
      "",
      "  error Bad { reason: string }"
    ]
    const vibe = await syntaxTokens("module.vibe", ["module text {", ...body, "}", "", "error NotFound { id: string }", ""].join("\n"))
    const ts = await syntaxTokens("namespace.ts", ["namespace text {", ...body, "}", ""].join("\n"))
    assert(vibe[0].c === "module" && ts[0].c === "namespace", `first tokens ${JSON.stringify([vibe[0], ts[0]])}`)
    const difference = firstDifference([{ ...vibe[0], c: "namespace" }, ...vibe.slice(1)], ts)
    assert(difference === "", difference)
    const after = vibe.slice(ts.length).find((token) => token.c === "error")
    assert(after?.t === "source.vibe meta.class.ts storage.type.class.ts", `the error after the module is ${JSON.stringify(after)}`)
  })

  await check("grammar: an identifier module specifier is a module name", async () => {
    const tokens = await syntaxTokens(
      "specifiers.vibe",
      [
        "module text {",
        "  export const clean = (value: string) => value.trim()",
        "}",
        "",
        "import { clean } from text",
        "import text",
        "export { clean as tidy } from text",
        "export * from text",
        ""
      ].join("\n")
    )
    const names = tokens.filter((token) => token.c === "text")
    assert(names.length === 5, `text tokens ${JSON.stringify(names)}`)
    for (const name of names) assert(name.t.endsWith(" entity.name.type.module.ts"), `scopes ${name.t}`)
    const exports = tokens.filter((token) => token.c === "export")
    assert(exports.length === 3, `export tokens ${JSON.stringify(exports)}`)
    for (const keyword of exports) assert(keyword.t.endsWith(" keyword.control.export.ts"), `scopes ${keyword.t}`)
  })

  await check("grammar: namespace and declare module keep TypeScript's scopes", async () => {
    const text = ["namespace tools {", "  export const version = 1", "}", "", 'declare module "effect-extra" {', "  export const extra: number", "}", ""].join("\n")
    const vibe = await syntaxTokens("namespaces.vibe", text)
    const ts = await syntaxTokens("namespaces.ts", text)
    const difference = firstDifference(vibe, ts)
    assert(difference === "" && vibe.length === ts.length, difference || `${vibe.length} tokens, TypeScript's ${ts.length}`)
  })

  await check("diagnostics: TypeScript's TS2322 appears on broken.vibe", async () => {
    const broken = await open("src/broken.vibe")
    const found = await waitFor("TS2322", () => vscode.languages.getDiagnostics(broken.uri).find((d) => codeOf(d) === 2322))
    assert(found.range.start.line === 1, `line ${found.range.start.line}`)
  })

  await check("diagnostics: EK1003 appears on wait.vibe with source EK", async () => {
    const wait = await open("src/wait.vibe")
    const found = await waitFor("EK1003", () => vscode.languages.getDiagnostics(wait.uri).find((d) => codeOf(d) === 1003))
    assert(found.source === "EK", `source ${found.source}`)
  })

  await check("hover: a .vibe function shows its inferred contract", async () => {
    const shown = await hoverText(main, "lookup(id")
    assert(shown.includes("function lookup(id: number): string!NotFound!"), `hover ${JSON.stringify(shown)}`)
  })

  await check("hover: a call that runs shows its success value", async () => {
    const shown = await hoverText(main, "lookup(1)")
    assert(shown.includes("Runs here and returns `string`"), `hover ${JSON.stringify(shown)}`)
  })

  await check("definition: a .ts function called from .vibe goes to users.ts", async () => {
    const locations = await vscode.commands.executeCommand("vscode.executeDefinitionProvider", main.uri, positionOf(main, "findName(id)"))
    const uris = locations.map((l) => (l.targetUri ?? l.uri).fsPath)
    assert(uris.some((uri) => uri.endsWith(path.join("src", "users.ts"))), `went to ${JSON.stringify(uris)}`)
  })

  await check("completion: prelude names and a service's members in services.vibe", async () => {
    const services = await open("src/services.vibe")
    const labels = async (needle, offset) => {
      const list = await vscode.commands.executeCommand("vscode.executeCompletionItemProvider", services.uri, positionOf(services, needle, offset))
      return list.items.map((item) => (typeof item.label === "string" ? item.label : item.label.label))
    }
    const scope = await labels("Users.find", 0)
    for (const label of ["TaggedError", "wire", "Users", "double"]) assert(scope.includes(label), `no ${label} in completions`)
    assert((await labels("Users.find", 6)).includes("find"), "no find after Users.")
  })

  await check("hover: a plain function shows the signature VibeLang calls", async () => {
    const services = await open("src/services.vibe")
    const shown = await hoverText(services, "double(n")
    assert(shown.includes("function double(n: number): number"), `hover ${JSON.stringify(shown)}`)
  })

  await check("diagnostics: main.vibe has none", () => {
    const items = vscode.languages.getDiagnostics(main.uri)
    assert(items.length === 0, JSON.stringify(items.map((d) => d.message)))
  })

  if (failures > 0) throw new Error(`${failures} check(s) failed`)
}

module.exports = { run }
