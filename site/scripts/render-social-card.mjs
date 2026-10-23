// Reproducible SVG-to-PNG render: pinned resvg, outlined text, no system fonts.
import { readFileSync, writeFileSync } from 'node:fs'
import { Resvg } from '@resvg/resvg-js'

const source = new URL('../public/social-card.svg', import.meta.url)
const output = new URL('../public/social-card.png', import.meta.url)
const png = new Resvg(readFileSync(source), {
  font: { loadSystemFonts: false },
}).render().asPng()
if (png.length >= 300_000) throw new Error('Social card must be under 300 KB')
writeFileSync(output, png)
console.log(`social-card: 1200x630, ${png.length} bytes`)
