// node --import vibelang/register: Node loads .vibe modules from source.
//
// A .vibe module keeps its source URL as its identity, so a program that also
// imports .ts files sees one copy of each module. Its JavaScript comes from one
// whole-project compile per tsconfig per process: emit is type-directed, so no
// module compiles alone. The project is the nearest tsconfig.json above the
// module, or VIBELANG_PROJECT.
import { existsSync } from "node:fs"
import { registerHooks } from "node:module"
import { resolve } from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"
import { compileProjectSync, formatDiagnostic, nearestProject } from "./project.mjs"

if (typeof registerHooks !== "function") {
  throw new Error(`vibelang/register needs module.registerHooks, in Node 22.15 or later; this is Node ${process.version}`)
}

const results = new Map()

function projectOf(file) {
  if (process.env.VIBELANG_PROJECT) return resolve(process.env.VIBELANG_PROJECT)
  const project = nearestProject(file)
  if (project === undefined) throw new Error(`vibelang/register: no tsconfig.json above ${file}`)
  return project
}

function emitted(file) {
  const project = projectOf(file)
  let result = results.get(project)
  if (result === undefined) {
    result = compileProjectSync({ project })
    results.set(project, result)
  }
  if (result.errors.length) {
    throw new Error(`vibelang: ${project} did not compile\n${result.errors.map(formatDiagnostic).join("\n")}`)
  }
  const output = result.outputs.get(file)
  if (output === undefined) {
    throw new Error(`vibelang/register: ${project} does not include ${file}; set VIBELANG_PROJECT to a tsconfig that does`)
  }
  return output
}

// vibelang rewrites a .vibe module's relative imports to the emitted .js names.
// From source, those names are the .vibe or TypeScript files beside it.
const sources = { ".js": [".vibe", ".ts", ".tsx"], ".mjs": [".mts"], ".cjs": [".cts"] }
const clean = (url) => url.replace(/[?#].*$/, "")

registerHooks({
  resolve(specifier, context, nextResolve) {
    try {
      return nextResolve(specifier, context)
    } catch (error) {
      const parent = context.parentURL
      if (
        error?.code !== "ERR_MODULE_NOT_FOUND" || !/^\.\.?\//.test(specifier) ||
        parent === undefined || !parent.startsWith("file:") || !clean(parent).endsWith(".vibe")
      ) throw error
      const target = new URL(specifier, parent)
      const path = fileURLToPath(clean(target.href))
      const extension = Object.keys(sources).find((name) => path.endsWith(name))
      for (const replacement of extension === undefined ? [] : sources[extension]) {
        const candidate = path.slice(0, -extension.length) + replacement
        if (existsSync(candidate)) return nextResolve(pathToFileURL(candidate).href + target.search + target.hash, context)
      }
      throw error
    }
  },
  load(url, context, nextLoad) {
    if (!url.startsWith("file:") || !clean(url).endsWith(".vibe")) return nextLoad(url, context)
    const { code, map } = emitted(fileURLToPath(clean(url)))
    const inline = Buffer.from(JSON.stringify(map)).toString("base64")
    return {
      format: "module",
      source: `${code}\n//# sourceMappingURL=data:application/json;base64,${inline}\n`,
      shortCircuit: true
    }
  }
})
