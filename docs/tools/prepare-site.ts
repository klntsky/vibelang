// Run from the repository root: bun docs/tools/prepare-site.ts <site dir>
//
// Prepares the assembled docs site for publication. The private repository's
// address works only inside the repository, so the published copy names the
// repository without it. render-user.ts already drops links to repository
// files, such as the upstream issue drafts, that are not pages. The script
// adds a 404 page, then refuses to publish local paths, the build host, LAN
// addresses, the private repository, or any credential value present in the
// environment. It never prints a credential.
import { readdirSync, readFileSync, writeFileSync } from "node:fs"
import { join, relative } from "node:path"
import { leaks } from "./leaks"

const site = process.argv[2]
if (!site) throw new Error("usage: bun docs/tools/prepare-site.ts <site dir>")

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e => e.isDirectory() ? files(join(dir, e.name)) : [join(dir, e.name)])
}

// Each rewrite has an HTML form and a Markdown form, for the pages' .md copies.
const rewrites: [RegExp, string][] = [
  [/<code>github\.com\/roninjin10\/effectscript<\/code>/g, "<code>effectscript</code> on GitHub"],
  [/`github\.com\/roninjin10\/effectscript`/g, "`effectscript` on GitHub"],
]

const notFound = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Page not found · Effeck Docs</title>
<style>
:root { color-scheme: light dark; --bg: #fbfbf9; --fg: #1d1d1f; --muted: #5d6066; --link: #1f5fbf; }
@media (prefers-color-scheme: dark) { :root { --bg: #15161a; --fg: #e6e4df; --muted: #a2a3a8; --link: #7fb0ff; } }
body { margin: 0; background: var(--bg); color: var(--fg); font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; }
main { max-width: 640px; margin: 15vh auto; padding: 0 16px; }
p { color: var(--muted); }
a { color: var(--link); }
</style>
</head>
<body>
<main>
<h1>Page not found</h1>
<p>This page does not exist. Start from the <a href="/">Effeck Docs</a> or read the <a href="/spec/">specification</a>.</p>
</main>
</body>
</html>
`
writeFileSync(join(site, "404.html"), notFound)

let problems = 0
const all = files(site)
for (const file of all) {
  if (!/\.(?:html|css|js|json|svg|txt|md|xml)$/.test(file)) continue
  let text = readFileSync(file, "utf8")
  for (const [pattern, replacement] of rewrites) text = text.replace(pattern, replacement)
  writeFileSync(file, text)
  for (const problem of leaks(text)) {
    problems++
    console.error(`${relative(site, file)}: ${problem}`)
  }
}
if (problems) {
  console.error(`prepare-site: ${problems} problems; fix the sources before publishing`)
  process.exit(1)
}
console.log(`prepare-site: ${all.length} files ready in ${site}`)
