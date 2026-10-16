#!/usr/bin/env node
// Headless language server test. Starts `esc --lsp --stdio` on
// effectscript/lsp-fixture, a project with .es and .ts files, and checks
// what an editor shows: pulled diagnostics, hovers with inferred contracts,
// definitions, references, rename, completions, and file watching across
// .es and .ts files.
//
// Usage: node effectscript/lsp-smoke.mjs [esc]
//
// esc defaults to built/local/esc. effectscript/lsp-smoke.sh builds it and
// links the fixture's node_modules first. Each check prints "ok" or "FAIL";
// the exit status is 1 when any check fails.
import { spawn } from "node:child_process"
import { readFileSync, rmSync, writeFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"

const here = path.dirname(fileURLToPath(import.meta.url))
const fixture = path.join(here, "lsp-fixture")
const server = path.resolve(process.argv[2] ?? path.join(here, "..", "built", "local", "esc"))
const requestTimeoutMs = 60_000

// A JSON-RPC connection to a language server over stdio.
class Connection {
  constructor(command, args, cwd) {
    this.process = spawn(command, args, { cwd, stdio: ["pipe", "pipe", "pipe"] })
    this.buffer = Buffer.alloc(0)
    this.nextId = 1
    this.pending = new Map()
    this.registrations = []
    this.log = []
    this.exited = new Promise((resolve) => this.process.on("exit", (code, signal) => resolve({ code, signal })))
    this.process.on("error", (error) => {
      for (const { reject } of this.pending.values()) reject(error)
    })
    this.process.stderr.on("data", (chunk) => this.remember(`stderr: ${chunk}`))
    this.process.stdout.on("data", (chunk) => this.receive(chunk))
  }

  remember(line) {
    this.log.push(line.trimEnd())
    if (this.log.length > 40) this.log.shift()
  }

  receive(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk])
    for (;;) {
      const end = this.buffer.indexOf("\r\n\r\n")
      if (end < 0) return
      const length = Number(/Content-Length: *(\d+)/i.exec(this.buffer.subarray(0, end).toString())?.[1])
      if (this.buffer.length < end + 4 + length) return
      const message = JSON.parse(this.buffer.subarray(end + 4, end + 4 + length).toString())
      this.buffer = this.buffer.subarray(end + 4 + length)
      this.dispatch(message)
    }
  }

  dispatch(message) {
    if (message.method === undefined) {
      const waiter = this.pending.get(message.id)
      this.pending.delete(message.id)
      if (message.error) waiter?.reject(new Error(`${waiter.method}: ${message.error.message}`))
      else waiter?.resolve(message.result)
      return
    }
    if (message.method === "window/logMessage") this.remember(message.params.message)
    if (message.id === undefined) return
    // A request from the server. Answer the ones a minimal editor answers.
    let result = null
    if (message.method === "workspace/configuration") result = message.params.items.map(() => null)
    if (message.method === "client/registerCapability") this.registrations.push(...message.params.registrations)
    this.send({ jsonrpc: "2.0", id: message.id, result })
  }

  send(message) {
    const body = Buffer.from(JSON.stringify(message))
    this.process.stdin.write(`Content-Length: ${body.length}\r\n\r\n`)
    this.process.stdin.write(body)
  }

  request(method, params) {
    const id = this.nextId++
    this.send({ jsonrpc: "2.0", id, method, params })
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`${method}: no response in ${requestTimeoutMs} ms`)), requestTimeoutMs)
      this.pending.set(id, {
        method,
        resolve: (value) => (clearTimeout(timer), resolve(value)),
        reject: (error) => (clearTimeout(timer), reject(error))
      })
    })
  }

  notify(method, params) {
    this.send({ jsonrpc: "2.0", method, params })
  }

  // close asks the server to shut down and exit, and returns how it exited.
  async close() {
    try {
      await this.request("shutdown")
      this.notify("exit")
    } finally {
      this.process.stdin.end()
      const timer = setTimeout(() => this.process.kill("SIGKILL"), 5_000)
      const exit = await this.exited
      clearTimeout(timer)
      return exit
    }
  }
}

const fileUri = (relative) => pathToFileURL(path.join(fixture, relative)).href
const texts = new Map()
const text = (relative) => {
  if (!texts.has(relative)) texts.set(relative, readFileSync(path.join(fixture, relative), "utf8"))
  return texts.get(relative)
}

// at returns the LSP position of the nth occurrence (from 0) of needle in a
// fixture file, moved right by offset characters.
function at(relative, needle, { nth = 0, offset = 0 } = {}) {
  const source = text(relative)
  let index = -1
  for (let i = 0; i <= nth; i++) {
    index = source.indexOf(needle, index + 1)
    if (index < 0) throw new Error(`${relative} has no occurrence ${nth} of ${JSON.stringify(needle)}`)
  }
  index += offset
  const before = source.slice(0, index)
  return { line: before.split("\n").length - 1, character: index - before.lastIndexOf("\n") - 1 }
}

const position = (relative, needle, options) => ({ textDocument: { uri: fileUri(relative) }, position: at(relative, needle, options) })

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

function lineOf(relative, needle) {
  return at(relative, needle).line
}

// globMatches reports whether an LSP glob pattern (with **, *, ?, and {a,b})
// matches an absolute path.
function globMatches(glob, file) {
  let pattern = ""
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i]
    if (c === "*" && glob[i + 1] === "*") {
      pattern += glob[i + 2] === "/" ? "(?:.*/)?" : ".*"
      i += glob[i + 2] === "/" ? 2 : 1
    } else if (c === "*") pattern += "[^/]*"
    else if (c === "?") pattern += "[^/]"
    else if (c === "{") pattern += "(?:"
    else if (c === "}") pattern += ")"
    else if (c === ",") pattern += "|"
    else pattern += c.replace(/[.+^$()|[\]\\]/g, "\\$&")
  }
  return new RegExp(`^${pattern}$`).test(file)
}

function watcherGlob(watcher) {
  const pattern = watcher.globPattern
  if (typeof pattern === "string") return pattern.startsWith("file://") ? fileURLToPath(pattern) : pattern
  const base = typeof pattern.baseUri === "string" ? pattern.baseUri : pattern.baseUri.uri
  return `${fileURLToPath(base)}/${pattern.pattern}`
}

function hoverText(hover) {
  const contents = hover?.contents
  if (contents === undefined) return ""
  if (typeof contents === "string") return contents
  if (Array.isArray(contents)) return contents.map((c) => (typeof c === "string" ? c : c.value)).join("\n")
  return contents.value
}

function workspaceEditFiles(edit) {
  const files = new Map()
  for (const [uri, edits] of Object.entries(edit?.changes ?? {})) files.set(uri, edits)
  for (const change of edit?.documentChanges ?? []) if (change.textDocument) files.set(change.textDocument.uri, change.edits)
  return files
}

let failures = 0
async function check(name, body) {
  try {
    await body()
    console.log(`ok   ${name}`)
  } catch (error) {
    failures++
    console.log(`FAIL ${name}: ${error.message}`)
  }
}

console.log(`server: ${server} --lsp --stdio`)
const connection = new Connection(server, ["--lsp", "--stdio"], fixture)
const extraFile = path.join(fixture, "src", "extra.es")
try {
  const rootUri = pathToFileURL(fixture).href
  const initialized = await connection.request("initialize", {
    processId: process.pid,
    rootUri,
    workspaceFolders: [{ uri: rootUri, name: "lsp-fixture" }],
    capabilities: {
      general: { positionEncodings: ["utf-16"] },
      textDocument: {
        synchronization: { didSave: true },
        hover: { contentFormat: ["markdown", "plaintext"] },
        diagnostic: { dynamicRegistration: false },
        definition: { linkSupport: false },
        references: {},
        rename: { prepareSupport: true },
        completion: { completionItem: { snippetSupport: false } }
      },
      workspace: {
        workspaceFolders: true,
        configuration: true,
        workspaceEdit: { documentChanges: true },
        didChangeWatchedFiles: { dynamicRegistration: true, relativePatternSupport: true },
        symbol: {}
      }
    }
  })
  connection.notify("initialized", {})

  await check("initialize: the server pulls diagnostics and serves hover, definition, references, rename, and completion", () => {
    const caps = initialized.capabilities
    assert(caps.diagnosticProvider, "no diagnosticProvider")
    for (const provider of ["hoverProvider", "definitionProvider", "referencesProvider", "renameProvider", "completionProvider"]) {
      assert(caps[provider], `no ${provider}`)
    }
  })

  await check("initialize: renaming an .es file asks the server to update its importers", () => {
    const filters = initialized.capabilities.workspace?.fileOperations?.willRename?.filters ?? []
    assert(filters.some((f) => globMatches(f.pattern.glob, extraFile)), `filters ${JSON.stringify(filters)}`)
  })

  const open = (relative, languageId) =>
    connection.notify("textDocument/didOpen", {
      textDocument: { uri: fileUri(relative), languageId, version: 1, text: text(relative) }
    })
  open("src/main.es", "effectscript")
  open("src/users.ts", "typescript")
  open("src/broken.es", "typescript")

  const diagnostics = async (relative) =>
    (await connection.request("textDocument/diagnostic", { textDocument: { uri: fileUri(relative) } })).items

  await check("diagnostics: main.es, opened as effectscript, checks clean", async () => {
    const items = await diagnostics("src/main.es")
    assert(items.length === 0, `expected none, got ${JSON.stringify(items.map((d) => d.message))}`)
  })

  await check("diagnostics: users.ts checks clean", async () => {
    const items = await diagnostics("src/users.ts")
    assert(items.length === 0, `expected none, got ${JSON.stringify(items.map((d) => d.message))}`)
  })

  await check("diagnostics: broken.es, opened as typescript, reports TS2322 at its .es span", async () => {
    const items = await diagnostics("src/broken.es")
    assert(items.length === 1, `expected one diagnostic, got ${JSON.stringify(items.map((d) => d.message))}`)
    const [item] = items
    assert(item.code === 2322, `expected code 2322, got ${JSON.stringify(item.code)}`)
    const start = at("src/broken.es", "count")
    assert(item.range.start.line === start.line && item.range.start.character === start.character, `range ${JSON.stringify(item.range)}`)
  })

  await check("diagnostics: wait.es reports ES1003 with source ES at the async keyword", async () => {
    open("src/wait.es", "effectscript")
    const items = await diagnostics("src/wait.es")
    assert(items.length === 1, `expected one diagnostic, got ${JSON.stringify(items.map((d) => d.message))}`)
    const [item] = items
    assert(item.source === "ES" && item.code === 1003, `expected ES 1003, got ${item.source} ${JSON.stringify(item.code)}`)
    const start = at("src/wait.es", "async function")
    assert(item.range.start.line === start.line && item.range.start.character === start.character, `range ${JSON.stringify(item.range)}`)
  })

  await check("diagnostics: an edit to main.es is checked with EffectScript types", async () => {
    // name is the success value of lookup(1), a string, so toFixed does not exist.
    const broken = text("src/main.es").replace("name.toUpperCase()", "name.toFixed()")
    const uri = fileUri("src/main.es")
    connection.notify("textDocument/didChange", { textDocument: { uri, version: 2 }, contentChanges: [{ text: broken }] })
    const items = await diagnostics("src/main.es")
    connection.notify("textDocument/didChange", { textDocument: { uri, version: 3 }, contentChanges: [{ text: text("src/main.es") }] })
    assert(items.some((d) => d.code === 2339 || d.code === 2551), `expected TS2339 on toFixed, got ${JSON.stringify(items.map((d) => d.message))}`)
    assert((await diagnostics("src/main.es")).length === 0, "the reverted file still has diagnostics")
  })

  const hover = async (relative, needle, options) => hoverText(await connection.request("textDocument/hover", position(relative, needle, options)))
  const contract = (signature) => new RegExp(signature.replace(/[()]/g, "\\$&").replace("Effect<", "(?:Effect\\.)?Effect<"))

  await check("hover: an .es function shows its inferred contract", async () => {
    const shown = await hover("src/main.es", "lookup(id")
    assert(contract("function lookup(id: number): Effect<string, NotFound, never>").test(shown), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: an exported .es function shows the contract its callee's failure flows into", async () => {
    const shown = await hover("src/main.es", "main()", { offset: 1 })
    assert(contract("function main(): Effect<void, NotFound, never>").test(shown), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a call that runs shows the callee's contract and the success value", async () => {
    const shown = await hover("src/main.es", "lookup(1)")
    assert(contract("function lookup(id: number): Effect<string, NotFound, never>").test(shown), `got ${JSON.stringify(shown)}`)
    assert(/Runs here and returns `string`/.test(shown), `no success value in ${JSON.stringify(shown)}`)
  })

  await check("hover: a TypeScript call that runs shows its success value", async () => {
    const shown = await hover("src/main.es", "findName(id)")
    assert(/Runs here and returns `string`/.test(shown), `no success value in ${JSON.stringify(shown)}`)
  })

  await check("hover: a variable holding a call's result has the success type", async () => {
    const shown = await hover("src/main.es", "name = ")
    assert(shown.includes("const name: string"), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a call to Effect.fail in an .es body runs and fails", async () => {
    const shown = await hover("src/main.es", "Effect.fail", { offset: 7 })
    assert(/Runs here and returns `never`/.test(shown), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a call in a .ts file does not run", async () => {
    const shown = await hover("src/users.ts", "Effect.succeed", { offset: 7 })
    assert(shown.includes("succeed") && !shown.includes("Runs here"), `got ${JSON.stringify(shown)}`)
  })

  const definition = async (relative, needle, options) => {
    const result = await connection.request("textDocument/definition", position(relative, needle, options))
    return Array.isArray(result) ? result : result ? [result] : []
  }

  await check("definition: a .ts function called from .es goes to users.ts", async () => {
    const [location, ...rest] = await definition("src/main.es", "findName(id)")
    assert(location && rest.length === 0, `expected one location, got ${JSON.stringify([location, ...rest])}`)
    assert(location.uri === fileUri("src/users.ts"), `went to ${location.uri}`)
    assert(location.range.start.line === lineOf("src/users.ts", "export function findName"), `line ${location.range.start.line}`)
  })

  await check("definition: an .es call goes to the .es declaration", async () => {
    const [location] = await definition("src/main.es", "lookup(1)")
    assert(location?.uri === fileUri("src/main.es"), `went to ${location?.uri}`)
    assert(location.range.start.line === lineOf("src/main.es", "function lookup"), `line ${location.range.start.line}`)
  })

  await check("references: a .ts function's references include its .es callers", async () => {
    const locations = await connection.request("textDocument/references", {
      ...position("src/users.ts", "findName"),
      context: { includeDeclaration: true }
    })
    const lines = locations.filter((l) => l.uri === fileUri("src/main.es")).map((l) => l.range.start.line)
    assert(lines.includes(lineOf("src/main.es", "findName(id)")), `main.es lines ${JSON.stringify(lines)}`)
    assert(locations.some((l) => l.uri === fileUri("src/users.ts")), "no users.ts location")
  })

  await check("rename: renaming a .ts function edits its .es callers", async () => {
    const edit = await connection.request("textDocument/rename", { ...position("src/users.ts", "findName"), newName: "nameOf" })
    const files = workspaceEditFiles(edit)
    const lines = (relative) => (files.get(fileUri(relative)) ?? []).map((e) => e.range.start.line).sort((a, b) => a - b)
    const expected = [lineOf("src/main.es", "import { findName"), lineOf("src/main.es", "findName(id)")]
    assert(JSON.stringify(lines("src/main.es")) === JSON.stringify(expected), `main.es edit lines ${JSON.stringify(lines("src/main.es"))}, want ${JSON.stringify(expected)}`)
    assert(lines("src/users.ts").length === 1, `users.ts edits: ${JSON.stringify(files.get(fileUri("src/users.ts")))}`)
  })

  await check("rename: renaming an .es function edits its declaration and calls", async () => {
    const edit = await connection.request("textDocument/rename", { ...position("src/main.es", "lookup(1)"), newName: "find" })
    const lines = (workspaceEditFiles(edit).get(fileUri("src/main.es")) ?? []).map((e) => e.range.start.line).sort((a, b) => a - b)
    const expected = [lineOf("src/main.es", "function lookup"), lineOf("src/main.es", "lookup(1)")]
    assert(JSON.stringify(lines) === JSON.stringify(expected), `edit lines ${JSON.stringify(lines)}, want ${JSON.stringify(expected)}`)
  })

  const completionLabels = async (relative, needle, options) => {
    const result = await connection.request("textDocument/completion", position(relative, needle, options))
    return (Array.isArray(result) ? result : result.items).map((item) => item.label)
  }

  await check("completion: members of an imported Effect module in .es", async () => {
    const labels = await completionLabels("src/main.es", "Console.log", { offset: 8 })
    assert(labels.includes("log") && labels.includes("error"), `labels ${JSON.stringify(labels.slice(0, 20))}`)
  })

  await check("completion: .es declarations in scope", async () => {
    const labels = await completionLabels("src/main.es", "name.toUpperCase", { offset: 0 })
    for (const label of ["lookup", "name", "NotFound", "findName"]) assert(labels.includes(label), `no ${label} in completions`)
  })

  open("src/services.es", "effectscript")
  open("src/report.ts", "typescript")

  await check("diagnostics: services.es, with the prelude and a service, checks clean", async () => {
    const items = await diagnostics("src/services.es")
    assert(items.length === 0, `expected none, got ${JSON.stringify(items.map((d) => d.message))}`)
  })

  await check("diagnostics: report.ts, which imports an .es function, checks clean", async () => {
    const items = await diagnostics("src/report.ts")
    assert(items.length === 0, `expected none, got ${JSON.stringify(items.map((d) => d.message))}`)
  })

  // An entry is in scope when it is not an auto-import, which names its module.
  const inScope = async (relative, needle, options) => {
    const result = await connection.request("textDocument/completion", position(relative, needle, options))
    const items = Array.isArray(result) ? result : result.items
    return (label) => items.some((item) => item.label === label && item.labelDetails?.description === undefined)
  }

  await check("completion: prelude names are in scope in an .es file", async () => {
    const has = await inScope("src/services.es", "Users.find", { offset: 0 })
    for (const label of ["Effect", "Layer", "Schema", "Scope", "TaggedError", "pipe", "wire", "fork", "Users", "double"]) {
      assert(has(label), `no ${label} in scope`)
    }
  })

  await check("completion: resolving a prelude entry gives its declaration", async () => {
    const result = await connection.request("textDocument/completion", position("src/services.es", "Users.find", { offset: 0 }))
    const item = (Array.isArray(result) ? result : result.items).find((i) => i.label === "wire" && i.labelDetails?.description === undefined)
    assert(item, "no wire entry")
    const resolved = await connection.request("completionItem/resolve", item)
    assert(/wire/.test(resolved.detail ?? ""), `detail ${JSON.stringify(resolved.detail)}`)
  })

  await check("completion: prelude names are in scope in an .es file that uses none yet", async () => {
    const has = await inScope("src/main.es", "name.toUpperCase", { offset: 0 })
    for (const label of ["Effect", "Layer", "TaggedError", "pipe", "wire"]) assert(has(label), `no ${label} in scope`)
  })

  await check("completion: prelude names are not in scope in a .ts file", async () => {
    const has = await inScope("src/users.ts", "names[id]", { offset: 0 })
    assert(has("names"), "no names in scope")
    for (const label of ["TaggedError", "wire", "fork", "Layer"]) assert(!has(label), `${label} in scope`)
  })

  await check("completion: a service's members after Users. in an .es function", async () => {
    const labels = await completionLabels("src/services.es", "Users.find", { offset: 6 })
    assert(labels.includes("find"), `labels ${JSON.stringify(labels.slice(0, 20))}`)
    for (const label of ["pipe", "of", "key"]) assert(!labels.includes(label), `the key's own member ${label} in completions`)
  })

  await check("hover: a plain .es function shows the signature EffectScript calls", async () => {
    const shown = await hover("src/services.es", "double(n")
    assert(shown.includes("function double(n: number): number"), `got ${JSON.stringify(shown)}`)
    assert(/TypeScript callers see `(Effect\.)?Effect<number, never, never>`/.test(shown), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a call to a plain .es function shows its plain signature and does not run", async () => {
    const shown = await hover("src/services.es", "double(id)")
    assert(shown.includes("function double(n: number): number") && !shown.includes("Runs here"), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a .ts file sees a plain .es function as an Effect", async () => {
    const shown = await hover("src/report.ts", "double(2)")
    assert(/Effect<number, never, never>/.test(shown) && !shown.includes("Plain JavaScript"), `got ${JSON.stringify(shown)}`)
  })

  await check("hover: a service member call runs and shows its success value", async () => {
    const shown = await hover("src/services.es", "find(id)")
    assert(/Runs here and returns `string`/.test(shown), `got ${JSON.stringify(shown)}`)
  })

  await check("watching: the server registers a watcher that covers new .es files", () => {
    const watchers = connection.registrations
      .filter((r) => r.method === "workspace/didChangeWatchedFiles")
      .flatMap((r) => r.registerOptions.watchers)
    assert(watchers.some((w) => globMatches(watcherGlob(w), extraFile)), `globs ${JSON.stringify(watchers.map(watcherGlob))}`)
  })

  await check("watching: a new .es file on disk joins the project", async () => {
    writeFileSync(extraFile, "export function extraHelper(n: number) {\n  return n + 1\n}\n")
    connection.notify("workspace/didChangeWatchedFiles", { changes: [{ uri: pathToFileURL(extraFile).href, type: 1 }] })
    const deadline = Date.now() + 20_000
    for (;;) {
      const symbols = (await connection.request("workspace/symbol", { query: "extraHelper" })) ?? []
      if (symbols.some((s) => s.location?.uri === pathToFileURL(extraFile).href)) break
      assert(Date.now() < deadline, `workspace/symbol never found extraHelper: ${JSON.stringify(symbols)}`)
      await new Promise((resolve) => setTimeout(resolve, 250))
    }
  })
} catch (error) {
  failures++
  console.log(`FAIL ${error.message}`)
} finally {
  rmSync(extraFile, { force: true })
  await check("shutdown: the server exits with status 0 after shutdown and exit", async () => {
    const { code, signal } = await connection.close()
    assert(code === 0, `exit status ${code}, signal ${signal}`)
  })
}

if (failures > 0) {
  console.log(`\n${failures} check(s) failed. Last server log lines:`)
  for (const line of connection.log) console.log(`  ${line}`)
  process.exit(1)
}
