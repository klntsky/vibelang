import assert from "node:assert/strict"
import { readFileSync } from "node:fs"

const site = readFileSync(new URL("../../../site/public/icon.svg", import.meta.url), "utf8")
const paths = (svg) => [...svg.matchAll(/<path\b[^>]*\bd="([^"]+)"/g)].map((match) => match[1])
const stops = (svg) => [...svg.matchAll(/<stop\b[^>]*\/?>/g)].map((match) => match[0])

export function assertLanguageIcons(manifest, read) {
  const languages = manifest.contributes.languages.filter((language) => language.extensions.includes(".vibe"))
  assert.equal(languages.length, 1, "one language owns .vibe files")
  for (const theme of ["light", "dark"]) {
    const file = languages[0].icon?.[theme]
    assert.ok(file, `${theme}: the language declares an icon`)
    const svg = read(file)
    assert.doesNotMatch(svg, /<text\b/i, `${theme}: the icon must use paths, not text`)
    assert.doesNotMatch(svg, /EK/, `${theme}: no old-name initials`)
    assert.deepEqual(paths(svg), paths(site), `${theme}: use the site mark path`)
    assert.deepEqual(stops(svg), stops(site), `${theme}: use the site gradient`)
    assert.doesNotMatch(svg, /<rect\b/i, `${theme}: keep the background transparent`)
  }
}
