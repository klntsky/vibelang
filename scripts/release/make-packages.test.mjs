// Local release assembly only: npm pack runs, npm publish never does.
import assert from "node:assert/strict"
import { execFileSync, spawnSync } from "node:child_process"
import { createHash } from "node:crypto"
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const repo = fileURLToPath(new URL("../../", import.meta.url))
const platforms = ["darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64", "win32-arm64", "win32-x64"]
const json = (file) => JSON.parse(readFileSync(file, "utf8"))
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "vibelang-release-test-"))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  for (const path of ["scripts/release/make-packages.mjs", "LICENSE", "typescript/LICENSE.txt", "typescript/NOTICE.txt", "typescript/tsc/internal/vibelang/identity.go",
    ...["compiler", "runtime", "vite", "create-vibelang", "unplugin"].flatMap((name) => [`packages/${name}/package.json`, `packages/${name}/README.md`]),
    "packages/compiler/bin", "packages/compiler/project.mjs", "packages/compiler/project.d.mts", "packages/compiler/register.mjs", "packages/vite/src", "packages/create-vibelang/bin", "packages/unplugin/src", "templates/app"]) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    cpSync(join(repo, path), join(root, path), { recursive: true, filter: (file) => !["node_modules", "dist", ".prototype"].includes(file.split("/").at(-1)) })
  }
  for (const path of ["packages/runtime/dist/index.js", "packages/runtime/dist/index.d.ts", "packages/runtime/dist/kernel/kernel.js", "editors/vscode/vibelang.vsix"]) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), `fixture ${path}\n`)
  }
  for (const platform of platforms) {
    const binary = join(root, "dist-npm/bin", platform, platform.startsWith("win32") ? "vibelang.exe" : "vibelang")
    mkdirSync(dirname(binary), { recursive: true })
    writeFileSync(binary, Buffer.from(`binary ${platform}\0\xff`))
  }
  return root
}
function assemble(root) {
  return spawnSync(process.execPath, [join(root, "scripts/release/make-packages.mjs"), "--skip-build"], { encoding: "utf8", timeout: 120_000 })
}
function archive(root, name, path) {
  const version = json(join(root, "packages/compiler/package.json")).version
  return execFileSync("tar", ["-xOzf", join(root, "dist-npm/tarballs", `${name}-${version}.tgz`), `package/${path}`])
}

test("packs every file the compiler package lists and every target its exports name", { timeout: 120_000 }, (t) => {
  const root = fixture(t)
  const result = assemble(root)
  assert.equal(result.status, 0, result.stderr)
  const compiler = json(join(root, "packages/compiler/package.json"))
  const targets = Object.values(compiler.exports).flatMap((target) => typeof target === "string" ? [target] : Object.values(target))
  for (const path of [...targets.map((target) => target.replace(/^\.\//, "")), ...compiler.files.filter((file) => file !== "bin" && file !== "editor")]) {
    assert.ok(archive(root, "vibelang", path).length > 0, `vibelang tarball lacks ${path}`)
  }
})

test("assembles every package in publish order with pinned versions, platform binaries and portable templates", { timeout: 120_000 }, (t) => {
  const root = fixture(t)
  const result = assemble(root)
  assert.equal(result.status, 0, result.stderr)
  const manifest = json(join(root, "dist-npm/tarballs/manifest.json"))
  const names = [...platforms.map((p) => `vibelang-${p}`), "vibelang", "vibelang-runtime", "vibelang-vite", "create-vibelang", "vibelang-unplugin"]
  assert.deepEqual(manifest.packages.map((p) => p.name), names)
  for (const pkg of manifest.packages) {
    assert.equal(pkg.version, manifest.version)
    assert.equal(readFileSync(join(root, "dist-npm/tarballs", pkg.file)).length, pkg.size)
    assert.ok(pkg.files > 0)
    const packed = JSON.parse(archive(root, pkg.name, "package.json"))
    assert.equal(packed.name, pkg.name)
    assert.equal(packed.version, manifest.version)
  }
  for (const platform of platforms) {
    const [os, cpu] = platform.split("-")
    const name = `vibelang-${platform}`
    const pkg = JSON.parse(archive(root, name, "package.json"))
    assert.deepEqual(pkg.os, [os])
    assert.deepEqual(pkg.cpu, [cpu])
    const exe = os === "win32" ? "vibelang.exe" : "vibelang"
    assert.deepEqual(archive(root, name, `bin/${exe}`), readFileSync(join(root, "dist-npm/bin", platform, exe)))
    const mode = execFileSync("tar", ["-tvzf", join(root, "dist-npm/tarballs", `${name}-${manifest.version}.tgz`)], { encoding: "utf8" })
    assert.match(mode, /-rwxr-xr-x/)
  }
  const plugin = JSON.parse(archive(root, "vibelang", "dprint-plugin.json"))
  assert.equal(plugin.kind, "process")
  assert.equal(plugin.name, "dprint-plugin-vibelang")
  assert.equal(plugin.version, manifest.version)
  const dprintNames = { "darwin-arm64": "darwin-aarch64", "darwin-x64": "darwin-x86_64", "linux-arm64": "linux-aarch64", "linux-x64": "linux-x86_64", "win32-arm64": "windows-aarch64", "win32-x64": "windows-x86_64" }
  for (const platform of platforms) {
    const exe = platform.startsWith("win32") ? "vibelang.exe" : "vibelang"
    const tarball = readFileSync(join(root, "dist-npm/tarballs", `vibelang-${platform}-${manifest.version}.tgz`))
    assert.deepEqual(plugin[dprintNames[platform]], {
      reference: `npm:vibelang-${platform}@${manifest.version}/bin/${exe}`,
      checksum: createHash("sha256").update(tarball).digest("hex")
    })
  }
  const launcherFiles = execFileSync("tar", ["-tzf", join(root, "dist-npm/tarballs", `vibelang-${manifest.version}.tgz`)], { encoding: "utf8" })
  assert.doesNotMatch(launcherFiles, /package\/native\//)
  assert.deepEqual(archive(root, "vibelang", "editor/vibelang.vsix"), readFileSync(join(root, "editors/vscode/vibelang.vsix")))
  const app = JSON.parse(archive(root, "create-vibelang", "template/package.json"))
  assert.equal(app.dependencies["vibelang-runtime"], manifest.version)
  assert.equal(app.devDependencies.vibelang, manifest.version)
  assert.equal(app.devDependencies["vibelang-vite"], manifest.version)
  assert.ok(archive(root, "create-vibelang", "template/gitignore").length)
  assert.ok(archive(root, "create-vibelang", "template/gitattributes").length)
  assert.match(archive(root, "vibelang", "NOTICE").toString(), /Microsoft/)
  assert.match(archive(root, "vibelang-runtime", "LICENSE").toString(), /MIT License/)
})

test("reports inconsistent versions, pins and missing binaries together before replacing release artifacts", (t) => {
  const root = fixture(t)
  const runtimePath = join(root, "packages/runtime/package.json")
  const runtime = json(runtimePath)
  runtime.version = "99.0.0"
  writeFileSync(runtimePath, JSON.stringify(runtime))
  const compilerPath = join(root, "packages/compiler/package.json")
  const compiler = json(compilerPath)
  compiler.optionalDependencies["vibelang-linux-x64"] = "99.0.0"
  delete compiler.optionalDependencies["vibelang-win32-x64"]
  writeFileSync(compilerPath, JSON.stringify(compiler))
  rmSync(join(root, "dist-npm/bin/darwin-arm64"), { recursive: true })
  mkdirSync(join(root, "dist-npm/tarballs"))
  const previous = join(root, "dist-npm/tarballs/manifest.json")
  writeFileSync(previous, "previous release")
  const result = assemble(root)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /vibelang-runtime: version 99\.0\.0/)
  assert.match(result.stderr, /optionalDependencies must list exactly/)
  assert.match(result.stderr, /vibelang-linux-x64 pinned to 99\.0\.0/)
  assert.match(result.stderr, /missing dist-npm\/bin\/darwin-arm64/)
  assert.equal(readFileSync(previous, "utf8"), "previous release")
})

for (const missing of ["editors/vscode/vibelang.vsix", "packages/runtime/dist/index.js"]) {
  test(`--skip-build refuses a missing ${missing} before replacing release artifacts`, (t) => {
    const root = fixture(t)
    rmSync(join(root, missing))
    const result = assemble(root)
    assert.equal(result.status, 1)
    assert.ok(result.stderr.includes(missing), result.stderr)
    assert.match(result.stderr, /run without --skip-build/)
    assert.ok(!existsSync(join(root, "dist-npm/tarballs")))
  })
}
