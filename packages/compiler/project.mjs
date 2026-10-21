// Whole-project compilation for the bundler plugins (vibelang-vite,
// vibelang-unplugin) and the Node loader (vibelang/register).
//
// VibeLang's emit is type-directed: a module's JavaScript depends on the
// contracts of the modules it imports, so no module compiles alone. Each
// compile is one vibelang process over a whole tsconfig project.
import { spawn, spawnSync } from "node:child_process"
import { closeSync, existsSync, mkdtempSync, openSync, readFileSync, readSync, realpathSync, rmSync, statSync } from "node:fs"
import { createRequire } from "node:module"
import { tmpdir } from "node:os"
import { dirname, isAbsolute, join, relative, resolve } from "node:path"

const slash = (path) => path.replaceAll("\\", "/")
const located = /^(.+?)\((\d+),(\d+)\):\s+(error|warning)\s+(\w+\d+):\s?(.*)$/
const global = /^(error|warning)\s+(\w+\d+):\s?(.*)$/

/**
 * Returns the compiler command: the `compiler` option, else VIBELANG_BINARY,
 * else the launcher of the vibelang package installed for `root`.
 */
export function findCompiler(root, compiler) {
  if (compiler) return resolve(root, compiler)
  if (process.env.VIBELANG_BINARY) return resolve(root, process.env.VIBELANG_BINARY)
  const require = createRequire(join(root, "package.json"))
  return join(dirname(require.resolve("vibelang/package.json")), "bin/vibelang")
}

// The npm launcher and the native platform packages both install bin/vibelang,
// so read the shebang instead of trusting the filename.
function isNodeLauncher(path) {
  if (/\.(?:[cm]?js)$/.test(path)) return true
  let file
  try {
    file = openSync(path, "r")
    const header = Buffer.alloc(256)
    const bytes = readSync(file, header, 0, header.length, 0)
    return /^#![^\r\n]*\bnode(?:\s|$)/.test(header.subarray(0, bytes).toString("utf8"))
  } catch {
    return false
  } finally {
    if (file !== undefined) closeSync(file)
  }
}

const command = (compiler, args) =>
  isNodeLauncher(compiler) ? [process.execPath, [compiler, ...args]] : [compiler, args]

/**
 * Parses `vibelang --pretty false --listFiles` output into the files the
 * program read and the diagnostics it reported. A diagnostic's position is
 * 1-based, as the compiler prints it.
 */
export function parseOutput(output, root) {
  const files = []
  const diagnostics = []
  for (const line of output.split(/\r?\n/)) {
    let match
    if ((match = line.match(located))) {
      diagnostics.push({
        file: resolve(root, match[1]), line: Number(match[2]), column: Number(match[3]),
        category: match[4], code: match[5], text: match[6]
      })
    } else if ((match = line.match(global))) {
      diagnostics.push({ category: match[1], code: match[2], text: match[3] })
    } else if (/^\s/.test(line) && diagnostics.length) {
      diagnostics.at(-1).text += "\n" + line
    } else if (isAbsolute(line.trim())) {
      files.push(line.trim())
    }
  }
  return { files, diagnostics }
}

const imported = /^\s+Imported via (".*"|'.*') from file '(.+?)'(?: with packageId '[^']*')?( to import .+)?$/

/**
 * Parses `vibelang --pretty false --explainFiles --listFilesOnly` output into
 * the program's files, the imports its sources write, and the diagnostics the
 * compiler reported. Each import is { importer, specifier, file }: the file
 * the compiler resolved `specifier` to. Imports the compiler adds itself, such
 * as JSX factories and import helpers, and its bundled libraries are left out.
 * The output's paths are relative to root; the result's are absolute.
 */
export function parseExplainFiles(output, root) {
  const files = []
  const imports = []
  const diagnostics = []
  // An indented line belongs to the last unindented one: a file's reasons or a diagnostic's detail.
  let file
  let diagnostic
  let match
  for (const line of output.split(/\r?\n/)) {
    if (/^\s/.test(line)) {
      if (diagnostic) diagnostic.text += "\n" + line
      else if (file && (match = line.match(imported)) && !match[3]) {
        imports.push({ importer: resolve(root, match[2]), specifier: match[1].slice(1, -1), file })
      }
      continue
    }
    file = diagnostic = undefined
    if ((match = line.match(located))) {
      diagnostic = {
        file: resolve(root, match[1]), line: Number(match[2]), column: Number(match[3]),
        category: match[4], code: match[5], text: match[6]
      }
      diagnostics.push(diagnostic)
    } else if ((match = line.match(global))) {
      diagnostic = { category: match[1], code: match[2], text: match[3] }
      diagnostics.push(diagnostic)
    } else if (line.trim() && !line.startsWith("bundled:")) {
      file = resolve(root, line.trim())
      files.push(file)
    }
  }
  return { files, imports, diagnostics }
}

/**
 * projectImports lists a tsconfig project's files and the imports its `.ts`
 * and `.vibe` sources write, each with the file the compiler resolved it to,
 * without type-checking the project. A tool that needs a module graph, such as
 * a circular-dependency check, reads it here instead of resolving `.vibe`
 * specifiers itself. It returns { files, imports, errors }; errors holds the
 * diagnostics that stopped the compiler from reading the project.
 */
export function projectImports({ project, root = dirname(project), compiler = findCompiler(root) }) {
  const [file, argv] = command(compiler, ["--project", project, "--pretty", "false", "--explainFiles", "--listFilesOnly"])
  const run = spawnSync(file, argv, { cwd: root, encoding: "utf8", maxBuffer: 1 << 30 })
  if (run.error) throw new Error(`vibelang: ${run.error.message}\nCompiler: ${compiler}`)
  const output = run.stdout + run.stderr
  const { files, imports, diagnostics } = parseExplainFiles(output, root)
  const errors = diagnostics.filter((diagnostic) => diagnostic.category === "error")
  if (run.status !== 0 && !errors.length) {
    errors.push({ category: "error", code: "vibelang", text: output.trim() || `exited with ${run.signal || run.status}` })
  }
  return { files, imports, errors }
}

/** Formats a diagnostic as `file:line:column - error TS2322: text`. */
export function formatDiagnostic(diagnostic) {
  const where = diagnostic.file ? `${diagnostic.file}:${diagnostic.line}:${diagnostic.column} - ` : ""
  return `${where}${diagnostic.category} ${diagnostic.code}: ${diagnostic.text}`
}

const emitArgs = (project, root, out) => [
  "--project", project, "--pretty", "false", "--listFiles", "true", "--rootDir", root,
  "--outDir", out, "--sourceMap", "true", "--inlineSources", "true",
  "--inlineSourceMap", "false", "--sourceRoot", slash(root) + "/", "--declaration", "false", "--declarationMap", "false",
  "--noEmit", "false", "--emitDeclarationOnly", "false", "--noEmitOnError", "true",
  "--incremental", "false"
]

const realpath = (path) => {
  try {
    return realpathSync(path)
  } catch {
    return path
  }
}

// Turns one compiler run into its result:
//   outputs      Map from each .vibe source, by listed and real path, to its { code, map }
//   inputs       project source files the result depends on
//   errors       diagnostics that blocked emission; outputs is then empty
//   warnings     diagnostics of a successful compile
function collect({ status, signal, output }, root, out) {
  const { files, diagnostics } = parseOutput(output, root)
  // Dependencies are contracts too; their sources change only on install.
  const inputs = files.filter((path) => !slash(path).includes("/node_modules/"))
  const errors = diagnostics.filter((diagnostic) => diagnostic.category === "error")
  const warnings = diagnostics.filter((diagnostic) => diagnostic.category === "warning")
  if (status !== 0 && !errors.length) {
    // A crash or an unknown flag prints no diagnostic; report the raw output.
    errors.push({ category: "error", code: "vibelang", text: output.trim() || `exited with ${signal || status}` })
  }
  const outputs = new Map()
  for (const source of errors.length ? [] : inputs) {
    if (!source.endsWith(".vibe")) continue
    const path = relative(root, source)
    if (path.startsWith("..") || isAbsolute(path)) continue
    const emitted = join(out, path.replace(/\.vibe$/, ".js"))
    const map = JSON.parse(readFileSync(emitted + ".map", "utf8"))
    map.sources = map.sources.map((entry) => slash(resolve(dirname(emitted), map.sourceRoot || "", entry)))
    map.sourceRoot = ""
    const result = { code: readFileSync(emitted, "utf8").replace(/\/\/# sourceMappingURL=.*$/m, ""), map }
    // Bundlers ask by the listed path; Node asks by the real path.
    outputs.set(source, result)
    outputs.set(realpath(source), result)
  }
  return { outputs, inputs, errors, warnings }
}

/**
 * projectBackend compiles a whole tsconfig project with one vibelang process,
 * in a fresh output directory each time. compile(project) resolves to the
 * result described above.
 */
export function projectBackend({ root, compiler }) {
  function run(args) {
    const [file, argv] = command(compiler, args)
    return new Promise((accept, reject) => {
      let output = ""
      const child = spawn(file, argv, { cwd: root, stdio: ["ignore", "pipe", "pipe"] })
      child.stdout.on("data", (data) => { output += data })
      child.stderr.on("data", (data) => { output += data })
      child.on("error", (error) => reject(new Error(`vibelang: ${error.message}\nCompiler: ${compiler}`)))
      child.on("close", (status, signal) => accept({ status, signal, output }))
    })
  }

  async function compile(project) {
    // Outside the project: webpack and esbuild rebuild when a directory they resolved through changes.
    const out = mkdtempSync(join(tmpdir(), "vibelang-project-"))
    try {
      return collect(await run(emitArgs(project, root, out)), root, out)
    } finally {
      // Outputs are in memory now. The next compile starts from an empty directory.
      rmSync(out, { recursive: true, force: true })
    }
  }

  return { compile }
}

/**
 * compileProjectSync is projectBackend's compile for callers that cannot wait,
 * such as Node's synchronous module hooks.
 */
export function compileProjectSync({ project, root = dirname(project), compiler = findCompiler(root) }) {
  const out = mkdtempSync(join(tmpdir(), "vibelang-project-"))
  try {
    const [file, argv] = command(compiler, emitArgs(project, root, out))
    const run = spawnSync(file, argv, { cwd: root, encoding: "utf8", maxBuffer: 1 << 30 })
    if (run.error) throw new Error(`vibelang: ${run.error.message}\nCompiler: ${compiler}`)
    return collect({ status: run.status, signal: run.signal, output: run.stdout + run.stderr }, root, out)
  } finally {
    rmSync(out, { recursive: true, force: true })
  }
}

/** Returns the nearest tsconfig.json above file, or undefined. */
export function nearestProject(file) {
  for (let directory = dirname(file); ; directory = dirname(directory)) {
    const candidate = join(directory, "tsconfig.json")
    if (existsSync(candidate)) return candidate
    if (dirname(directory) === directory) return undefined
  }
}

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

// Module code for a failed compile. The bundler reports the real error.
const failed = "throw new Error(\"vibelang: this module did not compile; see the build errors\")\n"

/**
 * projects compiles .vibe modules for a bundler, one whole-project compile per
 * tsconfig. A module inside root belongs to `project`; one of another
 * workspace package, outside root or nested inside it with its own package.json
 * and tsconfig.json, belongs to the nearest tsconfig.json above it, its own
 * package's project. Each project
 * compiles once per revision; invalidate() starts a new revision.
 *
 * load(file) resolves to { project, code, map, inputs, errors, warnings, first, error }.
 * errors and warnings are the whole compile's; first is true for the first
 * module loaded from that compile, so a bundler can report them once. `error`
 * says that the project does not include the module.
 */
export function projects({ root, project, compiler, debug = () => {} }) {
  const states = new Map()
  let revision = 0

  function projectOf(file) {
    const path = relative(root, file)
    if (path.startsWith("..") || isAbsolute(path)) return nearestProject(file) ?? project
    // A workspace package nested inside root, such as packages/a/b inside
    // packages/a, compiles with its own tsconfig.json: the innermost package.json
    // below root, with a tsconfig.json between it and the module.
    for (let directory = dirname(file); directory !== root && dirname(directory) !== directory; directory = dirname(directory)) {
      if (!existsSync(join(directory, "package.json"))) continue
      const own = nearestProject(file)
      return own !== undefined && !relative(directory, own).startsWith("..") ? own : project
    }
    return project
  }

  function stateOf(tsconfig) {
    let state = states.get(tsconfig)
    if (state === undefined) {
      const projectRoot = tsconfig === project ? root : dirname(tsconfig)
      state = { tsconfig, backend: projectBackend({ root: projectRoot, compiler }), result: undefined, pending: undefined, compiled: -1, reported: -1 }
      states.set(tsconfig, state)
    }
    return state
  }

  async function compiled(state) {
    while (state.compiled !== revision) {
      if (!state.pending) {
        const started = revision
        const time = Date.now()
        debug(`compile ${state.tsconfig} ${started} started`)
        state.pending = state.backend.compile(state.tsconfig)
          .then((value) => {
            state.result = value
            state.compiled = started
            debug(`compile ${state.tsconfig} ${started} finished in ${Date.now() - time}ms: ${value.outputs.size} outputs, ${value.errors.length} errors`)
            // A file saved during the compile may be missing from its output. esbuild records
            // watch files when a load returns, so it would not rebuild for that save. Compile again.
            const changed = changedSince([state.tsconfig, ...value.inputs], time)
            if (changed) {
              revision++
              debug(`${changed} changed during compile ${started}: revision ${revision}`)
            }
          })
          .finally(() => { state.pending = undefined })
      }
      await state.pending
    }
    return state.result
  }

  return {
    projectOf,
    /** Starts a new revision: every project compiles again on its next load. */
    invalidate() {
      revision++
    },
    async load(file) {
      const state = stateOf(projectOf(file))
      const { outputs, inputs, errors, warnings } = await compiled(state)
      // Every .vibe module shares its project's diagnostics.
      const first = state.reported !== state.compiled
      state.reported = state.compiled
      if (errors.length) return { project: state.tsconfig, code: failed, inputs, errors, warnings, first }
      const output = outputs.get(file)
      return {
        project: state.tsconfig, code: output?.code ?? failed, map: output?.map, inputs, errors, warnings, first,
        error: !output && {
          message: `vibelang: ${file} is not part of ${state.tsconfig}. Add it to the tsconfig's include.`, id: file
        }
      }
    }
  }
}
