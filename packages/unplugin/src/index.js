import { existsSync, statSync } from "node:fs"
import { readFile } from "node:fs/promises"
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

async function esbuildMessage(diagnostic) {
  return {
    text: formatDiagnostic(diagnostic),
    location: diagnostic.file && {
      file: diagnostic.file, line: diagnostic.line, column: diagnostic.column - 1,
      lineText: (await readFile(diagnostic.file, "utf8")).split(/\r?\n/)[diagnostic.line - 1]
    }
  }
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

  async function loadModule(id) {
    const file = cleanId(id)
    const { outputs, inputs, errors, warnings } = await ensureCompiled()
    debug(`load ${file} from compile ${compiled}`)
    // Every .effeck module shares the project's diagnostics. Report them once per compile.
    const report = reported !== compiled
    if (errors.length || warnings.length) reported = compiled
    if (errors.length) {
      return { code: failed, inputs, errors: report ? errors : [], warnings: [] }
    }
    const output = outputs.get(file)
    return {
      code: output?.code ?? failed, map: output?.map, inputs,
      errors: [], warnings: report ? warnings : [],
      error: !output && {
        message: `effeck: ${file} is not part of ${project}. Add it to the tsconfig's include.`, id: file
      }
    }
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
      const { code, map, inputs, errors, warnings, error } = await loadModule(id)
      // Any project source or the tsconfig can change the contracts this module compiles against.
      for (const input of [project, ...inputs]) this.addWatchFile(input)
      if (errors.length) {
        const first = errors.find((diagnostic) => diagnostic.file) || {}
        this.error({
          message: errors.map(formatDiagnostic).join("\n"),
          id: first.file || cleanId(id),
          loc: first.file && { file: first.file, line: first.line, column: first.column - 1 }
        })
      }
      for (const warning of warnings) this.warn(formatDiagnostic(warning))
      if (error) this.error(error)
      return { code, map, moduleType: "js" }
    },
    vite: { configResolved(config) { configure(config.root) } },
    webpack(compiler) { configure(compiler.context) },
    rspack(compiler) { configure(compiler.context) },
    esbuild: {
      config(initialOptions) { configure(initialOptions.absWorkingDir) },
      // No path matches /a^/: use the native loader to preserve diagnostic lineText.
      onLoadFilter: /a^/,
      setup(build) {
        build.onLoad({ filter: /\.effeck(?:\?.*)?$/ }, async ({ path }) => {
          const { code, map, inputs, errors, warnings, error } = await loadModule(path)
          const messages = []
          if (errors.length) {
            messages.push({
              ...await esbuildMessage(errors.find((diagnostic) => diagnostic.file) || errors[0]),
              text: errors.map(formatDiagnostic).join("\n")
            })
          }
          if (error) messages.push({ text: error.message })
          return {
            contents: map
              ? `${code}\n//# sourceMappingURL=data:application/json;charset=utf-8;base64,${Buffer.from(JSON.stringify(map)).toString("base64")}`
              : code,
            loader: "js", resolveDir: dirname(path), watchFiles: [project, ...inputs],
            errors: messages, warnings: await Promise.all(warnings.map(esbuildMessage))
          }
        })
      }
    }
  }
})

export default unplugin
