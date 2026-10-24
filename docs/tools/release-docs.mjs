#!/usr/bin/env node
// Current-release references come from the compiler manifest; release notes
// and historical compiler-gap notes retain the versions they describe.
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs"
import { join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = fileURLToPath(new URL("../../", import.meta.url))
const semver = String.raw`\d+\.\d+\.\d+(?:-[\w.-]+)?`
const historical = /(?:^|\/)(?:release-notes|compiler-gaps)\.(?:md|mdx)$/

export function renderReleasePage(path, text, version) {
  if (historical.test(path)) return text
  let output = text
    .replace(new RegExp(String.raw`\b(vibelang(?:-[a-z0-9-]+)?@)` + semver, "g"), (_, prefix) => prefix + version)
    .replace(new RegExp(String.raw`\b((?:VibeLang|vibelang|[Vv]ersion|Rehearsed for|In) )` + semver, "g"), (_, prefix) => prefix + version)
    .replace(new RegExp('(vibelang@' + semver + ' version\\x60? and expect \\x60)' + semver + '(\\x60)', "g"), (_, prefix, suffix) => prefix + version + suffix)
  if (path === "site/src/pages/index.mdx") {
    output = output.replace(new RegExp("(<strong>)" + semver + "(</strong>)", "g"), (_, prefix, suffix) => prefix + version + suffix)
  }
  return output
}

export function releasePaths(repository) {
  const walk = path => !existsSync(path) ? [] : readdirSync(path, { withFileTypes: true }).flatMap(entry => {
    const file = join(path, entry.name)
    return entry.isDirectory() ? (["site", "node_modules", "notes"].includes(entry.name) ? [] : walk(file))
      : /\.(?:md|mdx)$/.test(entry.name) ? [file] : []
  })
  const packages = join(repository, "packages")
  return [...new Set([
    ...["docs/user", "docs/design/src", "site/src/pages"].flatMap(path => walk(join(repository, path))),
    ...["README.md", ".smithers/instructions/app.md"].map(path => join(repository, path)),
    ...(existsSync(packages) ? readdirSync(packages).map(name => join(packages, name, "README.md")) : [])
  ])].filter(existsSync).map(path => relative(repository, path).replaceAll("\\", "/")).sort()
}

export function releaseDocsProblems(repository = root, write = false) {
  const version = JSON.parse(readFileSync(join(repository, "packages/compiler/package.json"), "utf8")).version
  if (!new RegExp("^" + semver + "$").test(version)) throw new Error("Invalid compiler version: " + version)
  const problems = []
  for (const path of releasePaths(repository)) {
    const file = join(repository, path)
    const current = readFileSync(file, "utf8")
    const generated = renderReleasePage(path, current, version)
    if (current === generated) continue
    if (write && (!path.startsWith("site/src/pages/") || path === "site/src/pages/index.mdx")) writeFileSync(file, generated)
    else if (!write) problems.push(path + ": stale VibeLang version; expected " + version + " from packages/compiler/package.json")
  }
  return problems
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [mode] = process.argv.slice(2)
  if (!["--write", "--check"].includes(mode)) {
    console.error("usage: node docs/tools/release-docs.mjs --write | --check")
    process.exitCode = 2
  } else {
    const problems = releaseDocsProblems(root, mode === "--write")
    for (const problem of problems) console.error(problem)
    if (problems.length) {
      console.error("Run (cd .smithers && bun run docs:generate) to regenerate release docs and site copies.")
      process.exitCode = 1
    } else console.log("release-docs: current-release references match the compiler manifest")
  }
}
