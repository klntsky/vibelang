import { existsSync, statSync } from "node:fs"
import { dirname, isAbsolute, resolve } from "node:path"
import { createUnplugin } from "unplugin"
import { findCompiler, formatDiagnostic, projectBackend } from "./backend.js"

const cleanId = (id) => id.split("?")[0]

// EFFECK_UNPLUGIN_DEBUG=1 traces each compile and module load on stderr.
const debug = process.env.EFFECK_UNPLUGIN_DEBUG ? (text) => console.error(`effeck-unplugin: ${text}`) : () => {}

// Returns a file modified after time, or one that is gone.
function changedSince(files, time) {
  return files.find((file) => {
    try {
      return statSync(file).mtimeMs > time
    } catch {
      return true
    }
  })
}

// Module code for a failed build. The bundler reports the real error.
const failed = "throw new Error(\"effeck: this module did not compile; see the build errors\")\n"

/** @type {import('./index.d.ts').unplugin} */
export const unplugin = createUnplugin((options = {}, meta) => {
  let root, project, backend, result, pending
  let revision = 0
  let compiled = -1
  let reported = -1

  function configure(base) {
    if (backend) return
    root = resolve(options.root || base || process.cwd())
    project = resolve(root, options.project || "tsconfig.json")
    backend = projectBackend({ root, compiler: findCompiler(root, options.compiler) })
  }

  async function ensureCompiled() {
    configure()
    while (compiled !== revision) {
      if (!pending) {
        const started = revision
        const time = Date.now()
        debug(`compile ${started} started`)
        pending = backend.compile(project)
          .then((value) => {
            result = value
            compiled = started
            debug(`compile ${started} finished in ${Date.now() - time}ms: ${value.outputs.size} outputs, ${value.errors.length} errors`)
            // A file saved during the compile may be missing from its output. esbuild records
            // watch files when a load returns, so it would not rebuild for that save. Compile again.
            const changed = changedSince([project, ...value.inputs], time)
            if (changed) {
              revision++
              debug(`${changed} changed during compile ${started}: revision ${revision}`)
            }
          })
          .finally(() => { pending = undefined })
      }
      await pending
    }
    return result
  }

  return {
    name: "effeck",
    enforce: "pre",
    // Every build, and every watch rebuild, compiles the project once.
    buildStart() {
      configure()
      revision++
      debug(`build start: revision ${revision}`)
    },
    watchChange(id) {
      if (/\.(?:effeck|[cm]?tsx?|json)$/.test(id)) revision++
      debug(`watch change ${id}: revision ${revision}`)
    },
    resolveId(source, importer) {
      const path = cleanId(source)
      const query = source.slice(path.length)
      // An absolute id is resolved; webpack sends each result back through this hook.
      if (isAbsolute(path) || (importer && !path.startsWith("."))) return
      const target = resolve(importer ? dirname(cleanId(importer)) : root || process.cwd(), path)
      // Resolve .effeck here too, so esbuild loads every .effeck module in one namespace.
      if (path.endsWith(".effeck")) return existsSync(target) ? target + query : undefined
      // effeck rewrites `./x.effeck` and `./x.ts` imports to `./x.js`. Map them back to the source.
      if (!importer || !path.endsWith(".js") || existsSync(target)) return
      const stem = target.slice(0, -3)
      // Only a compiled .effeck module has rewritten .ts imports. esbuild maps those itself,
      // and a plugin result would load the .ts file in the effeck namespace.
      const sources = cleanId(importer).endsWith(".effeck") && meta.framework !== "esbuild"
        ? [".effeck", ".ts", ".tsx"] : [".effeck"]
      for (const extension of sources) if (existsSync(stem + extension)) return stem + extension + query
    },
    loadInclude(id) {
      return cleanId(id).endsWith(".effeck")
    },
    async load(id) {
      const file = cleanId(id)
      const { outputs, inputs, errors, warnings } = await ensureCompiled()
      debug(`load ${file} from compile ${compiled}`)
      // Any project source or the tsconfig can change the contracts this module compiles against.
      for (const input of [project, ...inputs]) this.addWatchFile(input)
      if (errors.length) {
        // One report per compile: every .effeck module shares the project's diagnostics.
        if (reported !== compiled) {
          reported = compiled
          const first = errors.find((diagnostic) => diagnostic.file) || {}
          this.error({
            message: errors.map(formatDiagnostic).join("\n"),
            id: first.file || file,
            loc: first.file && { file: first.file, line: first.line, column: first.column - 1 }
          })
        }
        // esbuild, webpack, Rspack and Bun collect the error and need module code.
        return { code: failed, moduleType: "js" }
      }
      if (warnings.length && reported !== compiled) {
        reported = compiled
        for (const warning of warnings) this.warn(formatDiagnostic(warning))
      }
      const output = outputs.get(file)
      if (!output) {
        this.error({ message: `effeck: ${file} is not part of ${project}. Add it to the tsconfig's include.`, id: file })
        return { code: failed, moduleType: "js" }
      }
      return { code: output.code, map: output.map, moduleType: "js" }
    },
    vite: { configResolved(config) { configure(config.root) } },
    webpack(compiler) { configure(compiler.context) },
    rspack(compiler) { configure(compiler.context) },
    esbuild: { config(initialOptions) { configure(initialOptions.absWorkingDir) } }
  }
})

export default unplugin
