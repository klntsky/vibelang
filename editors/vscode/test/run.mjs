// Tests the packaged extension in a real VS Code. It copies the language
// server fixture to a scratch project that has vibelang
// installed with the compiler under test as its native binary, installs
// vibelang.vsix into a fresh VS Code profile, opens the project, and runs
// test/harness/index.js inside VS Code.
//
// Usage: bun run package && node test/run.mjs
//
// Environment:
//   VIBELANG                  the vibelang binary (default: ../../typescript/built/local/vibelang)
//   EFFECT_NODE_MODULES  a node_modules directory that holds effect 4.0.5
//                        (default: ../../experiments/node_modules, else
//                        ~/effectscript/experiments/node_modules)
//   VIBELANG_RUNTIME the built vibelang-runtime package directory
//                        (default: ../../packages/runtime)
//   VSCODE_EXECUTABLE    an installed VS Code to test with, such as
//                        "/Applications/Visual Studio Code.app/Contents/MacOS/Electron";
//                        otherwise VS Code VSCODE_VERSION (default 1.98.2) is
//                        downloaded to VSCODE_TEST_CACHE (default .vscode-test/)
//
// On Linux, run it under xvfb-run. On macOS it opens a VS Code window for
// about a minute.
import { spawnSync } from "node:child_process"
import { chmodSync, cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, symlinkSync } from "node:fs"
import os from "node:os"
import path from "node:path"
import { fileURLToPath } from "node:url"
import { downloadAndUnzipVSCode, resolveCliPathFromVSCodeExecutablePath, runTests } from "@vscode/test-electron"

const here = path.dirname(fileURLToPath(import.meta.url))
const extensionRoot = path.resolve(here, "..")
const repo = path.resolve(extensionRoot, "..", "..")
const vsix = path.join(extensionRoot, "vibelang.vsix")
const vibelang = path.resolve(process.env.VIBELANG ?? path.join(repo, "typescript", "built", "local", "vibelang"))
const effectModules = path.resolve(
  process.env.EFFECT_NODE_MODULES ??
    [path.join(repo, "experiments", "node_modules"), path.join(os.homedir(), "vibelang", "experiments", "node_modules")].find(
      (dir) => existsSync(path.join(dir, "effect"))
    ) ??
    path.join(repo, "experiments", "node_modules")
)
const fixture = path.join(repo, "typescript", "vibelang", "lsp-fixture")
const runtime = path.resolve(process.env.VIBELANG_RUNTIME ?? path.join(repo, "packages", "runtime"))

for (const [what, file] of [
  ["the packaged extension; run bun run package", vsix],
  ["vibelang; set VIBELANG", vibelang],
  ["effect; set EFFECT_NODE_MODULES", path.join(effectModules, "effect")],
  ["the built runtime; set VIBELANG_RUNTIME", path.join(runtime, "dist", "index.d.ts")]
]) {
  if (!existsSync(file)) {
    console.error(`test: missing ${what}: ${file}`)
    process.exit(1)
  }
}

// VS Code puts IPC sockets in its user data directory, and macOS limits socket
// paths to about 100 bytes, so keep the scratch directory short there.
const scratch = mkdtempSync(path.join(process.platform === "darwin" ? "/tmp" : os.tmpdir(), "es-vscode-"))
const project = path.join(scratch, "project")
const extensionsDir = path.join(scratch, "extensions")
const userDataDir = path.join(scratch, "user-data")
cpSync(fixture, project, { recursive: true, filter: (file) => path.basename(file) !== "node_modules" })
mkdirSync(path.join(project, "node_modules", ".bin"), { recursive: true })
for (const name of readdirSync(effectModules)) {
  if (name !== ".bin" && name !== "vibelang-runtime") symlinkSync(path.join(effectModules, name), path.join(project, "node_modules", name))
}
symlinkSync(runtime, path.join(project, "node_modules", "vibelang-runtime"))
// node_modules/.bin/vibelang is vibelang's Node launcher, as bun
// install leaves it, and the launcher runs native/vibelang next to it.
const compiler = path.join(project, "node_modules", "vibelang")
mkdirSync(path.join(compiler, "bin"), { recursive: true })
mkdirSync(path.join(compiler, "native"))
cpSync(path.join(repo, "packages", "compiler", "package.json"), path.join(compiler, "package.json"))
cpSync(path.join(repo, "packages", "compiler", "bin", "vibelang"), path.join(compiler, "bin", "vibelang"))
chmodSync(path.join(compiler, "bin", "vibelang"), 0o755)
symlinkSync(vibelang, path.join(compiler, "native", "vibelang"))
symlinkSync("../vibelang/bin/vibelang", path.join(project, "node_modules", ".bin", "vibelang"))

try {
  const vscodeExecutablePath =
    process.env.VSCODE_EXECUTABLE ??
    (await downloadAndUnzipVSCode({
      version: process.env.VSCODE_VERSION ?? "1.98.2",
      cachePath: path.resolve(process.env.VSCODE_TEST_CACHE ?? path.join(extensionRoot, ".vscode-test"))
    }))
  const cli = resolveCliPathFromVSCodeExecutablePath(vscodeExecutablePath)
  const install = spawnSync(cli, [`--extensions-dir=${extensionsDir}`, `--user-data-dir=${userDataDir}`, "--install-extension", vsix], {
    stdio: "inherit"
  })
  if (install.status !== 0) throw new Error(`installing ${vsix} exited with ${install.status}`)
  const pass = (name, ...args) =>
    runTests({
      vscodeExecutablePath,
      extensionDevelopmentPath: path.join(here, "harness"),
      extensionTestsPath: path.join(here, "harness", "index.js"),
      launchArgs: [project, `--extensions-dir=${extensionsDir}`, `--user-data-dir=${userDataDir}`, "--disable-telemetry", ...args],
      extensionTestsEnv: { VIBELANG_TEST_WORKSPACE: project, VIBELANG_TEST_EXTENSIONS: extensionsDir, VIBELANG_TEST_PASS: name }
    })
  // First with VS Code's TypeScript features on, as a new install has them.
  await pass("default")
  // Then with the built-in TypeScript extension disabled, so VibeLang
  // serves .ts files too.
  await pass("typescript-off", "--disable-extension=vscode.typescript-language-features")
  console.log("test: the packaged extension passed")
} catch (error) {
  console.error(`test: ${error instanceof Error ? error.message : String(error)}`)
  process.exitCode = 1
} finally {
  rmSync(scratch, { recursive: true, force: true })
}
