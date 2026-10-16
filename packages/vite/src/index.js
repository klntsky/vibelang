import { spawn } from "node:child_process"
import { existsSync } from "node:fs"
import { mkdir, mkdtemp, readFile, rm } from "node:fs/promises"
import { createRequire } from "node:module"
import { dirname, isAbsolute, join, relative, resolve } from "node:path"

const cleanId = (id) => id.split("?")[0]
const slash = (path) => path.replaceAll("\\", "/")

/** @param {import('./index.d.ts').EffectScriptOptions} options */
export default function effectscript(options = {}) {
  let root, project, compiler, cache, pending
  let revision = 0
  let compiled = -1
  let logger

  async function compile() {
    if (!cache) {
      const base = join(root, "node_modules/.effectscript")
      await mkdir(base, { recursive: true })
      cache = await mkdtemp(join(base, "vite-"))
    }
    // A whole-program check preserves esc's inferred contracts across modules.
    await rm(join(cache, "out"), { recursive: true, force: true })
    const args = [
      "--project", project, "--pretty", "false", "--rootDir", root,
      "--outDir", join(cache, "out"), "--sourceMap", "true", "--inlineSources", "true",
      "--inlineSourceMap", "false", "--sourceRoot", slash(root) + "/", "--declaration", "false", "--declarationMap", "false",
      "--noEmit", "false", "--emitDeclarationOnly", "false", "--noEmitOnError", "true",
      "--incremental", "false"
    ]
    const command = /\.(?:[cm]?js)$/.test(compiler) || compiler.endsWith("/bin/esc")
      ? [process.execPath, [compiler, ...args]] : [compiler, args]
    await new Promise((accept, reject) => {
      let output = ""
      const child = spawn(command[0], command[1], { cwd: root, stdio: ["ignore", "pipe", "pipe"] })
      child.stdout.on("data", (data) => { output += data })
      child.stderr.on("data", (data) => { output += data })
      child.on("error", (error) => reject(new Error(`esc: ${error.message}\nCompiler: ${compiler}`)))
      child.on("close", (code, signal) => {
        if (code !== 0) {
          const error = new Error(output.trim() || `esc exited with ${signal || code}`)
          const span = output.match(/^(.+?)\((\d+),(\d+)\):\s+(?:error|warning)\s+\w+\d+:/m)
          if (span) {
            error.id = resolve(root, span[1])
            error.loc = { file: error.id, line: Number(span[2]), column: Number(span[3]) - 1 }
          }
          reject(error)
        } else {
          if (output.trim()) logger.warn(output.trim())
          accept()
        }
      })
    })
  }

  async function ensureCompiled() {
    while (compiled !== revision) {
      if (!pending) {
        const started = revision
        pending = compile().then(() => { compiled = started }).finally(() => { pending = undefined })
      }
      await pending
    }
  }

  return {
    name: "effectscript",
    enforce: "pre",
    configResolved(config) {
      root = config.root
      project = resolve(root, options.project || "tsconfig.json")
      logger = config.logger
      compiler = options.compiler || process.env.ESC_BINARY
      if (compiler) compiler = resolve(root, compiler)
      else {
        const require = createRequire(join(root, "package.json"))
        compiler = join(dirname(require.resolve("@effectscript/compiler/package.json")), "bin/esc")
      }
    },
    configureServer(server) {
      // Vite otherwise treats an unknown extension as a static asset on HTTP.
      server.middlewares.use((request, _response, next) => {
        const url = new URL(request.url || "/", "http://localhost")
        if (url.pathname.endsWith(".es") && !url.searchParams.has("raw") && !url.searchParams.has("url")) {
          url.searchParams.set("import", "")
          request.url = url.pathname + url.search
        }
        next()
      })
    },
    resolveId(source, importer) {
      // esc rewrites .es imports to .js. Keep the original source in Vite's graph.
      if (!importer || !source.startsWith(".") || !cleanId(source).endsWith(".js")) return
      const target = resolve(dirname(cleanId(importer)), cleanId(source))
      if (existsSync(target)) return
      const es = target.slice(0, -3) + ".es"
      if (existsSync(es)) return slash(es) + source.slice(cleanId(source).length)
    },
    async transform(_source, id) {
      const file = cleanId(id)
      if (!file.endsWith(".es") || /[?&](?:raw|url)(?:&|=|$)/.test(id)) return
      this.addWatchFile(project)
      try {
        await ensureCompiled()
        const path = relative(root, file)
        if (path.startsWith("..") || isAbsolute(path)) throw new Error(`EffectScript source is outside Vite root: ${file}`)
        const emitted = join(cache, "out", path.replace(/\.es$/, ".js"))
        const [code, rawMap] = await Promise.all([readFile(emitted, "utf8"), readFile(emitted + ".map", "utf8")])
        const map = JSON.parse(rawMap)
        map.sources = map.sources.map((source) => slash(resolve(dirname(emitted), map.sourceRoot || "", source)))
        map.sourceRoot = ""
        return { code: code.replace(/\/\/# sourceMappingURL=.*$/m, ""), map }
      } catch (error) {
        this.error({ message: error.message, id: error.id || file, loc: error.loc })
      }
    },
    watchChange(id) {
      if (/\.(?:es|[cm]?tsx?|json)$/.test(id)) revision++
    },
    async handleHotUpdate(context) {
      if (!/\.(?:es|[cm]?tsx?)$/.test(context.file) && !context.file.endsWith(".json")) return
      revision++
      // Any source can change the contracts used to compile an .es importer.
      const affected = new Set()
      const visit = (module) => {
        if (affected.has(module)) return
        affected.add(module)
        for (const importer of module.importers) visit(importer)
      }
      for (const module of context.modules) visit(module)
      for (const module of context.server.moduleGraph.idToModuleMap.values()) {
        if (module.file?.endsWith(".es")) visit(module)
      }
      for (const module of affected) context.server.moduleGraph.invalidateModule(module)
      try {
        await ensureCompiled()
      } catch (error) {
        context.server.config.logger.error(error.message)
        context.server.ws.send({ type: "error", err: { message: error.message, stack: "", plugin: "effectscript", id: error.id, loc: error.loc } })
        return []
      }
      return [...affected]
    },
    async closeBundle() {
      try { await pending } catch { /* A diagnostic already reached Vite. */ }
      if (cache) await rm(cache, { recursive: true, force: true })
      cache = undefined
      compiled = -1
    }
  }
}
