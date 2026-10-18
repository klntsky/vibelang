import { strict as assert } from "node:assert"
import { spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const create = fileURLToPath(new URL("../bin/create-vibelang.mjs", import.meta.url))
const version = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8")).version

function run(cwd, ...args) {
  return spawnSync(process.execPath, [create, ...args], { cwd, encoding: "utf8", env: { ...process.env, npm_config_user_agent: "npm/11.0.0 node/v26.0.0" } })
}

test("copies the starter app and pins the VibeLang packages to its own version", () => {
  const scratch = mkdtempSync(join(tmpdir(), "create-vibelang-"))
  try {
    const result = run(scratch, "My App", "--no-install", "--no-git")
    assert.equal(result.status, 0, result.stderr)
    const app = join(scratch, "My App")
    for (const file of ["src/main.vibe", "src/greeting.vibe", "src/greeting.test.vibe", "tsconfig.json", "vitest.config.ts", "scripts/dev.mjs", ".gitignore", ".gitattributes", "README.md"]) {
      assert.ok(existsSync(join(app, file)), file)
    }
    assert.ok(!existsSync(join(app, "node_modules")))
    assert.ok(!existsSync(join(app, ".git")))
    const manifest = JSON.parse(readFileSync(join(app, "package.json"), "utf8"))
    assert.equal(manifest.name, "my-app")
    assert.equal(manifest.dependencies["vibelang-runtime"], version)
    assert.equal(manifest.devDependencies.vibelang, version)
    assert.equal(manifest.devDependencies["vibelang-vite"], version)
    assert.match(result.stdout, /npm install/)
    assert.match(result.stdout, /node_modules\/vibelang\/editor\/vibelang\.vsix/)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("refuses a directory that is not empty", () => {
  const scratch = mkdtempSync(join(tmpdir(), "create-vibelang-"))
  try {
    mkdirSync(join(scratch, "app"))
    writeFileSync(join(scratch, "app", "keep.txt"), "mine")
    const result = run(scratch, "app", "--no-install")
    assert.equal(result.status, 1)
    assert.match(result.stderr, /app is not empty/)
    assert.equal(readFileSync(join(scratch, "app", "keep.txt"), "utf8"), "mine")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("rejects unknown options", () => {
  const result = run(tmpdir(), "--template", "x")
  assert.equal(result.status, 1)
  assert.match(result.stderr, /unknown option --template/)
})
