// Runs inside the VS Code that test/run.mjs starts. It checks the EffectScript
// extension installed from effectscript.vsix on a copy of the language server
// fixture (typescript/effectscript/lsp-fixture), whose node_modules/.bin/esc
// is the compiler under test. run() throws when any check fails.
const fs = require("node:fs")
const path = require("node:path")
const vscode = require("vscode")

const workspace = process.env.EFFECTSCRIPT_TEST_WORKSPACE
const extensionsDir = process.env.EFFECTSCRIPT_TEST_EXTENSIONS
const timeoutMs = Number(process.env.EFFECTSCRIPT_TEST_TIMEOUT ?? 90_000)

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

  await check("extension: effectscript.effectscript comes from the .vsix and activates", async () => {
    const extension = vscode.extensions.getExtension("effectscript.effectscript")
    assert(extension, "the extension is not installed")
    assert(extension.extensionPath.startsWith(extensionsDir), `loaded from ${extension.extensionPath}, not ${extensionsDir}`)
    await extension.activate()
  })

  if (process.env.EFFECTSCRIPT_TEST_PASS === "typescript-off") {
    // test/run.mjs disabled the built-in TypeScript extension, so only
    // EffectScript's server can report this .ts file's diagnostics.
    await check("typescript: with VS Code's TypeScript off, EffectScript reports .ts diagnostics", async () => {
      assert(vscode.extensions.getExtension("vscode.typescript-language-features") === undefined, "the built-in TypeScript extension is enabled")
      fs.writeFileSync(path.join(workspace, "src", "typed.ts"), 'export const count: number = "three"\n')
      const typed = await open("src/typed.ts")
      await waitFor("TS2322 in typed.ts", () => vscode.languages.getDiagnostics(typed.uri).find((d) => codeOf(d) === 2322))
    })
    if (failures > 0) throw new Error(`${failures} check(s) failed`)
    return
  }

  const main = await open("src/main.es")

  await check("language: an .es file opens as effectscript", () => {
    assert(main.languageId === "effectscript", `language ${main.languageId}`)
  })

  await check("grammar: the EffectScript grammar colors .es files", async () => {
    const tokens = await vscode.commands.executeCommand("_workbench.captureSyntaxTokens", main.uri)
    const keyword = tokens.find((token) => token.c.trim() === "function")
    assert(keyword, `no function token in ${tokens.length} tokens`)
    assert(keyword.t.startsWith("source.effectscript") && keyword.t.includes("storage.type.function"), `scopes ${keyword.t}`)
  })

  await check("diagnostics: TypeScript's TS2322 appears on broken.es", async () => {
    const broken = await open("src/broken.es")
    const found = await waitFor("TS2322", () => vscode.languages.getDiagnostics(broken.uri).find((d) => codeOf(d) === 2322))
    assert(found.range.start.line === 1, `line ${found.range.start.line}`)
  })

  await check("diagnostics: ES1003 appears on wait.es with source ES", async () => {
    const wait = await open("src/wait.es")
    const found = await waitFor("ES1003", () => vscode.languages.getDiagnostics(wait.uri).find((d) => codeOf(d) === 1003))
    assert(found.source === "ES", `source ${found.source}`)
  })

  await check("hover: an .es function shows its inferred contract", async () => {
    const shown = await hoverText(main, "lookup(id")
    assert(/function lookup\(id: number\): (Effect\.)?Effect<string, NotFound, never>/.test(shown), `hover ${JSON.stringify(shown)}`)
  })

  await check("hover: a call that runs shows its success value", async () => {
    const shown = await hoverText(main, "lookup(1)")
    assert(shown.includes("Runs here and returns `string`"), `hover ${JSON.stringify(shown)}`)
  })

  await check("definition: a .ts function called from .es goes to users.ts", async () => {
    const locations = await vscode.commands.executeCommand("vscode.executeDefinitionProvider", main.uri, positionOf(main, "findName(id)"))
    const uris = locations.map((l) => (l.targetUri ?? l.uri).fsPath)
    assert(uris.some((uri) => uri.endsWith(path.join("src", "users.ts"))), `went to ${JSON.stringify(uris)}`)
  })

  await check("diagnostics: main.es has none", () => {
    const items = vscode.languages.getDiagnostics(main.uri)
    assert(items.length === 0, JSON.stringify(items.map((d) => d.message)))
  })

  if (failures > 0) throw new Error(`${failures} check(s) failed`)
}

module.exports = { run }
