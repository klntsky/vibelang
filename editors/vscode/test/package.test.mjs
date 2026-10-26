// bun run package builds the real archive and runs these checks.
import assert from "node:assert/strict"
import { execFileSync } from "node:child_process"
import { readFileSync, readdirSync } from "node:fs"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"
import { build } from "esbuild"

const root = fileURLToPath(new URL("../", import.meta.url))
const packed = (path) => execFileSync("unzip", ["-p", join(root, "vibelang.vsix"), "extension/" + path])
const notices = packed("ThirdPartyNotices.txt").toString()

test("the VSIX ships the complete VibeLang MIT license", () => {
  // vsce normalizes a source LICENSE without an extension to LICENSE.txt.
  const license = packed("LICENSE.txt").toString()
  const manifest = execFileSync("unzip", ["-p", join(root, "vibelang.vsix"), "extension.vsixmanifest"], { encoding: "utf8" })
  assert.match(manifest, /<License>extension\/LICENSE\.txt<\/License>/)
  assert.equal(license.split("\n")[0], "MIT License")
  const source = readFileSync(join(root, "../../LICENSE"), "utf8")
  const heading = /^MIT License\r?$/m.exec(source)
  assert.ok(heading, "source has a standalone MIT heading")
  assert.equal(license, source.slice(heading.index))
})

test("the VSIX preserves the grammar and language configuration notices", () => {
  const source = readFileSync(join(root, "scripts/grammar-notices.txt"), "utf8").trim()
  assert.ok(source.length > 0)
  assert.ok(notices.includes(source), "VSIX lacks the complete grammar and configuration notices")
})

test("the VSIX preserves license and notice files for every dependency in its actual bundle", async () => {
  // Independently rebuild and compare with the archive, so this metadata describes
  // the shipped JavaScript rather than an unrelated or stale dependency list.
  const result = await build({
    absWorkingDir: root, entryPoints: ["src/extension.ts"], bundle: true,
    platform: "node", format: "cjs", target: "node20", external: ["vscode"],
    sourcemap: true, outfile: "dist/extension.js", metafile: true, write: false
  })
  const bundle = result.outputFiles.find((file) => file.path.endsWith("/extension.js"))
  assert.deepEqual(packed("dist/extension.js"), Buffer.from(bundle.contents))
  const output = Object.entries(result.metafile.outputs).find(([file]) => file.endsWith("/extension.js"))[1]
  const packages = new Set()
  for (const [input, contribution] of Object.entries(output.inputs)) {
    if (contribution.bytesInOutput === 0) continue
    const match = input.match(/^(.*node_modules\/(?:@[^/]+\/)?[^/]+)\//)
    if (match) packages.add(match[1])
  }
  const names = []
  for (const dir of packages) {
    const pkg = JSON.parse(readFileSync(join(root, dir, "package.json"), "utf8"))
    names.push(pkg.name)
    assert.ok(notices.includes(pkg.name + "@" + pkg.version + " (" + pkg.license + ")"), "missing " + pkg.name + "@" + pkg.version)
    const files = readdirSync(join(root, dir), { withFileTypes: true })
      .filter((file) => file.isFile() && /^(licen[cs]e|notice|copying)(?:[._-]|$)/i.test(file.name))
      .map((file) => file.name)
    assert.ok(files.some((name) => /^(licen[cs]e|copying)(?:[._-]|$)/i.test(name)), pkg.name + ": no upstream license checked")
    for (const file of files) {
      const text = readFileSync(join(root, dir, file), "utf8").trim()
      assert.ok(text.length > 0, pkg.name + ": upstream " + file + " is empty")
      assert.ok(notices.includes(text), pkg.name + ": VSIX lacks the complete " + file)
    }
  }
  for (const name of ["semver", "minimatch", "brace-expansion", "balanced-match"]) {
    assert.ok(names.includes(name), name + ": review reproduction must remain covered")
  }
  assert.ok(!notices.includes("esbuild@"), "build tools must not be listed as bundled dependencies")
})
