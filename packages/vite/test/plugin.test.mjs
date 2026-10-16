import assert from "node:assert/strict"
import { spawn } from "node:child_process"
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { createRequire } from "node:module"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import test from "node:test"
import { build, createServer } from "vite"
import effectscript from "../src/index.js"

const packageRoot = fileURLToPath(new URL("../", import.meta.url))
const compiler = join(packageRoot, "../compiler/bin/esc")
const require = createRequire(import.meta.url)
const vitest = join(dirname(require.resolve("vitest/package.json")), "vitest.mjs")

async function fixture(t) {
  const base = join(packageRoot, "test/.fixtures")
  await mkdir(base, { recursive: true })
  const root = await mkdtemp(join(base, "fixture-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  await cp(join(packageRoot, "test/project"), root, { recursive: true })
  await writeFile(join(root, "vitest.config.ts"), `
import { defineConfig } from "vitest/config"
import effectscript from ${JSON.stringify(join(packageRoot, "src/index.js"))}
export default defineConfig({
  plugins: [effectscript({ compiler: ${JSON.stringify(compiler)} })],
  test: { include: [process.env.FAILING_TEST ? "failure.test.es" : "passing.test.es"] }
})
`)
  return root
}

function runVitest(root, failing = false) {
  return new Promise((accept, reject) => {
    let output = ""
    const child = spawn(process.execPath, [vitest, "run", "--no-color"], {
      cwd: root, env: { ...process.env, FAILING_TEST: failing ? "1" : "", NO_COLOR: "1" }
    })
    child.stdout.on("data", (data) => { output += data })
    child.stderr.on("data", (data) => { output += data })
    child.on("error", reject)
    child.on("close", (code) => accept({ code, output }))
  })
}

async function until(check) {
  const deadline = Date.now() + 10000
  while (Date.now() < deadline) {
    if (await check()) return
    await new Promise((accept) => setTimeout(accept, 25))
  }
  assert.fail("Timed out waiting for Vite's file watcher")
}

test("Vitest runs .es Effect functions and plain callbacks", async (t) => {
  const result = await runVitest(await fixture(t))
  assert.equal(result.code, 0, result.output)
  assert.match(result.output, /2 passed/)
  console.log("PASS .es Vitest: 2 assertions")
})

test("a deliberate failing assertion reports failure.test.es:6", async (t) => {
  const result = await runVitest(await fixture(t), true)
  assert.equal(result.code, 1, result.output)
  assert.match(result.output, /expected 41 to equal 42/)
  assert.match(result.output, /failure\.test\.es:6:28/)
  console.log("PASS expected failure: assertion stack at failure.test.es:6:28")
})

test("Vite serves JS and maps, invalidates importers, reports diagnostics and recovers", async (t) => {
  const root = await fixture(t)
  const configPath = join(root, "tsconfig.json")
  const config = JSON.parse(await readFile(configPath, "utf8"))
  config.compilerOptions.sourceRoot = "https://example.invalid/sources/"
  await writeFile(configPath, JSON.stringify(config))
  await writeFile(join(root, "entry.ts"), 'export { answer } from "./main.js"\n')
  const errors = []
  const server = await createServer({
    root, configFile: false, plugins: [effectscript({ compiler })],
    customLogger: { info() {}, warn() {}, warnOnce() {}, error: (message) => errors.push(message),
      clearScreen() {}, hasErrorLogged() { return false }, hasWarned: false },
    server: { host: "127.0.0.1", port: 0 }
  })
  t.after(() => server.close())
  const messages = []
  const originalSend = server.ws.send.bind(server.ws)
  server.ws.send = (...args) => { messages.push(args[0]); return originalSend(...args) }
  await server.listen()
  const address = server.httpServer.address()
  const base = `http://127.0.0.1:${address.port}`
  let response = await fetch(`${base}/entry.ts`)
  assert.equal(response.status, 200)
  assert.match(await response.text(), /main\.es/)
  response = await fetch(`${base}/main.es`)
  assert.equal(response.status, 200)
  const javascript = await response.text()
  assert.match(javascript, /yield\* Effect\.succeed/)
  assert.doesNotMatch(javascript, /value: number/)
  const map = JSON.parse(Buffer.from(javascript.match(/sourceMappingURL=data:application\/json;base64,(\S+)/)[1], "base64"))
  assert.ok(map.sources.some((source) => source.endsWith("main.es")))
  assert.ok(map.sources.every((source) => !source.includes("example.invalid")))
  assert.ok(map.sourcesContent.some((source) => source.includes("value: number")))

  const importer = server.moduleGraph.getModuleById(join(root, "entry.ts"))
  assert.ok(importer.transformResult)
  const sourcePath = join(root, "main.es")
  const source = await readFile(sourcePath, "utf8")
  await writeFile(sourcePath, source.replace("double(21)", "double(22)"))
  await until(() => !importer.transformResult)
  response = await fetch(`${base}/main.es`)
  assert.equal(response.status, 200)
  assert.match(await response.text(), /double(\$direct)?\(22\)/)

  await writeFile(sourcePath, source + '\nconst broken: number = "wrong"\n')
  await until(() => messages.some((message) => message?.type === "error"))
  assert.ok(errors.some((message) => /TS2322/.test(message)), errors.join("\n"))
  const diagnostic = messages.find((message) => message?.type === "error").err
  assert.equal(diagnostic.plugin, "effectscript")
  assert.equal(diagnostic.loc.file, sourcePath)
  assert.match(diagnostic.message, /TS2322/)
  response = await fetch(`${base}/main.es`)
  assert.equal(response.status, 500)
  await writeFile(sourcePath, source.replace("double(21)", "double(23)"))
  await until(async () => {
    const recovered = await fetch(`${base}/main.es`)
    return recovered.status === 200 && /double(\$direct)?\(23\)/.test(await recovered.text())
  })
  console.log("PASS HTTP: JS, .es source map, importer invalidation, TS2322 terminal/overlay, recovery")
})

test("Vite production builds accept .es entry modules", async (t) => {
  const root = await fixture(t)
  const result = await build({
    root, configFile: false, plugins: [effectscript({ compiler })], logLevel: "silent",
    build: { write: false, sourcemap: true, lib: { entry: join(root, "main.es"), formats: ["es"] },
      rollupOptions: { external: ["effect", "effect/Effect"] } }
  })
  const output = result.output || result[0].output
  assert.ok(output.some((file) => file.type === "chunk" && file.code.includes("succeed")))
  assert.ok(output.some((file) => file.type === "asset" && file.fileName.endsWith(".map")))
})
