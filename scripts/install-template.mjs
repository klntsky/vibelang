import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = fileURLToPath(new URL("../", import.meta.url))
const base = process.env.BUN_CREATE_DIR || join(process.env.HOME, ".bun-create")
const destination = resolve(base, "effectscript")
const marker = ".effectscript-template"
if (existsSync(destination) && !existsSync(join(destination, marker))) {
  console.error(`setup: refusing to replace an unrelated template at ${destination}`)
  process.exit(1)
}
mkdirSync(dirname(destination), { recursive: true })
rmSync(destination, { recursive: true, force: true })
cpSync(join(root, "templates/app"), destination, {
  recursive: true,
  filter: (path) => !["node_modules", "dist", ".prototype", "bun.lock"].includes(path.split("/").at(-1))
})
const packagePath = join(destination, "package.json")
const pkg = JSON.parse(readFileSync(packagePath, "utf8"))
pkg.dependencies["@effectscript/runtime"] = `file:${join(root, "packages/runtime")}`
pkg.devDependencies["@effectscript/compiler"] = `file:${join(root, "packages/compiler")}`
writeFileSync(packagePath, JSON.stringify(pkg, null, 2) + "\n")
writeFileSync(join(destination, marker), `${root}\n`)
console.log(`setup: installed template at ${destination}`)
