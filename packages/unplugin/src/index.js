import { existsSync } from "node:fs"
import { readFile } from "node:fs/promises"
import { createRequire } from "node:module"
import { dirname, isAbsolute, join, resolve } from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"
import { createUnplugin } from "unplugin"

const cleanId = (id) => id.split("?")[0]

// VIBELANG_UNPLUGIN_DEBUG=1 traces each compile and module load on stderr.
const debug = process.env.VIBELANG_UNPLUGIN_DEBUG ? (text) => console.error(`vibelang-unplugin: ${text}`) : () => {}

// The compile backend is the vibelang package's own: the project's install
// first, so it matches the compiler that runs, else the one beside this plugin.
async function loadBackend(root) {
  for (const base of [join(root, "package.json"), fileURLToPath(import.meta.url)]) {
    let path
    try {
      path = createRequire(base).resolve("vibelang/project")
    } catch {
      continue
    }
    return import(pathToFileURL(path).href)
  }
  throw new Error("vibelang-unplugin needs the vibelang package: npm install --save-dev vibelang")
}

async function esbuildMessage(formatDiagnostic, diagnostic) {
  return {
    text: formatDiagnostic(diagnostic),
    location: diagnostic.file && {
      file: diagnostic.file, line: diagnostic.line, column: diagnostic.column - 1,
      lineText: (await readFile(diagnostic.file, "utf8")).split(/\r?\n/)[diagnostic.line - 1]
    }
  }
}

/** @type {import('./index.d.ts').unplugin} */
export const unplugin = createUnplugin((options = {}, meta) => {
  let root, project, backend, modules, current

  function configure(base) {
    if (root) return
    root = resolve(options.root || base || process.cwd())
    project = resolve(root, options.project || "tsconfig.json")
  }

  // One compile per tsconfig: the project for root's modules, and each
  // workspace package's own for modules outside root.
  function loaded() {
    configure()
    modules ??= loadBackend(root).then((value) => {
      backend = value
      current = backend.projects({ root, project, compiler: backend.findCompiler(root, options.compiler), debug })
      return current
    })
    return modules
  }

  async function loadModule(id) {
    const file = cleanId(id)
    const result = await (await loaded()).load(file)
    debug(`load ${file} from ${result.project}`)
    return result
  }

  // The module the plugin serves for a specifier, or undefined to leave it to the bundler.
  function resolveId(source, importer) {
    const path = cleanId(source)
    const query = source.slice(path.length)
    // An absolute id is resolved; webpack sends each result back through this hook.
    if (isAbsolute(path) || (importer && !path.startsWith("."))) return
    const target = resolve(importer ? dirname(cleanId(importer)) : root || process.cwd(), path)
    // Resolve .vibe here too, so esbuild loads every .vibe module in one namespace.
    if (path.endsWith(".vibe")) return existsSync(target) ? target + query : undefined
    // vibelang rewrites `./x.vibe` and `./x.ts` imports to `./x.js`. Map them back to the source.
    if (!importer || !path.endsWith(".js") || existsSync(target)) return
    const stem = target.slice(0, -3)
    // Only a compiled .vibe module has rewritten .ts imports. esbuild maps those itself,
    // and a plugin result would load the .ts file in the vibelang namespace.
    const sources = cleanId(importer).endsWith(".vibe") && meta.framework !== "esbuild"
      ? [".vibe", ".ts", ".tsx"] : [".vibe"]
    for (const extension of sources) if (existsSync(stem + extension)) return stem + extension + query
  }

  return {
    name: "vibelang",
    enforce: "pre",
    // Every build, and every watch rebuild, compiles each project once.
    async buildStart() {
      ;(await loaded()).invalidate()
      debug("build start")
    },
    watchChange(id) {
      if (/\.(?:vibe|[cm]?tsx?|json)$/.test(id)) current?.invalidate()
      debug(`watch change ${id}`)
    },
    // Bun 1.4.2 stops resolving Bun.build's in-memory `files` once a plugin's
    // onResolve matches their names, and panics on a `with { type: "file" }`
    // import once a plugin's onLoad matches every file, even when the hooks
    // return nothing. unplugin's Bun adapter registers both for every path, so
    // under Bun the plugin registers its own hooks, filtered to what it serves.
    resolveId: meta.framework === "bun" ? undefined : resolveId,
    loadInclude: meta.framework === "bun" ? undefined : (id) => cleanId(id).endsWith(".vibe"),
    load: meta.framework === "bun" ? undefined : async function load(id) {
      const { project, code, map, inputs, errors, warnings, first, error } = await loadModule(id)
      // Any project source or the tsconfig can change the contracts this module compiles against.
      for (const input of [project, ...inputs]) this.addWatchFile(input)
      // A compile's diagnostics are reported once, with its first module.
      if (errors.length && first) {
        const first = errors.find((diagnostic) => diagnostic.file) || {}
        this.error({
          message: errors.map(backend.formatDiagnostic).join("\n"),
          id: first.file || cleanId(id),
          loc: first.file && { file: first.file, line: first.line, column: first.column - 1 }
        })
      }
      if (first && !errors.length) for (const warning of warnings) this.warn(backend.formatDiagnostic(warning))
      if (error) this.error(error)
      return { code, map, moduleType: "js" }
    },
    bun: {
      setup(build) {
        build.onResolve({ filter: /(?:\.vibe|^\.\.?\/.+\.js)(?:\?.*)?$/ }, (args) => {
          const entry = args.kind === "entry-point-build" || args.kind === "entry-point-run"
          const id = resolveId(args.path, entry ? undefined : args.importer)
          return id === undefined ? undefined : { path: id }
        })
        // Bun ignores plugin source maps, so the module is its JavaScript alone.
        build.onLoad({ filter: /\.vibe(?:\?.*)?$/ }, async ({ path }) => {
          const { code, errors, warnings, first, error } = await loadModule(path)
          if (errors.length && first) throw new Error(errors.map(backend.formatDiagnostic).join("\n"))
          if (first && !errors.length) for (const warning of warnings) console.warn(backend.formatDiagnostic(warning))
          if (error) throw new Error(error.message)
          return { contents: code, loader: "js" }
        })
      }
    },
    vite: { configResolved(config) { configure(config.root) } },
    webpack(compiler) { configure(compiler.context) },
    rspack(compiler) { configure(compiler.context) },
    esbuild: {
      config(initialOptions) { configure(initialOptions.absWorkingDir) },
      // No path matches /a^/: use the native loader to preserve diagnostic lineText.
      onLoadFilter: /a^/,
      setup(build) {
        build.onLoad({ filter: /\.vibe(?:\?.*)?$/ }, async ({ path }) => {
          const { project, code, map, inputs, errors, warnings, first, error } = await loadModule(path)
          const messages = []
          if (errors.length && first) {
            messages.push({
              ...await esbuildMessage(backend.formatDiagnostic, errors.find((diagnostic) => diagnostic.file) || errors[0]),
              text: errors.map(backend.formatDiagnostic).join("\n")
            })
          }
          if (error) messages.push({ text: error.message })
          return {
            contents: map
              ? `${code}\n//# sourceMappingURL=data:application/json;charset=utf-8;base64,${Buffer.from(JSON.stringify(map)).toString("base64")}`
              : code,
            loader: "js", resolveDir: dirname(path), watchFiles: [project, ...inputs],
            errors: messages,
            warnings: first && !errors.length ? await Promise.all(warnings.map((warning) => esbuildMessage(backend.formatDiagnostic, warning))) : []
          }
        })
      }
    }
  }
})

export default unplugin
