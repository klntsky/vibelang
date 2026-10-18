import { strict as assert } from "node:assert"
import { spawnSync } from "node:child_process"
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs"
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
    for (const file of ["src/main.vibe", "src/greeting.vibe", "src/greeting.test.vibe", "tsconfig.json", "vitest.config.ts", "scripts/dev.mjs", ".gitignore", ".gitattributes", "README.md", "AGENTS.md"]) {
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
    const agents = readFileSync(join(app, "AGENTS.md"), "utf8")
    assert.match(agents, /`npm run build`/)
    assert.match(agents, /`npm run test`/)
    assert.doesNotMatch(agents, /bun run/)
    assert.match(result.stdout, /node_modules\/vibelang\/editor\/vibelang\.vsix/)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

// Every turn of a coding agent in a new app loads AGENTS.md, so it carries
// the docs' Essentials verbatim and stays small (CEO a-yrntzv).
test("the starter's AGENTS.md carries the docs' Essentials and stays under 2 KB", () => {
  const agents = readFileSync(new URL("../../../templates/app/AGENTS.md", import.meta.url), "utf8")
  const essentials = readFileSync(new URL("../../../docs/user/llms-essentials.md", import.meta.url), "utf8")
  assert.ok(agents.includes(essentials), "templates/app/AGENTS.md must contain docs/user/llms-essentials.md verbatim")
  assert.ok(Buffer.byteLength(agents) < 2048, `AGENTS.md is ${Buffer.byteLength(agents)} bytes`)
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

function scratch(t) {
  const directory = mkdtempSync(join(tmpdir(), "create-vibelang-cli-"))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  return directory
}

test("help and version exit successfully without creating an app", (t) => {
  const directory = scratch(t)
  for (const flag of ["-h", "--help", "-v", "--version"]) {
    const result = run(directory, flag)
    assert.equal(result.status, 0, result.stderr)
    assert.equal(result.stdout.trim(), flag.endsWith("h") || flag === "--help"
      ? "Usage: create-vibelang [directory] [--no-install]" : version)
    assert.ok(!existsSync(join(directory, "vibelang-app")))
  }
})

test("rejects extra positional arguments before writing files", (t) => {
  const directory = scratch(t)
  const result = run(directory, "first", "second", "--no-install")
  assert.equal(result.status, 1)
  assert.match(result.stderr, /unexpected argument second/)
  assert.ok(!existsSync(join(directory, "first")))
})

test("uses the default directory and accepts an existing empty directory", (t) => {
  const directory = scratch(t)
  mkdirSync(join(directory, "vibelang-app"))
  const result = run(directory, "--no-install")
  assert.equal(result.status, 0, result.stderr)
  const manifest = JSON.parse(readFileSync(join(directory, "vibelang-app/package.json"), "utf8"))
  assert.equal(manifest.name, "vibelang-app")
})

test("can scaffold the current directory", (t) => {
  const directory = scratch(t)
  const result = run(directory, ".", "--no-install")
  assert.equal(result.status, 0, result.stderr)
  assert.ok(existsSync(join(directory, "src/main.vibe")))
})

test("normalizes directory names to npm package names", (t) => {
  const directory = scratch(t)
  for (const [name, expected] of [["Hello WORLD!", "hello-world"], ["...Hidden_App--", "hidden_app"], ["日本語", "vibelang-app"], ["A~B.C_2", "a~b.c_2"]]) {
    const result = run(directory, name, "--no-install")
    assert.equal(result.status, 0, result.stderr)
    const manifest = JSON.parse(readFileSync(join(directory, name, "package.json"), "utf8"))
    assert.equal(manifest.name, expected)
  }
})

// Exercise real subprocess installation without contacting a registry. Each
// shim records the cwd and arguments, and proves the template was ready first.
for (const manager of ["npm", "bun", "pnpm", "yarn"]) {
  test(`installs with the invoking ${manager} and prints its next steps`, (t) => {
    const directory = scratch(t)
    const bin = join(directory, "bin")
    mkdirSync(bin)
    writeFileSync(join(bin, manager), `#!${process.execPath}\nimport { writeFileSync, existsSync } from "node:fs"\nwriteFileSync(${JSON.stringify(join(directory, "install.json"))}, JSON.stringify({ argv: process.argv.slice(2), cwd: process.cwd(), ready: existsSync("src/main.vibe") }))\n`, { mode: 0o755 })
    const result = spawnSync(process.execPath, [create, "app"], {
      cwd: directory, encoding: "utf8",
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, npm_config_user_agent: `${manager}/99.0.0` }
    })
    assert.equal(result.status, 0, result.stderr)
    assert.deepEqual(JSON.parse(readFileSync(join(directory, "install.json"), "utf8")), {
      argv: ["install"], cwd: realpathSync(join(directory, "app")), ready: true
    })
    assert.match(result.stdout, new RegExp(`Installing dependencies with ${manager}`))
    assert.match(result.stdout, new RegExp(`${manager} run test`))
    assert.doesNotMatch(result.stdout, new RegExp(`cd app\\n  ${manager} install`))
  })
}

test("propagates install failure and leaves a recoverable scaffold", (t) => {
  const directory = scratch(t)
  const bin = join(directory, "bin")
  mkdirSync(bin)
  writeFileSync(join(bin, "npm"), `#!${process.execPath}\nprocess.exit(42)\n`, { mode: 0o755 })
  const result = spawnSync(process.execPath, [create, "app"], {
    cwd: directory, encoding: "utf8",
    env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, npm_config_user_agent: "npm/11" }
  })
  assert.equal(result.status, 42)
  assert.match(result.stderr, /npm install failed\. Run it again in app/)
  assert.ok(existsSync(join(directory, "app/src/main.vibe")))
  assert.doesNotMatch(result.stdout, /Next steps:/)
})

test("falls back to the host package manager for an unknown user agent", (t) => {
  const directory = scratch(t)
  const result = spawnSync(process.execPath, [create, "app", "--no-install"], {
    cwd: directory, encoding: "utf8", env: { ...process.env, npm_config_user_agent: "other/1" }
  })
  assert.equal(result.status, 0, result.stderr)
  assert.match(result.stdout, new RegExp(`  ${process.versions.bun ? "bun" : "npm"} install`))
})

test("a missing installer exits with failure and useful recovery instructions", (t) => {
  const directory = scratch(t)
  const result = spawnSync(process.execPath, [create, "app"], {
    cwd: directory, encoding: "utf8",
    env: { ...process.env, PATH: directory, npm_config_user_agent: "pnpm/99" }
  })
  assert.equal(result.status, 1)
  assert.match(result.stderr, /pnpm install failed\. Run it again in app/)
})

test("the packaged template restores dotfiles and excludes generated artifacts", (t) => {
  const directory = scratch(t)
  const packaged = join(directory, "package")
  mkdirSync(join(packaged, "bin"), { recursive: true })
  cpSync(create, join(packaged, "bin/create-vibelang.mjs"))
  cpSync(new URL("../package.json", import.meta.url), join(packaged, "package.json"))
  cpSync(new URL("../../../templates/app", import.meta.url), join(packaged, "template"), { recursive: true })
  for (const ignored of ["node_modules", "dist", ".prototype"]) {
    mkdirSync(join(packaged, "template", ignored), { recursive: true })
    writeFileSync(join(packaged, "template", ignored, "excluded.txt"), "generated")
  }
  writeFileSync(join(packaged, "template/bun.lock"), "generated lock")
  writeFileSync(join(packaged, "template/packaged-only.txt"), "published template")
  const result = spawnSync(process.execPath, [join(packaged, "bin/create-vibelang.mjs"), "app", "--no-install"], {
    cwd: directory, encoding: "utf8"
  })
  assert.equal(result.status, 0, result.stderr)
  const app = join(directory, "app")
  assert.equal(readFileSync(join(app, "packaged-only.txt"), "utf8"), "published template")
  for (const name of ["gitignore", "gitattributes"]) {
    assert.ok(existsSync(join(app, `.${name}`)))
    assert.ok(!existsSync(join(app, name)))
  }
  for (const ignored of ["node_modules", "dist", ".prototype", "bun.lock"]) {
    assert.ok(!existsSync(join(app, ignored)), ignored)
  }
})
