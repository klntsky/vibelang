import { spawnSync } from "node:child_process"
import { mkdirSync, symlinkSync, existsSync, lstatSync, readlinkSync, rmSync } from "node:fs"
import { resolve, join } from "node:path"
import { fileURLToPath } from "node:url"
process.chdir(fileURLToPath(new URL(".", import.meta.url)))
// The checkout defaults to the repository this directory lives in.
const checkout = resolve(process.env.VIBELANG_CHECKOUT ?? "../..")
const runtime = resolve("node_modules/vibelang-runtime")
mkdirSync("node_modules", { recursive: true })
// Link the selected checkout's runtime, replacing a link left by a run against another checkout.
const target = join(checkout, "packages/runtime")
const linked = (() => { try { return lstatSync(runtime).isSymbolicLink() ? readlinkSync(runtime) : "not a link" } catch { return undefined } })()
if (linked !== target) {
  if (linked !== undefined) rmSync(runtime, { recursive: true, force: true })
  symlinkSync(target, runtime, "dir")
}
function run(command, args) {
  const result = spawnSync(command, args, { encoding: "utf8" })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(result.stdout + result.stderr)
  return result.stdout.trim()
}
const compiler = join(checkout, "packages/compiler/native/vibelang")
if (!existsSync(compiler)) throw new Error(`${compiler} is missing: run sh scripts/setup.sh in ${checkout} first`)
console.log(run(compiler, ["--version"]))
run(compiler, ["-p", "tsconfig.json"])
run(process.execPath, ["node_modules/typescript/bin/tsc", "-p", "tsconfig.effect.json"])
for (const file of ["dist/users.js", "dist-ts/users.js"]) {
  const output = run(process.execPath, [file])
  if (output !== "Ada Guest") throw new Error(`${file}: unexpected output ${output}`)
  console.log(`${file}: ${output}`)
}
