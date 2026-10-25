import { expect, test } from "bun:test"
import { cpSync, mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const packageDir = fileURLToPath(new URL("..", import.meta.url))
const effectDir = dirname(fileURLToPath(import.meta.resolve("effect/package.json")))
const effectMetadata = JSON.parse(readFileSync(join(effectDir, "package.json"), "utf8"))
const fixture = (version: string) => {
  const dir = mkdtempSync(join(tmpdir(), "vibelang-runtime-"))
  const runtime = join(dir, "node_modules/vibelang-runtime")
  const effect = join(dir, "node_modules/effect")
  mkdirSync(runtime, { recursive: true })
  mkdirSync(effect, { recursive: true })
  cpSync(join(packageDir, "dist"), join(runtime, "dist"), { recursive: true })
  cpSync(join(packageDir, "package.json"), join(runtime, "package.json"))
  writeFileSync(join(effect, "package.json"), JSON.stringify({ ...effectMetadata, version }))
  symlinkSync(join(effectDir, "dist"), join(effect, "dist"))
  writeFileSync(join(dir, "package.json"), '{"type":"module"}')
  writeFileSync(join(dir, "main.js"), `import { Effect, $bind } from "vibelang-runtime"; console.log(Effect.runSync($bind(42)))`)
  return dir
}

for (const version of ["4.0.5", "4.0.6", "3.99.0"]) {
  test(`package: Node, Bun, and browser bundles check effect@${version}`, async () => {
    const dir = fixture(version)
    try {
      const bundled = await Bun.build({
        entrypoints: [join(dir, "main.js")], target: "browser", format: "esm",
        outdir: join(dir, "bundle"), minify: true
      })
      expect(bundled.success).toBe(true)
      const bundle = await bundled.outputs[0].text()
      expect(bundle).not.toContain("node:module")
      const commands = [["node", "main.js"], [process.execPath, "main.js"], ["node", "bundle/main.js"]]
      for (const command of commands) {
        const result = Bun.spawnSync(command, { cwd: dir, stdout: "pipe", stderr: "pipe" })
        const output = result.stdout.toString() + result.stderr.toString()
        if (version === "4.0.5") {
          expect(result.exitCode).toBe(0)
          expect(output.trim()).toBe("42")
        } else {
          expect(result.exitCode).not.toBe(0)
          expect(output).toContain("vibelang-runtime@0.0.5 requires effect@4.0.5")
          expect(output).toContain(`found effect@${version}`)
          expect(output).toContain("npm install effect@4.0.5")
        }
      }
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  }, 30_000)
}

test("package: emitted declarations resolve and retain helper types", () => {
  const result = Bun.spawnSync(["bun", "x", "--no-install", "tsc", "-p", "tests/tsconfig.json"], {
    cwd: packageDir, stdout: "pipe", stderr: "pipe"
  })
  expect(result.stdout.toString() + result.stderr.toString()).toBe("")
  expect(result.exitCode).toBe(0)
}, 30_000)
