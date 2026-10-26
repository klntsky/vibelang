// Drives the stock TypeScript 7 language server over stdio and checks that it
// answers hovers and diagnostics at positions in a .vibe file through the
// vibelang-ts-mapper content mapper. No editor and no dependencies.
//
//   node lsp-hover.mjs <tsc> <project dir>
//
// Exits 0 when every expectation holds, 1 otherwise.

import { spawn } from "node:child_process"
import { readFileSync } from "node:fs"
import { join } from "node:path"
import { pathToFileURL } from "node:url"

const [tsc, projectDir] = process.argv.slice(2)
if (!tsc || !projectDir) {
  console.error("usage: node lsp-hover.mjs <tsc> <project dir>")
  process.exit(2)
}

const greetPath = join(projectDir, "src", "greet.vibe")
const greetText = readFileSync(greetPath, "utf8")
const greetUri = pathToFileURL(greetPath).href

// Each expectation names a token in greet.vibe by the text around it, and
// the text the hover must contain.
const hovers = [
  // Inferred: greet's Effect type comes from its projected generator.
  { at: "export function greet", token: "greet", expect: "function greet(name: string): Effect.Effect<string, EmptyName, never>" },
  // A call runs: loud is shout's success type, not an Effect.
  { at: "const loud", token: "loud", expect: "const loud: string" },
  // Annotated with the contract Effect<string>.
  { at: "function shout", token: "shout", expect: "function shout(text: string): Effect<string>" },
]

// position returns the LSP position of token inside the first occurrence of at.
function position(at, token) {
  const offset = greetText.indexOf(at)
  if (offset < 0) throw new Error(`fixture has no ${JSON.stringify(at)}`)
  const target = offset + at.indexOf(token)
  const before = greetText.slice(0, target).split("\n")
  return { line: before.length - 1, character: before.at(-1).length }
}

const server = spawn(tsc, ["--lsp", "--stdio"], { cwd: projectDir, stdio: ["pipe", "pipe", "pipe"] })
// The server logs to stderr, including "context canceled" on a clean exit;
// show the log only when a check fails.
let serverLog = ""
server.stderr.on("data", (chunk) => (serverLog += chunk))
let buffer = Buffer.alloc(0)
let nextId = 1
const pending = new Map()

// send writes one JSON-RPC message with LSP framing.
function send(message) {
  const body = Buffer.from(JSON.stringify({ jsonrpc: "2.0", ...message }), "utf8")
  server.stdin.write(`Content-Length: ${body.length}\r\n\r\n`)
  server.stdin.write(body)
}

// request sends a request and resolves with its result.
function request(method, params) {
  const id = nextId++
  send({ id, method, params })
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject, method }))
}

// answer replies to a request the server sent: registrations, configuration
// and progress all get the neutral answer.
function answer(message) {
  if (message.method === "workspace/configuration") {
    send({ id: message.id, result: message.params.items.map(() => null) })
  } else {
    send({ id: message.id, result: null })
  }
}

server.stdout.on("data", (chunk) => {
  buffer = Buffer.concat([buffer, chunk])
  for (;;) {
    const headerEnd = buffer.indexOf("\r\n\r\n")
    if (headerEnd < 0) return
    const length = Number(/Content-Length: (\d+)/i.exec(buffer.subarray(0, headerEnd).toString())[1])
    if (buffer.length < headerEnd + 4 + length) return
    const message = JSON.parse(buffer.subarray(headerEnd + 4, headerEnd + 4 + length).toString("utf8"))
    buffer = buffer.subarray(headerEnd + 4 + length)
    if (message.method && message.id !== undefined) {
      answer(message)
    } else if (message.id !== undefined && pending.has(message.id)) {
      const { resolve, reject, method } = pending.get(message.id)
      pending.delete(message.id)
      if (message.error) reject(new Error(`${method}: ${message.error.message}`))
      else resolve(message.result)
    }
  }
})

const timeout = setTimeout(() => {
  console.error(`lsp-hover: timed out after 60s; server log:\n${serverLog}`)
  process.exit(1)
}, 60_000)

let failures = 0
try {
  await request("initialize", {
    processId: process.pid,
    rootUri: pathToFileURL(projectDir).href,
    workspaceFolders: [{ uri: pathToFileURL(projectDir).href, name: "fixture" }],
    capabilities: {
      textDocument: {
        synchronization: { dynamicRegistration: true },
        hover: { dynamicRegistration: true, contentFormat: ["plaintext", "markdown"] },
        diagnostic: { dynamicRegistration: true },
      },
      workspace: { configuration: true },
    },
    initializationOptions: { runExternalCode: true },
  })
  send({ method: "initialized", params: {} })
  send({ method: "textDocument/didOpen", params: { textDocument: { uri: greetUri, languageId: "vibe", version: 1, text: greetText } } })

  for (const hover of hovers) {
    const at = position(hover.at, hover.token)
    const result = await request("textDocument/hover", { textDocument: { uri: greetUri }, position: at })
    const contents = result?.contents?.value ?? JSON.stringify(result)
    const ok = contents.includes(hover.expect)
    if (!ok) failures++
    console.log(`${ok ? "ok  " : "FAIL"} hover greet.vibe:${at.line + 1}:${at.character + 1} ${hover.token}`)
    console.log(`     ${contents.replace(/\n/g, "\n     ")}`)
  }

  const report = await request("textDocument/diagnostic", { textDocument: { uri: greetUri } })
  const diagnostics = (report?.items ?? []).filter((d) => d.severity === 1).map((d) => `greet.vibe:${d.range.start.line + 1}:${d.range.start.character + 1} TS${d.code}: ${d.message}`)
  const expected = "greet.vibe:11:9 TS2322: Type 'string' is not assignable to type 'number'."
  const ok = diagnostics.length === 1 && diagnostics[0] === expected
  if (!ok) failures++
  console.log(`${ok ? "ok  " : "FAIL"} diagnostics greet.vibe`)
  for (const line of diagnostics) console.log(`     ${line}`)

  await request("shutdown")
  send({ method: "exit" })
} catch (error) {
  failures++
  console.error("lsp-hover:", error.message)
  server.kill()
}
clearTimeout(timeout)
if (failures > 0) console.error(`lsp-hover: ${failures} check(s) failed; server log:\n${serverLog}`)
process.exit(failures === 0 ? 0 : 1)
