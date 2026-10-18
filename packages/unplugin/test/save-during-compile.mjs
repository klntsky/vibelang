// A compiler for the watch test: runs VIBELANG_REAL, then saves src/shop.vibe
// once, the first time the compiled source says "booked", to change it to
// "sold". The save lands after the compiler read the sources and before the
// plugin uses its output, as when a user saves during a compile.
import { spawnSync } from "node:child_process"
import { existsSync, readFileSync, writeFileSync } from "node:fs"

const result = spawnSync(process.env.VIBELANG_REAL, process.argv.slice(2), { stdio: "inherit" })
const file = "src/shop.vibe"
const source = readFileSync(file, "utf8")
if (source.includes('receipt("booked"') && !existsSync("saved")) {
  writeFileSync("saved", "")
  writeFileSync(file, source.replace('receipt("booked"', 'receipt("sold"'))
}
process.exit(result.status ?? 1)
