import { expect, test } from "bun:test"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { checkSite } from "./check-site"

const social = '<meta property="og:image" content="https://vibelang.sh/card.png"><meta name="twitter:image" content="https://vibelang.sh/card.png"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="630"><meta property="og:image:alt" content="VibeLang">'

test("checks built routes, headings and self-contained images", async () => {
  const root = mkdtempSync(join(tmpdir(), "check-site-"))
  try {
    mkdirSync(join(root, "guide"))
    writeFileSync(join(root, "card.png"), "image")
    writeFileSync(join(root, "index.html"), social + '<a href="/guide#start">Guide</a>')
    writeFileSync(join(root, "guide/index.html"), social + '<h1 id="start">Guide</h1><a href="/">Home</a>')
    await checkSite(root)
    for (const html of ['<a href="/missing">Missing</a>', '<a href="/guide#missing">Missing</a>', '<img src="https://example.com/image.png">']) {
      writeFileSync(join(root, "index.html"), social + html)
      await expect(checkSite(root)).rejects.toThrow()
    }
  } finally { rmSync(root, { recursive: true, force: true }) }
})

test("requires social metadata on every page and local HTTPS image targets", async () => {
  const root = mkdtempSync(join(tmpdir(), "check-social-"))
  try {
    writeFileSync(join(root, "card.png"), "image")
    writeFileSync(join(root, "index.html"), social)
    await checkSite(root)
    for (const key of ["og:image", "twitter:image", "og:image:width", "og:image:height", "og:image:alt"]) {
      writeFileSync(join(root, "index.html"), social.replace(new RegExp('<meta (?:property|name)="' + key + '"[^>]*>'), ''))
      await expect(checkSite(root)).rejects.toThrow(key)
    }
    for (const url of ["/card.png", "http://vibelang.sh/card.png", "https://vibelang.sh/missing.png"]) {
      writeFileSync(join(root, "index.html"), social.replaceAll("https://vibelang.sh/card.png", url))
      await expect(checkSite(root)).rejects.toThrow("og:image")
    }
    writeFileSync(join(root, "index.html"), social)
    writeFileSync(join(root, "404.html"), "No metadata")
    await expect(checkSite(root)).rejects.toThrow("404.html: og:image")
  } finally { rmSync(root, { recursive: true, force: true }) }
})
