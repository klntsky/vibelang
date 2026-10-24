// bun --preload vibelang/bun: Bun loads .vibe modules from source.
//
// The Bun counterpart of vibelang/register, for `bun run`, `bun test`, and a
// bunfig.toml `preload`. A .vibe module's JavaScript comes from one
// whole-project compile per tsconfig per process: emit is type-directed, so no
// module compiles alone. The project is the nearest tsconfig.json above the
// module, or VIBELANG_PROJECT for the modules in its directory (a module of
// another workspace package compiles with its own tsconfig.json). A compile's result is cached on disk
// (vibelang/project), so a later process reuses it while the project's inputs
// are unchanged.
import { existsSync } from "node:fs"
import { dirname, resolve } from "node:path"
import { compileProjectSync, formatDiagnostic, nearestProject, workspaceProject } from "./project.mjs"

if (typeof Bun === "undefined") {
  throw new Error("vibelang/bun is a Bun preload; under Node, use node --import vibelang/register")
}

const results = new Map()

function projectOf(file) {
  if (process.env.VIBELANG_PROJECT) {
    const named = resolve(process.env.VIBELANG_PROJECT)
    return workspaceProject(file, dirname(named), named)
  }
  const project = nearestProject(file)
  if (project === undefined) throw new Error(`vibelang/bun: no tsconfig.json above ${file}`)
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
    throw new Error(`vibelang/bun: ${project} does not include ${file}; set VIBELANG_PROJECT to a tsconfig that does`)
  }
  return output
}

Bun.plugin({
  name: "vibelang",
  setup(build) {
    // vibelang rewrites a .vibe module's relative imports to the emitted .js
    // names. From source, a missing ./x.js is the .vibe file beside it; Bun
    // maps a missing .js to its .ts or .tsx file itself.
    build.onResolve({ filter: /^\.\.?\/.*\.js$/ }, ({ path, importer }) => {
      if (!importer?.endsWith(".vibe")) return undefined
      const target = resolve(dirname(importer), path)
      if (existsSync(target)) return undefined
      const source = target.slice(0, -".js".length) + ".vibe"
      return existsSync(source) ? { path: source } : undefined
    })
    // Bun 1.4.2 ignores a plugin's source maps, so the module is its JavaScript alone.
    build.onLoad({ filter: /\.vibe$/ }, ({ path }) => ({ contents: emitted(path).code, loader: "js" }))
  }
})
