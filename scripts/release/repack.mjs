#!/usr/bin/env node
// Repack only vibelang's existing tarball when VIBELANG_SKIP leaves platforms out.
// No build or staging tree is used. All payloads except package.json must stay
// byte-identical, including the launcher and the packaged editor extension.
//
//   VIBELANG_SKIP=win32 node scripts/release/repack.mjs [tarball-directory]
import { deepStrictEqual } from "node:assert/strict"
import { execFileSync } from "node:child_process"
import { createHash } from "node:crypto"
import { lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, renameSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

function payload(dir) {
  return readdirSync(dir, { recursive: true }).sort().flatMap((path) => {
    const file = join(dir, path)
    const stat = lstatSync(file)
    if (stat.isDirectory()) return []
    return [{ path, mode: stat.mode & 0o777, content: stat.isSymbolicLink() ? readlinkSync(file) : readFileSync(file) }]
  })
}

export function repack(tarballs, skip = process.env.VIBELANG_SKIP ?? "") {
  tarballs = resolve(tarballs)
  const manifestPath = join(tarballs, "manifest.json")
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"))
  const entry = manifest.packages.find((pkg) => pkg.name === "vibelang")
  if (!entry) throw new Error("repack: manifest.json has no vibelang package")
  const tarball = join(tarballs, entry.file)
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-repack-"))
  try {
    execFileSync("tar", ["-xzf", tarball, "-C", scratch])
    const dir = join(scratch, "package")
    const packagePath = join(dir, "package.json")
    const pkg = JSON.parse(readFileSync(packagePath, "utf8"))
    if (pkg.name !== "vibelang" || pkg.version !== manifest.version || pkg.version !== entry.version) {
      throw new Error(`repack: expected vibelang@${manifest.version}, got ${pkg.name}@${pkg.version}`)
    }
    const patterns = skip.split(/[,\s]+/).filter(Boolean)
    const removed = Object.keys(pkg.optionalDependencies ?? {}).filter((name) => patterns.some((pattern) => name.includes(pattern)))
    for (const name of removed) delete pkg.optionalDependencies[name]
    if (removed.length) {
      writeFileSync(packagePath, JSON.stringify(pkg, null, 2) + "\n")
      const expected = payload(dir)
      const packedDir = join(scratch, "packed")
      mkdirSync(packedDir)
      const [packed] = JSON.parse(execFileSync("npm", ["pack", dir, "--pack-destination", packedDir, "--ignore-scripts", "--json", "--silent"], { encoding: "utf8" }))
      if (packed.name !== pkg.name || packed.version !== pkg.version || packed.filename !== entry.file) {
        throw new Error("repack: npm pack changed the package identity")
      }
      const repacked = join(packedDir, packed.filename)
      const check = join(scratch, "check")
      mkdirSync(check)
      execFileSync("tar", ["-xzf", repacked, "-C", check])
      deepStrictEqual(payload(join(check, "package")), expected, "repack: npm pack changed package files or permissions")
      const sha256 = createHash("sha256").update(readFileSync(repacked)).digest("hex")
      Object.assign(entry, { size: packed.size, unpackedSize: packed.unpackedSize, files: packed.files.length, sha256 })
      // Move the verified tarball into place only after packing and validation.
      // Copy through the destination filesystem so TMPDIR can be another mount.
      const next = `${tarball}.repack`
      try {
        writeFileSync(next, readFileSync(repacked))
        renameSync(next, tarball)
      } finally {
        rmSync(next, { force: true })
      }
      writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n")
      console.log(`repack: removed ${removed.join(", ")} from vibelang@${pkg.version}`)
    }
    console.log(`repack: vibelang@${pkg.version} optionalDependencies: ${JSON.stringify(pkg.optionalDependencies ?? {})}`)
    console.log(`repack: ${entry.file} sha256 ${createHash("sha256").update(readFileSync(tarball)).digest("hex")}`)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  repack(process.argv[2] ?? fileURLToPath(new URL("../../dist-npm/tarballs/", import.meta.url)))
}
