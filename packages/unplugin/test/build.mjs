// Bundles src/main.ts of the fixture in the current directory into out/
// with one bundler and vibelang-unplugin, and prints one JSON line per build:
//   {"ok":true}  or  {"ok":false,"message":"..."}
//
// Usage, from a fixture copy:
//   node build.mjs webpack|rspack|rollup|rolldown|esbuild|vite [--watch]
//   node build.mjs rollup|rolldown|vite --deferred   (the plugin from vibelang-unplugin/deferred)
//   bun build.mjs bun --files                       (entry points from Bun.build's in-memory files)
//   bun build.mjs bun
//
// The plugin finds the compiler through VIBELANG_BINARY. A build without
// --watch exits 1 when it fails.
import { statSync } from "node:fs"
import module from "node:module"
import { join, resolve } from "node:path"
import { createUnplugin } from "unplugin"

const [bundler, mode] = process.argv.slice(2)
const watch = mode === "--watch"
const root = process.cwd()
const entry = resolve(root, "src/main.ts")
const outDir = resolve(root, "out")
const { deferred } = await import("vibelang-unplugin/deferred")
const vibelang = mode === "--deferred"
  ? (options) => deferred(bundler, options)
  : (await import(`vibelang-unplugin/${bundler}`)).default

function report(ok, message, context) {
  console.log(JSON.stringify(ok ? { ok } : { ok, message, ...(context ? { context } : {}) }))
  if (!watch) process.exitCode = ok ? 0 : 1
}

// Watch diagnostics for a failing test: when each rebuild starts, and why.
const started = Date.now()
function trace(text) {
  if (watch) console.error(`${bundler} +${Date.now() - started}ms ${text}`)
}

// webpack, Rspack and Rollup do not read TypeScript. The entry needs only
// its types removed; Node's stripper keeps every position.
const stripTypes = createUnplugin(() => ({
  name: "strip-types",
  transformInclude: (id) => id.endsWith(".ts"),
  transform: (code) => ({ code: module.stripTypeScriptTypes(code), map: null })
}))

// Rollup does not resolve packages; Node resolves these from out/ at run time.
const external = (id) => /^(?:effect|vibelang-runtime)(?:\/|$)/.test(id)

if (bundler === "webpack" || bundler === "rspack") {
  const create = bundler === "webpack" ? (await import("webpack")).default : (await import("@rspack/core")).rspack
  const compiler = create({
    mode: "none", context: root, target: "node", devtool: "source-map", entry,
    output: { path: outDir, filename: "main.cjs" },
    plugins: [stripTypes[bundler](), vibelang()],
    infrastructureLogging: { level: "error" }
  })
  const done = (error, stats) => {
    if (error) return report(false, error.message)
    report(!stats.hasErrors(), stats.compilation.errors.map((item) => item.message).join("\n"))
  }
  compiler.hooks.watchRun.tap("trace", (watching) => trace(`start: ${[...(watching.modifiedFiles || [])].join(", ")}`))
  if (watch) compiler.watch({ aggregateTimeout: 100 }, done)
  else compiler.run((error, stats) => { done(error, stats); compiler.close(() => {}) })
} else if (bundler === "rollup" || bundler === "rolldown") {
  const api = await import(bundler)
  const input = bundler === "rollup"
    ? { input: entry, external, plugins: [stripTypes.rollup(), vibelang()], onwarn() {} }
    : { input: entry, platform: "node", plugins: [vibelang()], onLog() {} }
  const output = { dir: outDir, format: "es", sourcemap: true, entryFileNames: "main.js" }
  if (watch) {
    const watcher = api.watch({ ...input, output, watch: { clearScreen: false } })
    watcher.on("change", (id) => trace(`change: ${id}`))
    watcher.on("event", async (event) => {
      if (event.code === "START") trace("start")
      if (event.code === "BUNDLE_END") { await event.result.close(); report(true) }
      if (event.code === "ERROR") { await event.result?.close(); report(false, event.error.message) }
    })
  } else {
    try {
      const bundle = await api[bundler](input)
      await bundle.write(output)
      await bundle.close()
      report(true)
    } catch (error) {
      report(false, error.message)
    }
  }
} else if (bundler === "esbuild") {
  const esbuild = await import("esbuild")
  const reporter = {
    name: "report",
    setup(build) {
      build.onStart(() => trace("start"))
      build.onEnd((result) => report(!result.errors.length, result.errors.map((error) => {
        const at = error.location ? ` (${error.location.file}:${error.location.line}:${error.location.column})` : ""
        return `${error.text}${at} [plugin ${error.pluginName}]`
      }).join("\n")))
    }
  }
  const options = {
    entryPoints: [entry], bundle: true, platform: "node", format: "esm", sourcemap: true,
    outfile: join(outDir, "main.js"), plugins: [vibelang(), reporter], logLevel: "silent"
  }
  if (watch) await (await esbuild.context(options)).watch()
  else await esbuild.build(options).catch(() => {})
} else if (bundler === "bun") {
  const assetPath = join(root, "helper.bin")
  const snapshot = (path) => {
    try {
      const stat = statSync(path)
      return { exists: true, size: stat.size, mtimeMs: stat.mtimeMs, ctimeMs: stat.ctimeMs }
    } catch (error) {
      return { exists: false, error: error.code }
    }
  }
  let stage = "prepare", beforeBuild
  const failureContext = () => {
    let resolved
    try { resolved = Bun.resolveSync(assetPath, root) } catch (error) { resolved = error.message }
    return { stage, bun: Bun.version, root, asset: assetPath, beforeBuild, afterBuild: { asset: snapshot(assetPath), directory: snapshot(root) }, resolved }
  }
  try {
    // --files: a virtual entry that embeds a file asset and imports the fixture, and a virtual .js entry beside it,
    // as an app that compiles to one executable embeds a helper and a worker.
    const asset = JSON.stringify(join(root, "helper.bin"))
    // fixture() copies helper.bin before launching Bun. Creating it here is
    // too late: Bun can reuse the directory listing from module resolution.
    const files = mode === "--files"
      ? {
        "virtual-main.ts": `import asset from ${asset} with { type: "file" }\nif (typeof asset !== "string") throw new Error("no asset")\nawait import(${JSON.stringify(entry)})`,
        "virtual-worker.js": "export const worker = 1"
      }
      : undefined
    beforeBuild = { asset: snapshot(assetPath), directory: snapshot(root) }
    stage = "build"
    const result = await Bun.build({
      entrypoints: files ? Object.keys(files) : [entry], outdir: outDir, target: "bun", format: "esm",
      sourcemap: "external", naming: files ? "[name].js" : "main.js", plugins: [vibelang()], ...(files ? { files } : {})
    })
    report(result.success, result.logs.map(String).join("\n"), result.success ? undefined : failureContext())
  } catch (error) {
    report(false, [error.message, ...(error.errors || []).map(String)].join("\n"), failureContext())
  }
} else if (bundler === "vite") {
  const { build } = await import("vite")
  const result = build({
    root, configFile: false, logLevel: "silent", plugins: [vibelang()],
    build: {
      ssr: entry, outDir, sourcemap: true, emptyOutDir: true, watch: watch ? { clearScreen: false } : null,
      rolldownOptions: { output: { entryFileNames: "main.js" } }
    }
  })
  if (watch) {
    const watcher = await result
    watcher.on("change", (id) => trace(`change: ${id}`))
    watcher.on("event", async (event) => {
      if (event.code === "START") trace("start")
      if (event.code === "BUNDLE_END") { await event.result.close(); report(true) }
      if (event.code === "ERROR") { await event.result?.close(); report(false, event.error.message) }
    })
  } else {
    await result.then(() => report(true), (error) => report(false, error.message))
  }
} else {
  throw new Error(`build.mjs: unknown bundler ${bundler}`)
}
