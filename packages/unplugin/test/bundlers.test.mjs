// Builds test/fixture with each bundler through effeck-unplugin, runs the
// bundle, and checks its output, its source map, compiler diagnostics, and
// watch-mode recompiles. Run test/run.sh, which builds the compiler and the
// runtime and links the fixtures' node_modules first.
import assert from "node:assert/strict"
import { spawn, spawnSync } from "node:child_process"
import { once } from "node:events"
import { cp, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { SourceMap } from "node:module"
import { availableParallelism, tmpdir } from "node:os"
import { join } from "node:path"
import { createInterface } from "node:readline"
import { after, describe, it } from "node:test"
import { fileURLToPath } from "node:url"

const here = fileURLToPath(new URL(".", import.meta.url))
const build = join(here, "build.mjs")
const expected = await readFile(join(here, "expected.txt"), "utf8")
const hasBun = spawnSync("bun", ["--version"]).status === 0

// The plugin compiles into os.tmpdir(). A compile that close() interrupts
// leaves its directory there; keep this run's directories in one place.
process.env.TMPDIR = await mkdtemp(join(tmpdir(), "effeck-unplugin-test-"))
after(() => rm(process.env.TMPDIR, { recursive: true, force: true }))

// Bun.build has no watch mode.
const bundlers = {
  webpack: { bundle: "out/main.cjs", watch: true },
  rspack: { bundle: "out/main.cjs", watch: true },
  rollup: { bundle: "out/main.js", watch: true },
  rolldown: { bundle: "out/main.js", watch: true },
  esbuild: { bundle: "out/main.js", watch: true },
  bun: { bundle: "out/main.js", watch: false },
  vite: { bundle: "out/main.js", watch: true }
}

async function fixture(t, name) {
  // Fixtures share test/.fixtures/node_modules, which run.sh links.
  const root = await mkdtemp(join(here, ".fixtures", `${name}-`))
  t.after(() => rm(root, { recursive: true, force: true }))
  await cp(join(here, "fixture"), root, { recursive: true })
  return root
}

async function edit(file, from, to, watcher) {
  const source = await readFile(file, "utf8")
  assert.ok(source.includes(from), `${file} has no ${from}`)
  await writeFile(file, source.replace(from, to))
  watcher?.log(`edit ${file.split("/").at(-1)}: ${from} -> ${to}`)
}

function spawned(command, args, cwd) {
  return new Promise((accept, reject) => {
    let stdout = ""
    let stderr = ""
    const child = spawn(command, args, { cwd })
    child.stdout.on("data", (data) => { stdout += data })
    child.stderr.on("data", (data) => { stderr += data })
    child.on("error", reject)
    child.on("close", (code) => accept({ code, stdout, stderr }))
  })
}

const escape = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
const runtime = (bundler) => (bundler === "bun" ? "bun" : process.execPath)

async function bundleOnce(bundler, root) {
  const result = await spawned(runtime(bundler), ["--no-warnings", build, bundler], root)
  const line = result.stdout.trim().split("\n").at(-1)
  assert.ok(line?.startsWith("{"), `${bundler} printed no result\n${result.stdout}\n${result.stderr}`)
  return { code: result.code, event: JSON.parse(line), stderr: result.stderr }
}

async function runBundle(bundler, root) {
  const result = await spawned(runtime(bundler), [bundlers[bundler].bundle], root)
  assert.equal(result.code, 0, result.stderr)
  return result.stdout
}

// The bundle prints text once a rebuild has written it. A bundle that is
// still being written fails to run, which counts as not yet.
async function printsEventually(bundler, root, text) {
  for (let attempt = 0; attempt < 20; attempt++) {
    const result = await spawned(runtime(bundler), [bundlers[bundler].bundle], root)
    if (result.code === 0 && result.stdout.includes(text)) return true
    await new Promise((accept) => setTimeout(accept, 100))
  }
  return false
}

function watchBundle(bundler, root, env = {}) {
  // A process group of its own: close() stops the bundler and its compiler, as Ctrl-C does.
  const child = spawn(process.execPath, ["--no-warnings", build, bundler, "--watch"], {
    cwd: root, detached: true, env: { ...process.env, EFFECK_UNPLUGIN_DEBUG: "1", ...env }
  })
  const started = Date.now()
  // The failure message shows each build result, edit, and trace line in order.
  const history = []
  const log = (text) => history.push(`+${Date.now() - started}ms ${text}`)
  createInterface({ input: child.stderr }).on("line", (line) => history.push(`  stderr: ${line}`))
  const events = []
  let wake = () => {}
  createInterface({ input: child.stdout }).on("line", (line) => {
    if (!line.startsWith("{")) return
    const event = JSON.parse(line)
    log(event.ok ? "build ok" : `build failed: ${event.message.split("\n")[0]}`)
    events.push(event)
    wake()
  })
  return {
    // Waits for a build event that passes check. Earlier events are skipped,
    // so a bundler that reports one change twice still passes.
    async next(what, check) {
      const deadline = Date.now() + 60000
      while (Date.now() < deadline) {
        while (events.length) {
          const event = events.shift()
          if (await check(event)) return event
        }
        await new Promise((accept) => { wake = accept; setTimeout(accept, 250) })
      }
      assert.fail(`${bundler} --watch: no build ${what} within 60s\n${history.join("\n")}`)
    },
    log,
    // Stops the bundler and any compile it started before the fixture is removed.
    async close() {
      if (child.exitCode === null && child.signalCode === null) {
        const exited = once(child, "exit")
        process.kill(-child.pid)
        await exited
      }
      for (let attempt = 0; attempt < 100; attempt++) {
        try {
          process.kill(-child.pid, 0)
        } catch {
          return
        }
        await new Promise((accept) => setTimeout(accept, 50))
      }
    }
  }
}

// Edits a .effeck source, a type-only .ts import, and the tsconfig under a
// running watcher, and waits for the rebuild each edit causes.
async function watchEdits(bundler, root, watcher) {
  // Each edit waits for the bundler to re-arm its watcher after a build.
  // webpack before 5.112.0 misses a save made as a build finishes
  // (webpack/webpack#22646); that race is webpack's, not the plugin's.
  const settle = (ms = 500) => new Promise((accept) => setTimeout(accept, ms))
  await watcher.next("success", (event) => event.ok)
  assert.ok(await printsEventually(bundler, root, expected))
  await settle(1000)

  await edit(join(root, "src/shop.effeck"), 'receipt("ordered"', 'receipt("booked"', watcher)
  await watcher.next("with the .effeck edit", (event) => event.ok && printsEventually(bundler, root, "booked 2 tea"))

  // order.ts is a type-only import: only the compiler reads it.
  const order = join(root, "src/order.ts")
  await settle()
  await edit(order, "quantity: number", "quantity: string", watcher)
  const failed = await watcher.next("failure after the .ts edit", (event) => !event.ok)
  assert.match(failed.message, new RegExp(`${escape(join(root, "src/shop.effeck"))}:\\d+:\\d+ - error TS\\d+`))
  await settle()
  await edit(order, "quantity: string", "quantity: number", watcher)
  await watcher.next("success after the .ts fix", (event) => event.ok)

  const tsconfig = join(root, "tsconfig.json")
  await settle()
  await edit(tsconfig, '"types": []', '"types": ["missing-types"]', watcher)
  const broken = await watcher.next("failure after the tsconfig edit", (event) => !event.ok)
  assert.match(broken.message, /error TS2688: Cannot find type definition file for 'missing-types'/)
  await settle()
  await edit(tsconfig, '"types": ["missing-types"]', '"types": []', watcher)
  await watcher.next("success after the tsconfig fix", (event) => event.ok && printsEventually(bundler, root, "booked 2 tea"))
}

const concurrency = Math.max(2, Math.min(8, availableParallelism() >> 1))

describe("effeck-unplugin", { concurrency }, () => {
  for (const [bundler, { bundle, watch }] of Object.entries(bundlers)) {
    const skip = bundler === "bun" && !hasBun && "bun is not installed"

    it(`${bundler}: builds the fixture, runs it, and maps it to .effeck sources`, { skip }, async (t) => {
      const root = await fixture(t, bundler)
      const result = await bundleOnce(bundler, root)
      assert.deepEqual(result.event, { ok: true }, result.stderr)
      assert.equal(result.code, 0)
      assert.equal(await runBundle(bundler, root), expected)

      // "coffee" is at shop.effeck line 32, column 19.
      const code = await readFile(join(root, bundle), "utf8")
      const payload = JSON.parse(await readFile(join(root, bundle + ".map"), "utf8"))
      const source = payload.sources.findIndex((path) => path.endsWith("src/shop.effeck"))
      assert.ok(source >= 0, `no shop.effeck in ${payload.sources.join(", ")}`)
      const lines = code.split("\n")
      const line = lines.findIndex((text) => /"coffee"/.test(text))
      const entry = new SourceMap(payload).findEntry(line, lines[line].indexOf('"coffee"'))
      assert.match(entry.originalSource, /src\/shop\.effeck$/)
      if (bundler === "bun") {
        // Bun ignores plugin source maps: its map points into the compiled JavaScript.
        assert.ok(!payload.sourcesContent[source].includes("service Inventory"))
        return
      }
      assert.ok(payload.sourcesContent[source].includes("service Inventory"), "the map embeds the .effeck source")
      assert.deepEqual([entry.originalLine + 1, entry.originalColumn + 1], [32, 19])
    })

    it(`${bundler}: a type error fails the build at its .effeck position`, { skip }, async (t) => {
      const root = await fixture(t, `${bundler}-error`)
      const file = join(root, "src/shop.effeck")
      await writeFile(file, (await readFile(file, "utf8")) + 'export const broken: number = "wrong"\n')
      // Two .effeck modules load from the failed compile; it is reported once.
      await edit(join(root, "src/main.ts"), 'import { Effect }', 'import "./format.effeck"\nimport { Effect }')
      const result = await bundleOnce(bundler, root)
      assert.equal(result.code, 1)
      assert.equal(result.event.ok, false)
      assert.ok(result.event.message.includes(`${file}:35:14 - error TS2322: Type 'string' is not assignable to type 'number'.`),
        result.event.message)
      assert.equal(result.event.message.match(/TS2322/g).length, 1, "one report per compile")
    })

    if (!watch) continue
    it(`${bundler}: watch mode recompiles on .effeck, .ts and tsconfig changes`, async (t) => {
      const root = await fixture(t, `${bundler}-watch`)
      const watcher = watchBundle(bundler, root)
      try {
        await watchEdits(bundler, root, watcher)
      } finally {
        await watcher.close()
      }
    })

    it(`${bundler}: watch mode keeps a save made during a compile`, async (t) => {
      const root = await fixture(t, `${bundler}-save`)
      const watcher = watchBundle(bundler, root, {
        EFFECK_BINARY: join(here, "save-during-compile.mjs"), EFFECK_REAL: process.env.EFFECK_BINARY
      })
      try {
        await watcher.next("success", (event) => event.ok)
        await new Promise((accept) => setTimeout(accept, 1000))
        // The rebuild's compiler reads "booked"; the save to "sold" lands before its output is used.
        await edit(join(root, "src/shop.effeck"), 'receipt("ordered"', 'receipt("booked"', watcher)
        await watcher.next("with the save made during the compile", (event) => event.ok && printsEventually(bundler, root, "sold 2 tea"))
      } finally {
        await watcher.close()
      }
    })
  }
})
