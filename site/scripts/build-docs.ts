import { mkdirSync, readFileSync, writeFileSync } from "node:fs"
import { resolve } from "node:path"
import { kernelFacts } from "../../docs/tools/kernel-facts"
import { buildMarkdown } from "../../docs/tools/markdown"
const site = resolve(import.meta.dir, "..")
const factsFile = resolve(site, "../.cache/site-facts.json")
if (process.argv[2] === "--markdown") {
  buildMarkdown(resolve(site, "dist/public"), JSON.parse(readFileSync(factsFile, "utf8")))
} else {
  mkdirSync(resolve(site, "../.cache"), { recursive: true })
  writeFileSync(factsFile, JSON.stringify(kernelFacts()))
  for (const args of [
    ["scripts/sync-spec-docs.mjs", "../docs", "--facts", factsFile],
    ["scripts/sync-user-docs.mjs", "../docs/user"],
    ...(process.argv.includes("--sources") ? [] : [["scripts/patch-vocs-search.mjs"]]),
  ]) {
    const result = Bun.spawnSync(["node", ...args], { cwd: site, stdout: "inherit", stderr: "inherit" })
    if (result.exitCode) process.exit(result.exitCode)
  }
}
