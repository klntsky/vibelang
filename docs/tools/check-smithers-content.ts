// Checks the repository content smithers.sh serves for Effeck: the guest chat's instructions
// (.smithers/instructions/app.md), the repo home (.smithers/home.json), the convert flow's prompt and
// the effeck.dev hero (docs/site/converter/hero.json). It blocks the docs deploy, because these files
// are what the converter, the repo home and the chat say about Effeck.
//
//   bun docs/tools/check-smithers-content.ts
//
// app.md: under 24 KiB; every link resolves to a docs page or a repository file; every Effeck block
// passes check-snippets.ts; the install block matches what create-effeck prints. home.json: the
// blocks smithers.sh accepts, with existing markdown paths. prompt.md: not stale. hero.json: the
// hero golden case, with matching hashes.
import { createHash } from "node:crypto"
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { spawnSync } from "node:child_process"

const repo = resolve(import.meta.dir, "../..")
export const appLimit = 24 * 1024

/** Maps a link in app.md to the repository file that proves it resolves, or a reason it cannot. */
export function linkTarget(url: string): { file?: string; ok?: true; error?: string } {
  let parsed: URL
  try { parsed = new URL(url) } catch { return { error: "not a URL" } }
  if (parsed.protocol !== "https:") return { error: "not https" }
  if (parsed.hostname === "effeck.dev") {
    if (parsed.pathname === "/" || parsed.pathname === "") return { ok: true }
    const page = parsed.pathname.replace(/^\//, "")
    if (!page.endsWith(".html")) return { error: "an effeck.dev page ends in .html" }
    const md = page.replace(/\.html$/, ".md")
    return page.startsWith("spec/") ? { file: join("docs/design/src", md.slice("spec/".length)) } : { file: join("docs/user/src", md) }
  }
  if (parsed.hostname === "smithers.sh") {
    const blob = /^\/effeck\/effeck\/blob\/main\/(.+)$/.exec(parsed.pathname)
    if (blob) return { file: decodeURIComponent(blob[1]) }
    if (parsed.pathname === "/effeck/effeck/issues" || parsed.pathname === "/effeck/effeck") return { ok: true }
  }
  return { error: "links go to effeck.dev pages or effeck/effeck on smithers.sh only" }
}

export function links(markdown: string): string[] {
  return [...new Set([...markdown.matchAll(/https:\/\/[^\s)<>`"]+/g)].map((m) => m[0].replace(/[.,;:]+$/, "")))]
}

/** The fenced blocks of a language under one `## ` section. */
export function sectionBlocks(markdown: string, heading: string, lang: string): string[] {
  const start = markdown.indexOf(`## ${heading}`)
  if (start < 0) return []
  const end = markdown.indexOf("\n## ", start + 1)
  const section = markdown.slice(start, end < 0 ? undefined : end)
  return [...section.matchAll(new RegExp("```" + lang + "\\n([\\s\\S]*?)```", "g"))].map((m) => m[1].trim())
}

/** What create-effeck prints for an npm user: the scaffold command and its next steps. */
export function createEffeckSteps(): { commands: string[]; editor: string[] } {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-create-"))
  try {
    // A relative target, as a reader types it: create-effeck echoes it in its `cd` step.
    const run = spawnSync("node", [join(repo, "packages/create-effeck/bin/create-effeck.mjs"), "my-app", "--no-install"], {
      cwd: scratch, encoding: "utf8", env: { ...process.env, npm_config_user_agent: "npm/11.0.0 node/v26.0.0" }
    })
    if (run.status !== 0) throw new Error(`create-effeck failed: ${run.stderr}`)
    const lines = run.stdout.split("\n").map((l) => l.trim())
    const next = lines.slice(lines.indexOf("Next steps:") + 1)
    // --no-install adds the install step; a real `npm create` runs it itself.
    const commands = ["npm create effeck@latest my-app", ...next.filter((l) => l.startsWith("cd ") || l.startsWith("npm run start")).map((l) => l.replace(/\s{2,}.*$/, ""))]
    const editor = lines.filter((l) => l.startsWith("code --install-extension"))
    return { commands, editor }
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
}

const homeTypes = new Set(["prompt", "app", "flows", "markdown", "text", "stack", "links", "view"])

/** The checks smithers.sh's homepage route and RepositoryHomeBlockSchema apply. */
export function homeProblems(document: string, exists: (path: string) => boolean): string[] {
  const problems: string[] = []
  if (Buffer.byteLength(document) > 128 * 1024) problems.push("home.json is over 128 KiB")
  let parsed: { blocks?: Array<Record<string, unknown>> }
  try { parsed = JSON.parse(document) } catch { return ["home.json is not JSON"] }
  if (!Array.isArray(parsed.blocks) || parsed.blocks.length === 0 || parsed.blocks.length > 32) return ["home.json needs 1 to 32 blocks"]
  for (const [i, block] of parsed.blocks.entries()) {
    const type = String(block.type)
    if (!homeTypes.has(type)) problems.push(`block ${i}: unknown type ${type}`)
    for (const key of ["title", "placeholder"] as const) {
      if (block[key] !== undefined && (typeof block[key] !== "string" || !(block[key] as string).length || (block[key] as string).length > 120)) problems.push(`block ${i}: ${key} must be 1-120 characters`)
    }
    if (type === "text" && (typeof block.text !== "string" || !block.text.length || block.text.length > 4096)) problems.push(`block ${i}: text must be 1-4096 characters`)
    if (type === "markdown") {
      const path = String(block.path ?? "")
      if (!path || path.startsWith("/") || path.split("/").some((p) => p === "" || p === "." || p === "..")) problems.push(`block ${i}: invalid markdown path ${path}`)
      else if (!exists(path)) problems.push(`block ${i}: markdown file is missing: ${path}`)
    }
    if (type === "links") {
      const items = block.links as Array<{ label?: string; url?: string }> | undefined
      if (!Array.isArray(items) || !items.length) problems.push(`block ${i}: links needs at least one link`)
      for (const item of items ?? []) {
        if (!item.label || item.label.length > 120) problems.push(`block ${i}: link label must be 1-120 characters`)
        try { if (!["https:", "http:"].includes(new URL(String(item.url)).protocol)) problems.push(`block ${i}: ${item.url} is not http(s)`) } catch { problems.push(`block ${i}: ${item.url} is not a URL`) }
      }
    }
  }
  return problems
}

const sha = (text: string) => `sha256:${createHash("sha256").update(text).digest("hex")}`

async function main() {
  const problems: string[] = []
  const at = (path: string) => join(repo, path)
  const app = readFileSync(at(".smithers/instructions/app.md"), "utf8")
  if (Buffer.byteLength(app) >= appLimit) problems.push(`app.md is ${Buffer.byteLength(app)} bytes; the limit is ${appLimit}`)
  for (const url of links(app)) {
    const target = linkTarget(url)
    if (target.error) problems.push(`app.md: ${url}: ${target.error}`)
    else if (target.file && !existsSync(at(target.file))) problems.push(`app.md: ${url} does not resolve (${target.file} is missing)`)
  }
  const { commands, editor } = createEffeckSteps()
  const [install, editorBlock] = sectionBlocks(app, "Install", "sh")
  if (install !== commands.join("\n")) problems.push(`app.md's install block must be:\n${commands.join("\n")}\nnot:\n${install ?? "(missing)"}`)
  for (const line of (editorBlock ?? "").split("\n")) if (line && !editor.includes(line)) problems.push(`app.md: "${line}" is not what create-effeck prints`)

  const json = mkdtempSync(join(tmpdir(), "effeck-content-")) + "/snippets.json"
  const snippets = spawnSync("bun", [at("docs/tools/check-snippets.ts"), "--root", at(".smithers/instructions"), "--json", json], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] })
  try {
    const report = JSON.parse(readFileSync(json, "utf8"))
    const blocks = report.pages.flatMap((p: { blocks: Array<{ status: string }> }) => p.blocks)
    const bad = blocks.filter((b: { status: string }) => b.status !== "ok")
    if (!blocks.length) problems.push("app.md has no checked Effeck block")
    if (bad.length) problems.push(`app.md: ${bad.length} snippet(s) do not pass check-snippets.ts:\n${snippets.stdout.slice(-1500)}`)
  } catch {
    problems.push(`check-snippets.ts did not report on app.md (exit ${snippets.status}):\n${(snippets.stdout + snippets.stderr).slice(-1500)}`)
  }

  problems.push(...homeProblems(readFileSync(at(".smithers/home.json"), "utf8"), (p) => existsSync(at(p))).map((p) => `home.json: ${p}`))

  const prompt = spawnSync("node", [at(".smithers/flows/convert/build-prompt.mjs"), "--check"], { encoding: "utf8" })
  if (prompt.status !== 0) problems.push(prompt.stderr.trim() || "prompt.md is stale")

  const hero = JSON.parse(readFileSync(at("docs/site/converter/hero.json"), "utf8"))
  const golden = ".smithers/flows/convert/golden/01-hero-batch-lookup/"
  if (hero.effect !== readFileSync(at(golden + "input.ts"), "utf8") || hero.effeck !== readFileSync(at(golden + "expected.effeck"), "utf8")) problems.push("hero.json differs from golden case 01")
  if (hero.receipt?.inputSha !== sha(hero.effect) || hero.receipt?.outputSha !== sha(hero.effeck)) problems.push("hero.json's receipt hashes do not match its code")

  if (problems.length) {
    console.error(problems.map((p) => `check-smithers-content: ${p}`).join("\n"))
    process.exit(1)
  }
  console.log(`check-smithers-content: app.md ${Buffer.byteLength(app)} bytes, ${links(app).length} links, install block, snippets, home.json, prompt.md and hero.json pass`)
}

if (import.meta.main) await main()
