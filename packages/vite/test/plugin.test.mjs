import assert from "node:assert/strict"
import { spawn } from "node:child_process"
import { cp, mkdir, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises"
import { createRequire } from "node:module"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import test from "node:test"
import { build, createServer } from "vite"
import vibelang from "../src/index.js"

const packageRoot = fileURLToPath(new URL("../", import.meta.url))
const compiler = join(packageRoot, "../compiler/bin/vibelang")
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
import vibelang from ${JSON.stringify(join(packageRoot, "src/index.js"))}
export default defineConfig({
  plugins: [vibelang({ compiler: ${JSON.stringify(compiler)} })],
  test: { include: [process.env.FAILING_TEST ? "failure.test.vibe" : "passing.test.vibe"] }
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

test("Vitest runs .vibe Effect functions and plain callbacks", async (t) => {
  const result = await runVitest(await fixture(t))
  assert.equal(result.code, 0, result.output)
  assert.match(result.output, /2 passed/)
  console.log("PASS .vibe Vitest: 2 assertions")
})

test("Vitest compiles a workspace package's .vibe source with that package's own project", async (t) => {
  // dep lies beside the fixture, outside its root, as a workspace package
  // that the test imports by name through node_modules.
  const root = await fixture(t)
  const dep = `${root}-dep`
  t.after(() => rm(dep, { recursive: true, force: true }))
  await mkdir(join(dep, "src"), { recursive: true })
  await writeFile(join(dep, "package.json"), JSON.stringify({ name: "dep", type: "module", exports: { ".": "./src/index.vibe" } }))
  const config = JSON.parse(await readFile(join(root, "tsconfig.json"), "utf8"))
  await writeFile(join(dep, "tsconfig.json"), JSON.stringify({ ...config, include: ["src/**/*"] }))
  await writeFile(join(dep, "src/index.vibe"), "export function twice(value: number): number {\n  return value * 2\n}\n")
  await mkdir(join(root, "node_modules"), { recursive: true })
  await symlink(dep, join(root, "node_modules/dep"))
  await writeFile(join(root, "passing.test.vibe"), `import { assert, it } from "@effect/vitest"
import { twice } from "dep"

it("uses a workspace package's VibeLang source", () => {
  assert.strictEqual(twice(21), 42)
})
`)
  const result = await runVitest(root)
  assert.equal(result.code, 0, result.output)
  assert.match(result.output, /1 passed/)
})

test("a Vitest rerun recompiles a .vibe module whose import changed its contract", async (t) => {
  // Vitest 5 keeps transforms on disk between runs (fsModuleCache), keyed by
  // the module's own text; passing.test.vibe does not change, but value()
  // becomes an Effect function, so its call must now run the Effect.
  const root = await fixture(t)
  await writeFile(join(root, "passing.test.vibe"), `import { assert, it } from "@effect/vitest"
import { value } from "./dep.vibe"

it.effect("reads its dependency") {
  assert.strictEqual(value(), 7)
}
`)
  await writeFile(join(root, "dep.vibe"), "export function value(): number {\n  return 7\n}\n")
  let result = await runVitest(root)
  assert.equal(result.code, 0, result.output)
  await writeFile(join(root, "dep.vibe"), "export function value(): number!! {\n  Console.log(\"value\")\n  return 7\n}\n")
  result = await runVitest(root)
  assert.equal(result.code, 0, result.output)
  assert.match(result.output, /1 passed/)
  console.log("PASS Vitest rerun: a changed import's contract recompiles its importer")
})

test("register: true lets a Node process a test starts import .vibe source, from any directory", async (t) => {
  const root = await fixture(t)
  await writeFile(join(root, "plain.vibe"), "export function triple(value: number): number {\n  return value * 3\n}\n")
  await writeFile(join(root, "spawn.test.ts"), `import { execFileSync } from "node:child_process"
import { tmpdir } from "node:os"
import { expect, it } from "vitest"

it("starts a Node process that imports a .vibe module", () => {
  const plain = new URL("./plain.vibe", import.meta.url).href
  const program = \`const { triple } = await import(\${JSON.stringify(plain)}); console.log(triple(14), process.env.NODE_OPTIONS.includes("--no-warnings"))\`
  const output = execFileSync(process.execPath, ["--input-type=module", "-e", program], { cwd: tmpdir(), encoding: "utf8" })
  expect(output.trim()).toBe("42 true")
})
`)
  const config = (register) => `
import { defineConfig } from "vitest/config"
import vibelang from ${JSON.stringify(join(packageRoot, "src/index.js"))}
export default defineConfig({
  plugins: [vibelang({ compiler: ${JSON.stringify(compiler)}, register: ${register} })],
  test: { include: ["spawn.test.ts"], env: { NODE_OPTIONS: "--no-warnings" } }
})
`
  await writeFile(join(root, "vitest.config.ts"), config(true))
  const registered = await runVitest(root)
  assert.equal(registered.code, 0, registered.output)
  assert.match(registered.output, /1 passed/)
  await writeFile(join(root, "vitest.config.ts"), config(false))
  const plain = await runVitest(root)
  assert.equal(plain.code, 1, plain.output)
  assert.match(plain.output, /ERR_UNKNOWN_FILE_EXTENSION|Unknown file extension "\.vibe"/)
})

test("a deliberate failing assertion reports failure.test.vibe:6", async (t) => {
  const result = await runVitest(await fixture(t), true)
  assert.equal(result.code, 1, result.output)
  assert.match(result.output, /expected 41 to equal 42/)
  assert.match(result.output, /failure\.test\.vibe:6:28/)
  console.log("PASS expected failure: assertion stack at failure.test.vibe:6:28")
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
    root, configFile: false, plugins: [vibelang({ compiler })],
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
  assert.match(await response.text(), /main\.vibe/)
  response = await fetch(`${base}/main.vibe`)
  assert.equal(response.status, 200)
  const javascript = await response.text()
  // D274: a single-result Effect body returns its Effect without a generator.
  assert.match(javascript, /Effect\.succeed\(double(\$direct)?\(21\)\)/)
  assert.doesNotMatch(javascript, /value: number/)
  const map = JSON.parse(Buffer.from(javascript.match(/sourceMappingURL=data:application\/json;base64,(\S+)/)[1], "base64"))
  assert.ok(map.sources.some((source) => source.endsWith("main.vibe")))
  assert.ok(map.sources.every((source) => !source.includes("example.invalid")))
  assert.ok(map.sourcesContent.some((source) => source.includes("value: number")))

  const importer = server.moduleGraph.getModuleById(join(root, "entry.ts"))
  assert.ok(importer.transformResult)
  const sourcePath = join(root, "main.vibe")
  const source = await readFile(sourcePath, "utf8")
  await writeFile(sourcePath, source.replace("double(21)", "double(22)"))
  await until(() => !importer.transformResult)
  response = await fetch(`${base}/main.vibe`)
  assert.equal(response.status, 200)
  assert.match(await response.text(), /double(\$direct)?\(22\)/)

  await writeFile(sourcePath, source + '\nconst broken: number = "wrong"\n')
  await until(() => messages.some((message) => message?.type === "error"))
  assert.ok(errors.some((message) => /TS2322/.test(message)), errors.join("\n"))
  const diagnostic = messages.find((message) => message?.type === "error").err
  assert.equal(diagnostic.plugin, "vibelang")
  assert.equal(diagnostic.loc.file, sourcePath)
  assert.match(diagnostic.message, /TS2322/)
  response = await fetch(`${base}/main.vibe`)
  assert.equal(response.status, 500)
  await writeFile(sourcePath, source.replace("double(21)", "double(23)"))
  await until(async () => {
    const recovered = await fetch(`${base}/main.vibe`)
    return recovered.status === 200 && /double(\$direct)?\(23\)/.test(await recovered.text())
  })
  console.log("PASS HTTP: JS, .vibe source map, importer invalidation, TS2322 terminal/overlay, recovery")
})

test("Vite production builds accept .vibe entry modules", async (t) => {
  const root = await fixture(t)
  const result = await build({
    root, configFile: false, plugins: [vibelang({ compiler })], logLevel: "silent",
    build: { write: false, sourcemap: true, lib: { entry: join(root, "main.vibe"), formats: ["es"] },
      rollupOptions: { external: ["effect", "effect/Effect"] } }
  })
  const output = result.output || result[0].output
  assert.ok(output.some((file) => file.type === "chunk" && file.code.includes("succeed")))
  assert.ok(output.some((file) => file.type === "asset" && file.fileName.endsWith(".map")))
})

test("a native compiler named bin/vibelang is executed without Node", async (t) => {
  const root = await fixture(t)
  const native = join(root, "bin", "vibelang")
  await mkdir(dirname(native), { recursive: true })
  await cp(process.env.VIBELANG_BINARY || join(packageRoot, "../../typescript/built/local/tsc"), native)
  const plugin = vibelang({ compiler: native })
  plugin.configResolved({ root, logger: { warn() {} } })
  const file = join(root, "main.vibe")
  const result = await plugin.transform.call({
    addWatchFile() {},
    error(error) { throw new Error(error.message) }
  }, await readFile(file, "utf8"), file)
  assert.match(result.code, /Effect\.succeed/)
})
