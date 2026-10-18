import { expect, test } from "bun:test"
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { defaultCompiler, linkModules, normalizeOutput } from "./check-snippets"

const repo = resolve(import.meta.dir, "../..")
const compiler = defaultCompiler()

test.skipIf(!existsSync(compiler))("the local starter runs and its stdout matches both first-run guides", async () => {
  const temp = mkdtempSync(join(tmpdir(), "docs-first-run-"))
  const command = async (args: string[], cwd = temp) => {
    const child = Bun.spawn(args, { cwd, env: { ...process.env, VIBELANG_BINARY: compiler }, stdout: "pipe", stderr: "pipe" })
    const [stdout, stderr, exitCode] = await Promise.all([
      new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited
    ])
    expect({ args, exitCode, failure: exitCode ? stdout + stderr : "" }).toEqual({ args, exitCode: 0, failure: "" })
    return stdout
  }
  try {
    // Generate the same files as npm/bun create, without registry installs.
    await command(["node", join(repo, "packages/create-vibelang/bin/create-vibelang.mjs"), "app", "--no-install"])
    linkModules(temp)
    const app = join(temp, "app")
    // The Vite plugin must be able to check and emit a new, unbuilt app.
    const tests = await command(["node", join(repo, "docs/tools/node_modules/vitest/vitest.mjs"), "run"], app)
    expect(tests).toContain("2 passed")
    await command([compiler, "-p", "tsconfig.json"], app)
    const stdout = await command(["node", "--enable-source-maps", "dist/main.js"], app)
    for (const page of ["013-getting-started.md", "getting-started/004-installation.md"]) {
      const markdown = readFileSync(join(repo, "docs/user/src", page), "utf8")
      const outputs = [...markdown.matchAll(/^```out\r?\n([\s\S]*?)\r?\n```/gm)]
      expect({ page, outputs: outputs.length }).toEqual({ page, outputs: 1 })
      expect({ page, stdout: normalizeOutput(stdout) }).toEqual({ page, stdout: normalizeOutput(outputs[0][1]) })
    }
  } finally { rmSync(temp, { recursive: true, force: true }) }
}, 30000)
