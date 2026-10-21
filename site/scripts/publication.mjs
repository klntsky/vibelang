import { readdirSync, readFileSync, writeFileSync } from "node:fs"
import { join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
export function publicationText(text, site) {
  // Vocs embeds absolute build-root and cache-directory metadata in its bundles
  // and search document IDs. These are build metadata, never page content.
  text = text.replaceAll(site + "/", "./").replaceAll(site, ".")
  // Shipped playground library examples: replace their synthetic home directory.
  text = text.replaceAll("/home/user/", "/example/user/")
  // Synthetic playground IP examples use documentation-only addresses.
  text = text.replace(/(["\x27`])192\.168\.1\.42\1,(\s*)(["\x27`])10\.0\.0\.134\3/g,
    (_, quote, space, nextQuote) => quote + "192.0.2.42" + quote + "," + space + nextQuote + "192.0.2.134" + nextQuote)
  // CSS logo path from the playground icon set: space-separated SVG numbers
  // avoid looking like a LAN address without changing the path geometry.
  return text.replaceAll("-.232-2.718 10.059.003.23-2.622", "-.232-2.718 10.059 .003 .23-2.622")
}
const site = fileURLToPath(new URL("../", import.meta.url)).replace(/\/$/, "")
function normalize(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const file = join(dir, entry.name)
    if (entry.isDirectory()) normalize(file)
    else if (/\.(?:js|json)$/.test(file)) {
      const original = readFileSync(file, "utf8")
      const text = publicationText(original, site)
      if (text !== original) writeFileSync(file, text)
    }
  }
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) normalize(join(site, "dist"))
