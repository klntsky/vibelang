import { spawn } from "node:child_process"
import { mkdtemp, readFile, rm } from "node:fs/promises"
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

/**
 * projectBackend compiles the whole tsconfig project with one vibelang process.
 * VibeLang's emit is type-directed: a module's JavaScript depends on the
 * contracts of the modules it imports, so no module compiles alone.
 *
 * compile(project) resolves to:
 *   outputs      Map from each .vibe source path to its { code, map }
 *   inputs       project source files the result depends on
 *   errors       diagnostics that blocked emission; outputs is then empty
 *   warnings     diagnostics of a successful compile
 */
export function projectBackend({ root, compiler }) {
  async function run(args) {
    const command = /\.(?:[cm]?js)$/.test(compiler) || compiler.endsWith("/bin/vibelang")
      ? [process.execPath, [compiler, ...args]] : [compiler, args]
    return new Promise((accept, reject) => {
      let output = ""
      const child = spawn(command[0], command[1], { cwd: root, stdio: ["ignore", "pipe", "pipe"] })
      child.stdout.on("data", (data) => { output += data })
      child.stderr.on("data", (data) => { output += data })
      child.on("error", (error) => reject(new Error(`vibelang: ${error.message}\nCompiler: ${compiler}`)))
      child.on("close", (code, signal) => accept({ code, signal, output }))
    })
  }

  async function compile(project) {
    // Outside the project: webpack and esbuild rebuild when a directory they resolved through changes.
    const out = await mkdtemp(join(tmpdir(), "vibelang-unplugin-"))
    try {
      const { code, signal, output } = await run([
        "--project", project, "--pretty", "false", "--listFiles", "true", "--rootDir", root,
        "--outDir", out, "--sourceMap", "true", "--inlineSources", "true",
        "--inlineSourceMap", "false", "--sourceRoot", slash(root) + "/", "--declaration", "false", "--declarationMap", "false",
        "--noEmit", "false", "--emitDeclarationOnly", "false", "--noEmitOnError", "true",
        "--incremental", "false"
      ])
      const { files, diagnostics } = parseOutput(output, root)
      // Dependencies are contracts too; their sources change only on install.
      const inputs = files.filter((file) => !slash(file).includes("/node_modules/"))
      const errors = diagnostics.filter((diagnostic) => diagnostic.category === "error")
      const warnings = diagnostics.filter((diagnostic) => diagnostic.category === "warning")
      if (code !== 0 && !errors.length) {
        // A crash or an unknown flag prints no diagnostic; report the raw output.
        errors.push({ category: "error", code: "vibelang", text: output.trim() || `exited with ${signal || code}` })
      }
      const outputs = new Map()
      if (!errors.length) {
        for (const file of inputs) {
          if (!file.endsWith(".vibe")) continue
          const path = relative(root, file)
          if (path.startsWith("..") || isAbsolute(path)) continue
          const emitted = join(out, path.replace(/\.vibe$/, ".js"))
          const [javascript, rawMap] = await Promise.all([readFile(emitted, "utf8"), readFile(emitted + ".map", "utf8")])
          const map = JSON.parse(rawMap)
          map.sources = map.sources.map((source) => slash(resolve(dirname(emitted), map.sourceRoot || "", source)))
          map.sourceRoot = ""
          outputs.set(file, { code: javascript.replace(/\/\/# sourceMappingURL=.*$/m, ""), map })
        }
      }
      return { outputs, inputs, errors, warnings }
    } finally {
      // Outputs are in memory now. The next compile starts from an empty directory.
      await rm(out, { recursive: true, force: true })
    }
  }

  return { compile }
}
