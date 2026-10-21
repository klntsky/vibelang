// Shared helpers for the vibelang/eslint tests.
import { readdirSync, statSync } from "node:fs"
import { join, relative } from "node:path"
import { fileURLToPath } from "node:url"

export const repo = fileURLToPath(new URL("../../../../", import.meta.url))

const skipped = new Set(["node_modules", ".cache", "dist", ".git", ".jj", "site"])

/** Returns the repository's files whose names end in one of suffixes, sorted. */
export function repositoryFiles(...suffixes) {
  const found = []
  const walk = (directory) => {
    for (const name of readdirSync(directory)) {
      if (skipped.has(name)) continue
      const path = join(directory, name)
      const stat = statSync(path, { throwIfNoEntry: false })
      if (stat?.isDirectory()) walk(path)
      else if (stat?.isFile() && suffixes.some((suffix) => name.endsWith(suffix))) found.push(path)
    }
  }
  walk(repo)
  return found.sort()
}

export const relativeToRepo = (path) => relative(repo, path)

/**
 * Returns a plain copy of an ESTree value for comparison: no loc or parent,
 * regular expressions and bigints as strings, keys sorted, undefined dropped.
 */
export function plain(value) {
  if (value === null || typeof value !== "object") {
    return typeof value === "bigint" ? `${value}n` : value
  }
  if (value instanceof RegExp) return String(value)
  if (Array.isArray(value)) return value.map(plain)
  const out = {}
  for (const key of Object.keys(value).sort()) {
    if (key === "loc" || key === "parent" || value[key] === undefined) continue
    out[key] = plain(value[key])
  }
  return out
}

/** Returns the path to the first difference between two plain values, or undefined. */
export function firstDifference(a, b, path = "$") {
  if (a === b) return undefined
  if (typeof a !== typeof b || a === null || b === null || typeof a !== "object") {
    return `${path}: ${JSON.stringify(a)?.slice(0, 160)} !== ${JSON.stringify(b)?.slice(0, 160)}`
  }
  if (Array.isArray(a) !== Array.isArray(b)) return `${path}: array mismatch`
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const key of keys) {
    const difference = firstDifference(a[key], b[key], `${path}.${key}${a?.type ? `<${a.type}>` : ""}`)
    if (difference) return difference
  }
  return undefined
}
