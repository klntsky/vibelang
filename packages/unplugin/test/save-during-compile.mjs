// A compiler for the watch test: runs EFFECK_REAL, then saves src/shop.effeck
// once, the first time the compiled source says "booked", to change it to
// "sold". The save lands after the compiler read the sources and before the
// plugin uses its output, as when a user saves during a compile.
import { spawnSync } from "node:child_process"
import { existsSync, readFileSync, writeFileSync } from "node:fs"

const result = spawnSync(process.env.EFFECK_REAL, process.argv.slice(2), { stdio: "inherit" })
const file = "src/shop.effeck"
const source = readFileSync(file, "utf8")
if (source.includes('receipt("booked"') && !existsSync("saved")) {
  writeFileSync("saved", "")
  writeFileSync(file, source.replace('receipt("booked"', 'receipt("sold"'))
}
process.exit(result.status ?? 1)
