import { expect, test } from "bun:test"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { checkSite } from "./check-site"

test("checks built routes, headings and self-contained images", async () => {
  const root = mkdtempSync(join(tmpdir(), "check-site-"))
  try {
    mkdirSync(join(root, "guide"))
    writeFileSync(join(root, "index.html"), '<a href="/guide#start">Guide</a>')
    writeFileSync(join(root, "guide/index.html"), '<h1 id="start">Guide</h1><a href="/">Home</a>')
    await checkSite(root)
    for (const html of ['<a href="/missing">Missing</a>', '<a href="/guide#missing">Missing</a>', '<img src="https://example.com/image.png">']) {
      writeFileSync(join(root, "index.html"), html)
      await expect(checkSite(root)).rejects.toThrow()
    }
  } finally { rmSync(root, { recursive: true, force: true }) }
})
