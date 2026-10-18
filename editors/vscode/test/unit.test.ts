// Editor lifecycle contracts with a simulated VS Code API. The packaged
// extension's real-editor tests remain in run.mjs and harness/index.js.
import { afterEach, beforeEach, expect, mock, test } from "bun:test"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"

let settings: Record<string, unknown>
let registered: Map<string, () => unknown>
let status: any
let logs: string[]
let clients: FakeClient[]
let builtin: boolean
let errorChoice: string | undefined
let infoChoice: string | undefined
let failStart: boolean
let failStop: boolean
let onConfiguration: (event: any) => void
let onFolders: () => void
let executed: unknown[][]
let updates: unknown[][]
let scratch: string
let oldPath: string | undefined
class FakeClient {
  handler: (event: any) => void = () => {}
  initializeResult = { serverInfo: { version: "7.1-test" } }
  stopped = false
  constructor(readonly id: string, readonly name: string, readonly server: any, readonly options: any) { clients.push(this) }
  onDidChangeState(handler: any) { this.handler = handler }
  async start() { if (failStart) throw new Error("compiler failed"); this.handler({ newState: 2 }) }
  async stop() { this.stopped = true; if (failStop) throw new Error("stop failed") }
}
const workspace: any = {
  workspaceFolders: [], textDocuments: [],
  getConfiguration(section: string) {
    return {
      get(key: string, fallback: unknown) { return settings[`${section}.${key}`] ?? fallback },
      async update(...args: unknown[]) { updates.push([section, ...args]) }
    }
  },
  onDidChangeConfiguration(handler: any) { onConfiguration = handler; return { dispose() {} } },
  onDidChangeWorkspaceFolders(handler: any) { onFolders = handler; return { dispose() {} } }
}
const window: any = {
  activeTextEditor: undefined,
  createOutputChannel: () => ({ info: (s: string) => logs.push(s), warn: (s: string) => logs.push(s), error: (s: string) => logs.push(s), show: () => executed.push(["show"]), dispose() {} }),
  async showErrorMessage(...args: unknown[]) { executed.push(["error", ...args]); return errorChoice },
  async showInformationMessage(...args: unknown[]) { executed.push(["info", ...args]); return infoChoice }
}
const env = { appName: "Visual Studio Code" }
mock.module("vscode", () => ({ workspace, window, env,
  commands: {
    registerCommand(id: string, fn: () => unknown) { registered.set(id, fn); return { dispose() {} } },
    async executeCommand(...args: unknown[]) { executed.push(args) }
  },
  languages: { createLanguageStatusItem: () => (status = { dispose() {} }) },
  LanguageStatusSeverity: { Error: 2, Warning: 1, Information: 0 }, ConfigurationTarget: { Workspace: 2 },
  extensions: { getExtension: () => builtin ? {} : undefined }
}))
mock.module("vscode-languageclient/node", () => ({ LanguageClient: FakeClient, State: { Running: 2, Stopped: 1 }, TransportKind: { stdio: 0 } }))
const { activate, deactivate } = await import("../src/extension")

beforeEach(() => {
  scratch = mkdtempSync(join(tmpdir(), "vibelang-editor-unit-"))
  oldPath = process.env.PATH
  settings = { "vibelang.server.path": "/configured/vibelang" }
  registered = new Map(); clients = []; logs = []; executed = []; updates = []
  builtin = true; failStart = false; failStop = false; errorChoice = undefined; infoChoice = undefined
  workspace.workspaceFolders = [{ uri: { fsPath: scratch } }]; workspace.textDocuments = []
  window.activeTextEditor = undefined; env.appName = "Visual Studio Code"
})
afterEach(async () => { await deactivate(); process.env.PATH = oldPath; rmSync(scratch, { recursive: true, force: true }) })
const start = () => activate({ subscriptions: [] } as any)
const command = (id: string) => registered.get(`vibelang.${id}`)!()
function executable(file: string) { mkdirSync(dirname(file), { recursive: true }); writeFileSync(file, "fixture", { mode: 0o755 }); return file }

test("starts the configured server with stdio and owns only VibeLang by default", async () => {
  await start()
  expect(clients).toHaveLength(1)
  expect(clients[0].server).toEqual({ command: "/configured/vibelang", args: ["--lsp"], transport: 0, options: { cwd: scratch } })
  expect(clients[0].options.documentSelector).toEqual([{ scheme: "file", language: "vibe" }, { scheme: "untitled", language: "vibe" }])
  expect(status.text).toBe("vibelang")
  expect(status.detail).toContain("TypeScript 7.1-test")
  await deactivate()
  expect(clients[0].stopped).toBe(true)
})

for (const [mode, hasBuiltin, appName, useTsgo, serveTS] of [
  ["on", true, "Visual Studio Code", false, true],
  ["off", false, "Cursor", true, false],
  ["auto", false, "Visual Studio Code", false, true],
  ["auto", true, "Cursor", true, true],
  ["auto", true, "Visual Studio Code", true, false],
  ["auto", true, "Cursor", false, false]
] as const) {
  test(`TypeScript ownership: ${mode}, builtin ${hasBuiltin}, ${appName}, tsgo ${useTsgo}`, async () => {
    settings["vibelang.server.typeScriptFiles"] = mode
    settings["typescript.experimental.useTsgo"] = useTsgo
    builtin = hasBuiltin; env.appName = appName
    await start()
    const languages = [...new Set(clients[0].options.documentSelector.map((d: any) => d.language))]
    expect(languages).toEqual(serveTS ? ["vibe", "typescript", "typescriptreact", "javascript", "javascriptreact"] : ["vibe"])
  })
}

test("resolves a configured relative path against the first workspace folder", async () => {
  settings["vibelang.server.path"] = "  tools/compiler  "
  await start()
  expect(clients[0].server.command).toBe(join(scratch, "tools/compiler"))
})

test("expands a configured home path and works without an open folder", async () => {
  const { homedir } = await import("node:os")
  settings["vibelang.server.path"] = "~/tools/vibelang"
  workspace.workspaceFolders = undefined
  await start()
  expect(clients[0].server.command).toBe(join(homedir(), "tools/vibelang"))
  expect(clients[0].server.options.cwd).toBeUndefined()
})

test("the active source uses its nearest compiler before other documents or workspace folders", async () => {
  settings["vibelang.server.path"] = ""
  executable(join(scratch, "node_modules/.bin/vibelang"))
  const nested = executable(join(scratch, "nested/node_modules/.bin/vibelang"))
  window.activeTextEditor = { document: { languageId: "vibe", uri: { scheme: "file", fsPath: join(scratch, "nested/src/main.vibe") } } }
  await start()
  expect(clients[0].server.command).toBe(nested)
})

test("open VibeLang documents find their compiler even without workspace folders", async () => {
  settings["vibelang.server.path"] = ""
  workspace.workspaceFolders = []
  const local = executable(join(scratch, "node_modules/.bin/vibelang"))
  workspace.textDocuments = [{ languageId: "vibe", uri: { scheme: "file", fsPath: join(scratch, "src/main.vibe") } }]
  await start()
  expect(clients[0].server.command).toBe(local)
})

test("PATH fallback ignores empty entries, nonexecutable files and directories", async () => {
  settings["vibelang.server.path"] = ""
  workspace.workspaceFolders = []
  window.activeTextEditor = { document: { languageId: "typescript", uri: { scheme: "file", fsPath: join(scratch, "ignored.ts") } } }
  workspace.textDocuments = [{ languageId: "vibe", uri: { scheme: "untitled", fsPath: "untitled:Untitled-1" } }]
  mkdirSync(join(scratch, "directory/vibelang"), { recursive: true })
  executable(join(scratch, "no-permission/vibelang"))
  const { chmodSync } = await import("node:fs")
  chmodSync(join(scratch, "no-permission/vibelang"), 0o644)
  const compiler = executable(join(scratch, "bin/vibelang"))
  process.env.PATH = `:${join(scratch, "directory")}:${join(scratch, "no-permission")}:${join(scratch, "bin")}`
  await start()
  expect(clients[0].server.command).toBe(compiler)
})

for (const choice of [undefined, "Open Settings", "Show Output"]) {
  test(`missing compiler reports recovery actions: ${choice ?? "dismiss"}`, async () => {
    settings["vibelang.server.path"] = ""
    workspace.workspaceFolders = []; process.env.PATH = ""
    errorChoice = choice
    await start()
    expect(clients).toHaveLength(0)
    expect(status.text).toBe("vibelang not found")
    expect(status.severity).toBe(2)
    expect(logs.join("\n")).toContain("Cannot find vibelang")
    if (choice === "Open Settings") expect(executed).toContainEqual(["workbench.action.openSettings", "vibelang.server.path"])
    if (choice === "Show Output") expect(executed).toContainEqual(["show"])
  })
}

test("configuration middleware preserves settings except diagnostic validation switches", async () => {
  await start()
  const configure = clients[0].options.middleware.workspace.configuration
  const value = { validate: { enable: false }, format: { enable: true } }
  const sections = ["typescript", "javascript", "js/ts", "other", undefined, "typescript", "javascript"]
  const values = [value, value, value, value, value, null, false]
  const result = await configure({ items: sections.map((section) => ({ section })) }, {}, async () => values)
  expect(result).toEqual([{ format: { enable: true } }, { format: { enable: true } }, { format: { enable: true } }, value, value, null, false])
  expect(value.validate.enable).toBe(false)
  expect(await configure({ items: [] }, {}, async () => null)).toBeNull()
})

test("start failure remains recoverable through restart and stale client events are ignored", async () => {
  failStart = true
  await start()
  expect(status.text).toBe("vibelang failed")
  expect(logs.join("\n")).toContain("compiler failed")
  const previous = clients[0]
  failStart = false
  await command("restartServer")
  expect(previous.stopped).toBe(true)
  previous.handler({ newState: 1 })
  expect(status.text).toBe("vibelang")
  clients[1].handler({ newState: 1 })
  expect(status.text).toBe("vibelang stopped")
  expect(status.severity).toBe(1)
})

test("restart continues after stop failure and exposes output", async () => {
  await start()
  failStop = true
  await command("restartServer")
  expect(clients).toHaveLength(2)
  expect(logs.join("\n")).toContain("Stopping the language server failed")
  await command("showOutput")
  expect(executed).toContainEqual(["show"])
})

test("server settings, Cursor ownership changes and workspace folder changes restart the client", async () => {
  await start()
  onConfiguration({ affectsConfiguration: (section: string) => section === "unrelated" })
  await Bun.sleep(0)
  expect(clients).toHaveLength(1)
  for (const section of ["vibelang.server", "typescript.experimental.useTsgo"]) {
    const previous = clients.at(-1)!
    onConfiguration({ affectsConfiguration: (candidate: string) => candidate === section })
    await Bun.sleep(0)
    expect(previous.stopped).toBe(true)
  }
  onFolders()
  await Bun.sleep(0)
  expect(clients).toHaveLength(4)
})

test("Cursor's serve TypeScript command updates workspace settings and restarts the host", async () => {
  env.appName = "Cursor"
  await start()
  await command("serveTypeScriptFiles")
  expect(updates).toEqual([["typescript", "experimental.useTsgo", true, 2]])
  expect(executed).toContainEqual(["workbench.action.restartExtensionHost"])
})

test("the serve TypeScript command acknowledges an already disabled built-in extension", async () => {
  builtin = false
  await start()
  await command("serveTypeScriptFiles")
  expect(executed.some((e) => String(e[1]).includes("already serves"))).toBe(true)
  expect(updates).toHaveLength(0)
})

for (const choice of [undefined, "Show Extension"]) {
  test(`VS Code's serve TypeScript command offers built-in extension settings: ${choice ?? "dismiss"}`, async () => {
    infoChoice = choice
    await start()
    await command("serveTypeScriptFiles")
    expect(updates).toHaveLength(0)
    if (choice) expect(executed).toContainEqual(["workbench.extensions.search", "@builtin TypeScript and JavaScript Language Features"])
    else expect(executed.filter((e) => e[0] === "workbench.extensions.search")).toHaveLength(0)
  })
}

test("an unconfigured workspace without any ancestor compiler reports a missing server", async () => {
  settings["vibelang.server.path"] = ""
  process.env.PATH = ""
  await start()
  expect(clients).toHaveLength(0)
  expect(status.text).toBe("vibelang not found")
})
