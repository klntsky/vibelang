#!/usr/bin/env node
// Brand gate: the tree says VibeLang and nothing of the retired name. It fails
// on any spelling of the retired name (effeck, Effeck, EFFECK, .effeck) in a
// tracked file's text or path, so a stray from a concurrent lane, a merge, or a
// copied snippet is caught by `node --test scripts/brand-gate.test.mjs`.
//
// The exemptions are the codemod's own (scripts/rename-to-vibelang.mjs):
// artifact paths, external names we do not control yet (domains, the issue
// tracker repository, deployed resource names), and lines that carry
// `brand-gate: allow` or sit between `brand-gate: allow-start` and
// `brand-gate: allow-end`.
//
//   node scripts/brand-gate.mjs            # check this checkout
//   node scripts/brand-gate.mjs --root DIR # check another tree (the test uses it)
import { lstatSync, readFileSync, readlinkSync } from "node:fs"
import { join, resolve } from "node:path"
import { ALLOW_END, ALLOW_MARKER, ALLOW_START, RETIRED, isBinary, isExempt, listFiles, maskKept } from "./rename-to-vibelang.mjs"

const args = process.argv.slice(2)
const rootIndex = args.indexOf("--root")
const root = resolve(rootIndex >= 0 ? args[rootIndex + 1] : process.cwd())

const spellings = (text) => [...new Set(text.match(RETIRED) ?? [])]

const findings = []
for (const rel of listFiles(root)) {
  if (isExempt(rel)) continue
  const named = spellings(maskKept(rel, rel).masked)
  if (named.length > 0) findings.push(`${rel}: the path spells ${named.join(", ")}`)
  const path = join(root, rel)
  // A tracked symlink stores its target as text, including links to directories.
  const buffer = lstatSync(path).isSymbolicLink() ? Buffer.from(readlinkSync(path)) : readFileSync(path)
  if (isBinary(rel, buffer)) continue
  let allowed = false
  buffer.toString("utf8").split("\n").forEach((line, index) => {
    if (line.includes(ALLOW_START)) { allowed = true; return }
    if (line.includes(ALLOW_END)) { allowed = false; return }
    if (allowed || line.includes(ALLOW_MARKER)) return
    const found = spellings(maskKept(line, rel).masked)
    if (found.length > 0) findings.push(`${rel}:${index + 1}: ${found.join(", ")}`)
  })
}

if (findings.length > 0) {
  process.stderr.write(`brand-gate: ${findings.length} finding(s)\n${findings.slice(0, 200).join("\n")}\n`)
  if (findings.length > 200) process.stderr.write(`... and ${findings.length - 200} more\n`)
  process.stderr.write("\nRemedy: node scripts/rename-to-vibelang.mjs, or mark a deliberate mention with `brand-gate: allow`.\n")
  process.exit(1)
}
process.stdout.write("brand-gate: ok\n")
