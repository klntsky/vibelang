// Whole-project compilation for vibelang/register. vibelang-vite and
// vibelang-unplugin still carry their own async copies of this logic.
import { spawnSync } from "node:child_process"
import { closeSync, mkdtempSync, openSync, readFileSync, readSync, realpathSync, rmSync } from "node:fs"
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

/**
 * compileProjectSync compiles the whole tsconfig project with one vibelang
 * process. VibeLang's emit is type-directed: a module's JavaScript depends on
 * the contracts of the modules it imports, so no module compiles alone.
 *
 * It returns:
 *   outputs      Map from each .vibe source, by listed and real path, to its { code, map }
 *   inputs       project source files the result depends on
 *   errors       diagnostics that blocked emission; outputs is then empty
 *   warnings     diagnostics of a successful compile
 */
export function compileProjectSync({ project, root = dirname(project), compiler = findCompiler(root) }) {
  const out = mkdtempSync(join(tmpdir(), "vibelang-project-"))
  try {
    const [file, argv] = command(compiler, emitArgs(project, root, out))
    const run = spawnSync(file, argv, { cwd: root, encoding: "utf8", maxBuffer: 1 << 30 })
    if (run.error) throw new Error(`vibelang: ${run.error.message}\nCompiler: ${compiler}`)
    const output = run.stdout + run.stderr
    const { files, diagnostics } = parseOutput(output, root)
    // Dependencies are contracts too; their sources change only on install.
    const inputs = files.filter((path) => !slash(path).includes("/node_modules/"))
    const errors = diagnostics.filter((diagnostic) => diagnostic.category === "error")
    const warnings = diagnostics.filter((diagnostic) => diagnostic.category === "warning")
    if (run.status !== 0 && !errors.length) {
      // A crash or an unknown flag prints no diagnostic; report the raw output.
      errors.push({ category: "error", code: "vibelang", text: output.trim() || `exited with ${run.signal || run.status}` })
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
      // Callers ask by the listed path or, as Node does, by the real path.
      outputs.set(source, result)
      outputs.set(realpath(source), result)
    }
    return { outputs, inputs, errors, warnings }
  } finally {
    rmSync(out, { recursive: true, force: true })
  }
}
