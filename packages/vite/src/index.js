import { existsSync } from "node:fs"
import { createRequire } from "node:module"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"

const cleanId = (id) => id.split("?")[0]
const slash = (path) => path.replaceAll("\\", "/")

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
  throw new Error("vibelang-vite needs the vibelang package: npm install --save-dev vibelang")
}

// The Node loader's absolute URL, so a process started in any directory finds it.
function registerUrl(root) {
  for (const base of [join(root, "package.json"), fileURLToPath(import.meta.url)]) {
    try {
      return pathToFileURL(createRequire(base).resolve("vibelang/register")).href
    } catch {
      continue
    }
  }
  throw new Error("vibelang-vite's register option needs the vibelang package: npm install --save-dev vibelang")
}

/** @param {import('./index.d.ts').VibeLangOptions} options */
export default function vibelang(options = {}) {
  let root, project, logger, backend, modules, current

  // One compile per tsconfig: the project for root's modules, and each
  // workspace package's own for modules outside root.
  function loaded() {
    modules ??= loadBackend(root).then((value) => {
      backend = value
      current = backend.projects({ root, project, compiler: backend.findCompiler(root, options.compiler) })
      return current
    })
    return modules
  }

  function diagnostics(errors, file) {
    const first = errors.find((diagnostic) => diagnostic.file) || {}
    return {
      message: errors.map(backend.formatDiagnostic).join("\n"),
      id: first.file || file,
      loc: first.file && { file: first.file, line: first.line, column: first.column - 1 }
    }
  }

  return {
    name: "vibelang",
    enforce: "pre",
    config(config) {
      if (!options.register) return
      // Vitest copies test.env into process.env, which the processes a test starts inherit.
      const flag = `--import=${registerUrl(resolve(config.root || process.cwd()))}`
      const inherited = config.test?.env?.NODE_OPTIONS ?? process.env.NODE_OPTIONS ?? ""
      if (inherited.includes(flag)) return
      return { test: { env: { NODE_OPTIONS: [inherited, flag].filter(Boolean).join(" ") } } }
    },
    configResolved(config) {
      root = config.root
      project = resolve(root, options.project || "tsconfig.json")
      logger = config.logger
    },
    configureServer(server) {
      // Vite otherwise treats an unknown extension as a static asset on HTTP.
      server.middlewares.use((request, _response, next) => {
        const url = new URL(request.url || "/", "http://localhost")
        if (url.pathname.endsWith(".vibe") && !url.searchParams.has("raw") && !url.searchParams.has("url")) {
          url.searchParams.set("import", "")
          request.url = url.pathname + url.search
        }
        next()
      })
    },
    resolveId(source, importer) {
      // vibelang rewrites .vibe imports to .js. Keep the original source in Vite's graph.
      if (!importer || !source.startsWith(".") || !cleanId(source).endsWith(".js")) return
      const target = resolve(dirname(cleanId(importer)), cleanId(source))
      if (existsSync(target)) return
      const es = target.slice(0, -3) + ".vibe"
      if (existsSync(es)) return slash(es) + source.slice(cleanId(source).length)
    },
    async transform(_source, id) {
      const file = cleanId(id)
      if (!file.endsWith(".vibe") || /[?&](?:raw|url)(?:&|=|$)/.test(id)) return
      const { project, code, map, errors, warnings, first, error } = await (await loaded()).load(file)
      this.addWatchFile(project)
      // Every module of a failed compile fails, so Vite reports each request.
      if (errors.length) this.error(diagnostics(errors, file))
      if (first) for (const warning of warnings) logger.warn(backend.formatDiagnostic(warning))
      if (error) this.error(error)
      return { code, map }
    },
    watchChange(id) {
      if (/\.(?:vibe|[cm]?tsx?|json)$/.test(id)) current?.invalidate()
    },
    async handleHotUpdate(context) {
      if (!/\.(?:vibe|[cm]?tsx?)$/.test(context.file) && !context.file.endsWith(".json")) return
      const projects = await loaded()
      projects.invalidate()
      // Any source can change the contracts used to compile a .vibe importer.
      const affected = new Set()
      const visit = (module) => {
        if (affected.has(module)) return
        affected.add(module)
        for (const importer of module.importers) visit(importer)
      }
      for (const module of context.modules) visit(module)
      for (const module of context.server.moduleGraph.idToModuleMap.values()) {
        if (module.file?.endsWith(".vibe")) visit(module)
      }
      for (const module of affected) context.server.moduleGraph.invalidateModule(module)
      // Compile the edited file's project now, so the overlay shows its diagnostics.
      const { errors } = await projects.load(context.file)
      if (errors.length) {
        const { message, id, loc } = diagnostics(errors, context.file)
        context.server.config.logger.error(message)
        context.server.ws.send({ type: "error", err: { message, stack: "", plugin: "vibelang", id, loc } })
        return []
      }
      return [...affected]
    },
    closeBundle() {
      // A later build compiles afresh.
      modules = undefined
      current = undefined
    }
  }
}
