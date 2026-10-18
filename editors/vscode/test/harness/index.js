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
