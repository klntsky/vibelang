import { spawn } from "node:child_process"

// Build first so Node always starts with an entry point and valid imports.
const build = spawn("effeck", [], { stdio: "inherit" })
build.on("error", fail)
build.on("exit", (code, signal) => {
  if (code !== 0 || signal) process.exit(code || 1)
  const compiler = spawn("effeck", ["--watch"], { stdio: "inherit" })
  const runner = spawn(process.execPath, ["--watch", "--enable-source-maps", "dist/main.js"], { stdio: "inherit" })
  const children = [compiler, runner]
  let stopping = false
  const stop = (code) => {
    if (stopping) return
    stopping = true
    for (const child of children) child.kill("SIGTERM")
    process.exitCode = code
  }
  for (const child of children) {
    child.on("error", (error) => { console.error(error.message); stop(1) })
    child.on("exit", (code) => stop(code ?? 1))
  }
  process.on("SIGINT", () => stop(130))
  process.on("SIGTERM", () => stop(143))
})

function fail(error) {
  console.error(`dev: ${error.message}. Install the project's dependencies first, with bun install or npm install.`)
  process.exitCode = 1
}
