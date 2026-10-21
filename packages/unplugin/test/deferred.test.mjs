// vibelang-unplugin/deferred: the bundled program carries no compiler, and a
// bundler that awaits it compiles .vibe modules as vibelang-unplugin/<bundler>
// does. Run test/run.sh, which links the fixtures' node_modules first.
import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import { cp, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { describe, it } from "node:test"
import { fileURLToPath } from "node:url"

const here = fileURLToPath(new URL(".", import.meta.url))
const expected = await readFile(join(here, "expected.txt"), "utf8")

async function fixture(t, name) {
  const root = await mkdtemp(join(here, ".fixtures", `${name}-`))
  t.after(() => rm(root, { recursive: true, force: true }))
  await cp(join(here, "fixture"), root, { recursive: true })
  return root
}

describe("vibelang-unplugin/deferred", () => {
  it("adds nothing but a promise-like object to a program that bundles it", async (t) => {
    const root = await fixture(t, "deferred-host")
    // A Worker module that declares its own bundler options, as an Alchemy Worker does.
    await writeFile(join(root, "worker.js"), [
      "import { deferred } from \"vibelang-unplugin/deferred\"",
      "export const build = { input: { plugins: [deferred(\"rolldown\")] } }",
      "console.log(typeof build.input.plugins[0].then)",
      ""
    ].join("\n"))
    const { rolldown } = await import("rolldown")
    const bundle = await rolldown({ input: join(root, "worker.js"), cwd: root, platform: "browser", onLog() {} })
    const { output } = await bundle.generate({ format: "es" })
    await bundle.close()
    assert.equal(output.length, 1, `one chunk: ${output.map((chunk) => chunk.fileName).join(", ")}`)
    const code = output[0].code
    assert.doesNotMatch(code, /child_process|createUnplugin|vibelang\/project/)
    await writeFile(join(root, "worker.bundle.mjs"), code)
    const run = spawnSync(process.execPath, [join(root, "worker.bundle.mjs")], { cwd: root, encoding: "utf8" })
    assert.equal(run.status, 0, run.stderr)
    assert.equal(run.stdout.trim(), "function")
  })

  for (const bundler of ["rolldown", "rollup"]) {
    it(`compiles .vibe modules once ${bundler} awaits it`, async (t) => {
      const root = await fixture(t, `deferred-${bundler}`)
      const build = spawnSync(process.execPath, ["--no-warnings", join(here, "build.mjs"), bundler, "--deferred"], { cwd: root, encoding: "utf8" })
      assert.equal(build.status, 0, `${build.stdout}\n${build.stderr}`)
      assert.equal(JSON.parse(build.stdout.trim().split("\n").at(-1)).ok, true)
      const run = spawnSync(process.execPath, ["out/main.js"], { cwd: root, encoding: "utf8" })
      assert.equal(run.status, 0, run.stderr)
      assert.equal(run.stdout, expected)
    })
  }

  it("starts nothing until awaited, then resolves to the bundler's plugin", async () => {
    const { deferred } = await import("vibelang-unplugin/deferred")
    const pending = deferred("rollup")
    assert.equal(Object.prototype.toString.call(pending), "[object Promise]")
    const plugin = await pending
    assert.equal(plugin.name, "vibelang")
    assert.equal(await pending.finally(() => {}), plugin)
  })

  it("refuses a bundler that does not await plugins", async () => {
    const { deferred } = await import("vibelang-unplugin/deferred")
    assert.throws(() => deferred("esbuild"), /esbuild does not await plugins; use one of rolldown, rollup, vite/)
  })
})
