// EFFECK_LANDING=1 bun docs/tools/render-user.ts, then bun docs/tools/check-landing.ts [--online]
// Checks the assembled Monday landing without making any first-load requests.
// --online also checks the external navigation destination, without credentials.
import { existsSync, readFileSync } from "node:fs"
import { join, resolve } from "node:path"
import { strict as assert } from "node:assert"

const repo = resolve(import.meta.dir, "../..")
const site = join(repo, ".cache/docs-site")
const html = readFileSync(join(site, "index.html"), "utf8")
const hero = JSON.parse(readFileSync(join(repo, "docs/site/converter/hero.json"), "utf8"))
const decode = (text: string) => text.replace(/&(amp|lt|gt|quot|#39);/g, (_, entity: string) =>
  ({ amp: "&", lt: "<", gt: ">", quot: '"', "#39": "'" })[entity]!)

assert(html.includes('class="front-h1">Effect, as a language.</h1>'), "index.html must be the landing")
assert(readFileSync(join(site, "001-onboarding.html"), "utf8").includes("Welcome to Effeck"), "onboarding must keep its path")
assert(html.includes('<meta name="converter" content="off">'), "converter must default off")
assert(!/<!-- hero:|<!-- \/?source-link|contenteditable|Powered by Smithers|Report a bug|Getting started|Convert button/i.test(html))
// By default (EFFECK_LANDING_SOURCE unset) the landing links to no private repository.
if (!process.env.EFFECK_LANDING_SOURCE) assert(!html.includes('class="source-link"'), "no source link by default")
assert(readFileSync(join(site, "001-onboarding.html"), "utf8").includes('<a class="brand" href="001-onboarding.html">'), "the docs brand opens the docs, not the landing")
assert(html.includes('value="npm create effeck@latest my-app" readonly'), "command must be exact and readonly")
assert(/<button[^>]*id="copy-button"[^>]*>[\s\S]*?<span[^>]*>Copy<\/span><\/button>/.test(html), "Copy must be a button")
// The page runs exactly one inline script, the Copy button's, and its CSP allows it by hash alone.
const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map(m => m[1]!)
assert.equal(scripts.length, 1, "the landing runs exactly one inline script")
assert(!/<script\s+src/i.test(html), "the landing loads no script file")
const digest = new Bun.CryptoHasher("sha256").update(scripts[0]!).digest("base64")
assert(html.includes(`script-src 'sha256-${digest}'`), "the CSP must allow the Copy script by its hash")
for (const side of ["effect", "effeck"] as const) {
  const panel = new RegExp(`id="${side}-panel"[\\s\\S]*?</section>`).exec(html)?.[0]
  assert(panel !== undefined, `missing ${side} code`)
  const clean = panel.replace(/<span class="front-tip-body"[\s\S]*?<\/code><span>[\s\S]*?<\/span><\/span>/g, "")
  const lines = [...clean.matchAll(/<span class="front-code-src">([\s\S]*?)\n<\/span><\/span>/g)].map(m => decode(m[1].replace(/<[^>]*>/g, "")))
  assert.equal(lines.join("\n") + "\n", hero[side], `${side} must match hero.json exactly`)

}
console.log("PASS: index.html = landing; 001-onboarding.html = onboarding; both hero panes match hero.json")

const hrefs: string[] = []
const resources: string[] = []
await new HTMLRewriter().on("*", {
  element(element) {
    const tag = element.tagName
    for (const [name, value] of element.attributes) {
      assert(!name.startsWith("on"), `inline event handler: ${name}`)
      if (tag === "a" && name === "href") hrefs.push(decode(value))
      if (["src", "srcset", "poster"].includes(name) || (tag === "link" && name === "href") || (tag === "object" && name === "data")) {
        if (!value.startsWith("data:")) resources.push(`${tag}[${name}]=${value}`)
      }
    }
  }
}).transform(new Response(html)).text()
const css = [...html.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map(m => m[1]).join("\n")
assert(!/@import\b|url\s*\(\s*(?!data:)/i.test(css), "CSS must not load resources")
assert.deepEqual(resources, [], "first load must not fetch resources")
const sourceLinks: Record<string, string[]> = {
  none: [],
  smithers: ["https://smithers.sh/effeck/effeck?utm_source=effeck.dev&utm_medium=source&utm_campaign=effeck-launch"],
  github: [process.env.EFFECK_LANDING_GITHUB_URL ?? ""],
}
assert.deepEqual(hrefs, Array(2).fill(["/001-onboarding.html", ...sourceLinks[process.env.EFFECK_LANDING_SOURCE ?? "none"]!]).flat())
console.log("PASS: first-load resource requests = 0 (source audit: one hashed inline script, embedded fonts, no remote assets or CSS imports)")

let failures = 0
for (const href of hrefs) {
  const url = new URL(href, "https://effeck.dev")
  if (url.origin === "https://effeck.dev") {
    assert(existsSync(join(site, url.pathname)), `missing local link: ${href}`)
    console.log(`PASS: ${href} resolves in the built site`)
  } else if (process.argv.includes("--online")) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(15000), redirect: "follow", credentials: "omit" })
      assert(response.ok, `HTTP ${response.status}`)
      const body = await response.text()
      assert(!/isn(?:'|’|&#39;|&apos;)t on Smithers yet/i.test(body), "repo is not publicly readable")
      console.log(`PASS: ${href} → HTTP ${response.status} (public repo visibility still needs a signed-out browser)`)
    } catch (error) {
      failures++
      console.error(`FAIL: ${href} → ${error instanceof Error ? error.message : "request failed"}`)
    }
  } else console.log(`SKIP: external HTTP check for ${href} (run with --online)`)
}
if (failures) process.exitCode = 1
