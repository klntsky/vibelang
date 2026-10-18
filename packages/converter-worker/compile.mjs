// compile checks one VibeLang source file with the pinned compiler and never runs it.
//
// Each call writes the source into a fresh temporary directory beside the fixed
// project's tsconfig, links the project's installed packages, and runs exactly
// `vibelang --noEmit --pretty false -p <dir>`: no user flags, no emit, and a
// minimal environment with no credentials. Limits: one CPU (GOMAXPROCS=1, plus
// the container's CPU quota), ten seconds of CPU and of wall time, a soft Go
// memory limit under the container's 512 MiB, and bounded output. The network
// is cut per compile with a user and network namespace (`unshare -rn`) where
// the host allows it; otherwise the deployment must deny egress, and the server
// refuses to start if it can reach the internet (see server.mjs).
import { spawn, spawnSync } from "node:child_process"
import { createHash } from "node:crypto"
import { existsSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const here = dirname(fileURLToPath(import.meta.url))

/** The largest source a conversion may compile, in bytes (the converter's own input limit). */
export const maxSourceBytes = 8 * 1024
const maxOutputBytes = 64 * 1024
const maxDiagnostics = 50
const maxMessageLength = 500

/**
 * Settings, from the environment so a deployment can pin them:
 *   VIBELANG_WORKER_PROJECT  the fixed project with installed packages (default ./project)
 *   VIBELANG_WORKER_BINARY   the native compiler (default: the platform package inside the project)
 *   VIBELANG_WORKER_TIMEOUT_MS  wall-clock limit (default 10000)
 *   VIBELANG_WORKER_NETWORK  "unshare" (cut the network per compile) or "policy" (the deployment denies egress)
 */
export function settings(env = process.env) {
  const project = env.VIBELANG_WORKER_PROJECT ?? join(here, "project")
  const binary = env.VIBELANG_WORKER_BINARY ?? platformBinary(project)
  const timeoutMs = Number(env.VIBELANG_WORKER_TIMEOUT_MS ?? 10_000)
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0 || timeoutMs > 2_147_483_647) {
    throw new Error("VIBELANG_WORKER_TIMEOUT_MS must be a positive number no greater than 2147483647")
  }
  const network = env.VIBELANG_WORKER_NETWORK ?? "unshare"
  if (network !== "unshare" && network !== "policy") throw new Error(`VIBELANG_WORKER_NETWORK must be unshare or policy, not ${network}`)
  return { project, binary, timeoutMs, network }
}

function platformBinary(project) {
  const arch = process.arch === "arm64" ? "arm64" : "x64"
  return join(project, "node_modules", `vibelang-${process.platform}-${arch}`, "bin", process.platform === "win32" ? "vibelang.exe" : "vibelang")
}

const sha256 = (data) => createHash("sha256").update(data).digest("hex")

/** Names the compiler and the packages every receipt depends on. Computed once at startup. */
export function identity(config) {
  if (!existsSync(config.binary)) throw new Error(`no compiler at ${config.binary}`)
  const printed = spawnSync(config.binary, ["--version"], { encoding: "utf8", timeout: 10_000 })
  const match = /VibeLang (\S+)/.exec(printed.stdout ?? "")
  if (!match) throw new Error(`cannot read the compiler version: ${(printed.stdout ?? "") + (printed.stderr ?? "")}`)
  const versions = ["vibelang", "vibelang-runtime", "effect"].map((name) => {
    const manifest = join(config.project, "node_modules", name, "package.json")
    if (!existsSync(manifest)) throw new Error(`the fixed project is missing ${name}; install it first`)
    return `${name}@${JSON.parse(readFileSync(manifest, "utf8")).version}`
  })
  const lockfile = join(config.project, "package-lock.json")
  const locked = existsSync(lockfile) ? sha256(readFileSync(lockfile)) : "no-lockfile"
  return {
    vibelangVersion: match[1],
    compilerDigest: `sha256:${sha256(readFileSync(config.binary))}`,
    dependenciesDigest: `sha256:${sha256([...versions, `lock:${locked}`].join("\n"))}`,
    packages: versions
  }
}

/** Parses `--pretty false` diagnostics for the converted file into bounded records. */
export function parseDiagnostics(output) {
  const diagnostics = []
  for (const line of output.split("\n")) {
    const found = /(?:^|\/)main\.vibe\((\d+),(\d+)\): (error|warning) ([A-Z]+\d+): (.*)$/.exec(line)
    if (!found) continue
    diagnostics.push({
      line: Number(found[1]),
      column: Number(found[2]),
      severity: found[3],
      code: found[4],
      message: found[5].slice(0, maxMessageLength)
    })
    if (diagnostics.length === maxDiagnostics) break
  }
  return diagnostics
}

/**
 * Compiles one source. Resolves to `{ exitCode, timedOut, diagnostics, truncated }`; never throws for
 * a bad program, only for a worker fault (missing project or compiler).
 */
export function compile(source, config) {
  if (typeof source !== "string") throw new TypeError("source must be a string")
  if (Buffer.byteLength(source, "utf8") > maxSourceBytes) return Promise.resolve({ refusal: "too_large" })
  if (source.includes("\u0000")) return Promise.resolve({ refusal: "invalid_source" })
  const dir = mkdtempSync(join(tmpdir(), "vibelang-convert-"))
  writeFileSync(join(dir, "main.vibe"), source)
  writeFileSync(join(dir, "tsconfig.json"), readFileSync(join(here, "project", "tsconfig.json")))
  symlinkSync(join(config.project, "node_modules"), join(dir, "node_modules"))
  const argv = ["--noEmit", "--pretty", "false", "-p", dir]
  // prlimit: ten CPU-seconds and no core files; GOMAXPROCS=1: one CPU's worth of threads.
  const limited = ["prlimit", "--cpu=10", "--core=0", "--", config.binary, ...argv]
  const command = config.network === "unshare" ? ["unshare", "--net", "--map-root-user", "--", ...limited] : limited
  const env = { PATH: "/usr/bin:/bin", HOME: dir, TMPDIR: dir, GOMAXPROCS: "1", GOMEMLIMIT: "400MiB", LANG: "C.UTF-8" }
  return new Promise((resolve) => {
    const child = spawn(command[0], command.slice(1), { cwd: dir, env, stdio: ["ignore", "pipe", "pipe"] })
    let output = ""
    let truncated = false
    const take = (chunk) => {
      if (output.length >= maxOutputBytes) { truncated = true; return }
      output += chunk.toString("utf8").slice(0, maxOutputBytes - output.length)
    }
    child.stdout.on("data", take)
    child.stderr.on("data", take)
    let timedOut = false
    const timer = setTimeout(() => { timedOut = true; child.kill("SIGKILL") }, config.timeoutMs)
    child.on("close", (code, signal) => {
      clearTimeout(timer)
      rmSync(dir, { recursive: true, force: true })
      const diagnostics = parseDiagnostics(output)
      resolve({ exitCode: code ?? (signal ? 128 : 1), timedOut, diagnostics, truncated, ...(code !== 0 && diagnostics.length === 0 ? { workerOutput: output.slice(0, 2000) } : {}) })
    })
    child.on("error", (error) => {
      clearTimeout(timer)
      rmSync(dir, { recursive: true, force: true })
      resolve({ exitCode: 127, timedOut: false, diagnostics: [], truncated: false, workerOutput: String(error.message).slice(0, 500) })
    })
  })
}

/**
 * Proves the compile environment has no network: tries one outbound TCP connection inside the same
 * wrapper a compile uses. Resolves true when the connection fails, as it must.
 */
export function proveNoEgress(config, target = { host: "1.1.1.1", port: 443 }) {
  const probe = `const s=require("net").connect(${target.port},${JSON.stringify(target.host)});s.on("connect",()=>{console.log("EGRESS");process.exit(0)});s.on("error",()=>process.exit(3));setTimeout(()=>process.exit(4),3000)`
  const base = [process.execPath, "-e", probe]
  const command = config.network === "unshare" ? ["unshare", "--net", "--map-root-user", "--", ...base] : base
  const result = spawnSync(command[0], command.slice(1), { encoding: "utf8", timeout: 10_000 })
  // Only the probe's explicit connection-error or timeout exits prove a blocked connection.
  // Missing tools, namespace failures, signals, and spawn timeouts must fail closed.
  return !result.error && [3, 4].includes(result.status) && !(result.stdout ?? "").includes("EGRESS")
}
