// Runs inside the VS Code that test/run.mjs starts. It checks the Effeck
// extension installed from effeck.vsix on a copy of the language server
// fixture (typescript/effeck/lsp-fixture), whose node_modules/.bin/effeck
// is the compiler under test. run() throws when any check fails.
const fs = require("node:fs")
const path = require("node:path")
const vscode = require("vscode")

const workspace = process.env.EFFECK_TEST_WORKSPACE
const extensionsDir = process.env.EFFECK_TEST_EXTENSIONS
const timeoutMs = Number(process.env.EFFECK_TEST_TIMEOUT ?? 90_000)

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

  await check("extension: effeck.effeck comes from the .vsix and activates", async () => {
    const extension = vscode.extensions.getExtension("effeck.effeck")
    assert(extension, "the extension is not installed")
    assert(extension.extensionPath.startsWith(extensionsDir), `loaded from ${extension.extensionPath}, not ${extensionsDir}`)
    await extension.activate()
  })

  if (process.env.EFFECK_TEST_PASS === "typescript-off") {
    // test/run.mjs disabled the built-in TypeScript extension, so only
    // Effeck's server can report this .ts file's diagnostics.
    await check("typescript: with VS Code's TypeScript off, Effeck reports .ts diagnostics", async () => {
      assert(vscode.extensions.getExtension("vscode.typescript-language-features") === undefined, "the built-in TypeScript extension is enabled")
      fs.writeFileSync(path.join(workspace, "src", "typed.ts"), 'export const count: number = "three"\n')
      const typed = await open("src/typed.ts")
      await waitFor("TS2322 in typed.ts", () => vscode.languages.getDiagnostics(typed.uri).find((d) => codeOf(d) === 2322))
    })
    if (failures > 0) throw new Error(`${failures} check(s) failed`)
    return
  }

  const main = await open("src/main.effeck")

  await check("language: an .effeck file opens as effeck", () => {
    assert(main.languageId === "effeck", `language ${main.languageId}`)
  })

  await check("grammar: the Effeck grammar colors .effeck files", async () => {
    const tokens = await vscode.commands.executeCommand("_workbench.captureSyntaxTokens", main.uri)
    const keyword = tokens.find((token) => token.c.trim() === "function")
    assert(keyword, `no function token in ${tokens.length} tokens`)
    assert(keyword.t.startsWith("source.effeck") && keyword.t.includes("storage.type.function"), `scopes ${keyword.t}`)
  })

  await check("diagnostics: TypeScript's TS2322 appears on broken.effeck", async () => {
    const broken = await open("src/broken.effeck")
    const found = await waitFor("TS2322", () => vscode.languages.getDiagnostics(broken.uri).find((d) => codeOf(d) === 2322))
    assert(found.range.start.line === 1, `line ${found.range.start.line}`)
  })

  await check("diagnostics: EK1003 appears on wait.effeck with source EK", async () => {
    const wait = await open("src/wait.effeck")
    const found = await waitFor("EK1003", () => vscode.languages.getDiagnostics(wait.uri).find((d) => codeOf(d) === 1003))
    assert(found.source === "ES", `source ${found.source}`)
  })

  await check("hover: an .effeck function shows its inferred contract", async () => {
    const shown = await hoverText(main, "lookup(id")
    assert(/function lookup\(id: number\): (Effect\.)?Effect<string, NotFound, never>/.test(shown), `hover ${JSON.stringify(shown)}`)
  })

  await check("hover: a call that runs shows its success value", async () => {
    const shown = await hoverText(main, "lookup(1)")
    assert(shown.includes("Runs here and returns `string`"), `hover ${JSON.stringify(shown)}`)
  })

  await check("definition: a .ts function called from .effeck goes to users.ts", async () => {
    const locations = await vscode.commands.executeCommand("vscode.executeDefinitionProvider", main.uri, positionOf(main, "findName(id)"))
    const uris = locations.map((l) => (l.targetUri ?? l.uri).fsPath)
    assert(uris.some((uri) => uri.endsWith(path.join("src", "users.ts"))), `went to ${JSON.stringify(uris)}`)
  })

  await check("completion: prelude names and a service's members in services.effeck", async () => {
    const services = await open("src/services.effeck")
    const labels = async (needle, offset) => {
      const list = await vscode.commands.executeCommand("vscode.executeCompletionItemProvider", services.uri, positionOf(services, needle, offset))
      return list.items.map((item) => (typeof item.label === "string" ? item.label : item.label.label))
    }
    const scope = await labels("Users.find", 0)
    for (const label of ["TaggedError", "wire", "Users", "double"]) assert(scope.includes(label), `no ${label} in completions`)
    assert((await labels("Users.find", 6)).includes("find"), "no find after Users.")
  })

  await check("hover: a plain function shows the signature Effeck calls", async () => {
    const services = await open("src/services.effeck")
    const shown = await hoverText(services, "double(n")
    assert(shown.includes("function double(n: number): number"), `hover ${JSON.stringify(shown)}`)
  })

  await check("diagnostics: main.effeck has none", () => {
    const items = vscode.languages.getDiagnostics(main.uri)
    assert(items.length === 0, JSON.stringify(items.map((d) => d.message)))
  })

  if (failures > 0) throw new Error(`${failures} check(s) failed`)
}

module.exports = { run }
