// Pack only the three existing packages the Linux worker needs. Never publishes to npm.
// Run after scripts/build-vibelang.sh and the runtime build, or in the Docker build stage.
import { execFileSync } from "node:child_process"
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = fileURLToPath(new URL("../../", import.meta.url))
const out = resolve(process.argv[2] ?? "dist-npm/tarballs")
const scratch = mkdtempSync(join(tmpdir(), "vibelang-worker-pack-"))
const manifest = JSON.parse(readFileSync(join(root, "packages/compiler/package.json"), "utf8"))
mkdirSync(out, { recursive: true })
try {
  for (const name of ["vibelang", "vibelang-runtime", "vibelang-linux-x64"]) {
    const dir = join(scratch, name)
    mkdirSync(dir)
    if (name === "vibelang-linux-x64") {
      mkdirSync(join(dir, "bin"))
      cpSync(join(root, "packages/compiler/native/vibelang"), join(dir, "bin/vibelang"))
      writeFileSync(join(dir, "package.json"), JSON.stringify({ name, version: manifest.version, os: ["linux"], cpu: ["x64"], files: ["bin"], license: "Apache-2.0" }))
    } else {
      const source = join(root, "packages", name === "vibelang" ? "compiler" : "runtime")
      for (const path of ["package.json", "README.md", name === "vibelang" ? "bin" : "dist"]) {
        cpSync(join(source, path), join(dir, path), { recursive: true })
      }
    }
    cpSync(join(root, name === "vibelang-runtime" ? "LICENSE" : "typescript/LICENSE.txt"), join(dir, "LICENSE"))
    execFileSync("npm", ["pack", dir, "--pack-destination", out, "--silent"], { stdio: "inherit" })
  }
} finally {
  rmSync(scratch, { recursive: true, force: true })
}
