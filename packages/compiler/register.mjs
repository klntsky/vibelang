// node --import vibelang/register: Node loads .vibe modules from source.
//
// A .vibe module keeps its source URL as its identity, so a program that also
// imports .ts files sees one copy of each module. Its JavaScript comes from one
// whole-project compile per tsconfig: emit is type-directed, so no module
// compiles alone. The project is the nearest tsconfig.json above the module, or
// VIBELANG_PROJECT for the modules in its directory; a module of another
// workspace package, such as a dependency linked to its source, compiles with
// its own package's tsconfig.json. A compile's result is cached on disk (vibelang/project), so
// a later process reuses it while the project's inputs are unchanged. A failed
// compile throws an Error with code ERR_VIBELANG_COMPILE and the diagnostics;
// a module the project does not include, ERR_VIBELANG_PROJECT. Source maps are
// turned on, so a stack frame names the .vibe line.
// VIBELANG_REGISTER_DEBUG=1 reports each project's compile or cache hit.
import { existsSync, statSync } from "node:fs"
import * as Module from "node:module"
import { registerHooks } from "node:module"
import { dirname, resolve } from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"
import { compileProjectSync, formatDiagnostic, nearestProject, workspaceProject } from "./project.mjs"

if (typeof registerHooks !== "function") {
  throw new Error(`vibelang/register needs module.registerHooks, in Node 22.15 or later; this is Node ${process.version}`)
}

// A .vibe module's frames name its .vibe path; without source maps they would
// carry the emitted JavaScript's line numbers. Node 22.14+ (every Node with
// registerHooks) has setSourceMapsSupport.
Module.setSourceMapsSupport?.(true)

const results = new Map()
const debug = process.env.VIBELANG_REGISTER_DEBUG
  ? (text) => process.stderr.write(`vibelang/register: ${text}\n`)
  : undefined

function projectOf(file) {
  if (process.env.VIBELANG_PROJECT) {
    const named = resolve(process.env.VIBELANG_PROJECT)
    return workspaceProject(file, dirname(named), named)
  }
  const project = nearestProject(file)
  if (project === undefined) throw new Error(`vibelang/register: no tsconfig.json above ${file}`)
  return project
}

// An Error with a code, as Node's own module errors have.
const failure = (code, message) => Object.assign(new Error(message), { code })

// A file saved after a compile started may be missing from its output.
function changedSince(file, time) {
  const stat = statSync(file, { throwIfNoEntry: false })
  return stat !== undefined && Math.max(stat.mtimeMs, stat.ctimeMs) > time
}

// A long-running process compiles a project again when a module it imports
// is newer than the compile, or was not there yet: a watcher's re-import of
// an edited file, or a module written after the program started.
function emitted(file) {
  const project = projectOf(file)
  let compiled = results.get(project)
  if (compiled !== undefined && changedSince(file, compiled.started)) compiled = undefined
  if (compiled !== undefined && !compiled.result.errors.length && !compiled.result.outputs.has(file) && !compiled.missing.has(file)) {
    compiled = undefined
  }
  if (compiled === undefined) {
    const started = Date.now()
    compiled = { started, result: compileProjectSync({ project, debug }), missing: new Set() }
    results.set(project, compiled)
  }
  const { result } = compiled
  if (result.errors.length) {
    throw failure("ERR_VIBELANG_COMPILE", `vibelang: ${project} did not compile\n${result.errors.map(formatDiagnostic).join("\n")}`)
  }
  const output = result.outputs.get(file)
  if (output === undefined) {
    compiled.missing.add(file)
    throw failure("ERR_VIBELANG_PROJECT", `vibelang/register: ${project} does not include ${file}; set VIBELANG_PROJECT to a tsconfig that does`)
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
      // Split, so tools that read this file for its own source map skip the template.
      source: `${code}\n//# sourceMapping${"URL"}=data:application/json;base64,${inline}\n`,
      shortCircuit: true
    }
  }
})
