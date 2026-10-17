#!/usr/bin/env bun
// Check the examples with the executable language reference while effeck is built.
import fs from "node:fs"
import path from "node:path"
import { spawnSync } from "node:child_process"
import { compile } from "../prototype/esc/src/compiler.ts"
import { report } from "../prototype/esc/src/cli.ts"

const root = path.resolve(import.meta.dir, "..")
const names = ["hello", "users", "http-api", "file-tree", "resilience", "approval", "pipeline"]
let failed = false
for (const name of names) {
  const dir = path.join(root, "examples", name)
  const staging = path.join(dir, ".prototype", "src")
  const output = path.join(dir, ".prototype", "output")
  const log = path.join(dir, ".prototype", "check.log")
  fs.mkdirSync(staging, { recursive: true })
  fs.mkdirSync(output, { recursive: true })
  // Keep disposable emitted modules and their source maps inside the project
  // so package imports resolve through its installed dependencies.
  for (const file of fs.readdirSync(path.join(dir, "src"))) {
    if (!file.endsWith(".effeck")) continue
    const source = fs.readFileSync(path.join(dir, "src", file), "utf8")
    fs.writeFileSync(path.join(staging, file), source)
  }
  const entry = "main.effeck"
  const test = fs.existsSync(path.join(staging, "main.test.effeck"))
  const entries = [path.join(staging, entry)]
  if (test) entries.push(path.join(staging, "main.test.effeck"))
  if (name !== "file-tree" && fs.existsSync(path.join(staging, "run.effeck"))) {
    entries.push(path.join(staging, "run.effeck"))
  }
  fs.writeFileSync(log, "")
  try {
    const result = compile(entries, { outDir: output })
    fs.writeFileSync(log, report(result) + "\n")
    if (result.diagnostics.some((d) => d.severity === "error")) throw new Error("compile")
    const expected = path.join(dir, "expected-output.txt")
    if (fs.existsSync(expected)) {
      // Execute the emitted entry once. The prototype CLI's run subcommand
      // would additionally invoke exported main, duplicating hello's runner.
      const module = path.join(output, "main.ts")
      const launcher = name === "hello" ? module : path.join(output, "launch.ts")
      if (name !== "hello") {
        fs.writeFileSync(launcher, 'import { Effect } from "effect"\nimport { main } from "./main.ts"\nawait Effect.runPromise(main())\n')
      }
      const run = spawnSync(process.execPath, [launcher], {
        cwd: dir, encoding: "utf8", timeout: 15000, killSignal: "SIGKILL"
      })
      fs.appendFileSync(log, (run.stdout ?? "") + (run.stderr ?? ""))
      if (run.error) fs.appendFileSync(log, run.error.stack ?? run.error.message)
      if (run.error || run.status !== 0) throw new Error("run")
      if (run.stdout !== fs.readFileSync(expected, "utf8")) throw new Error("output")
    }
    if (test) {
      const config = path.join(dir, ".prototype", "vitest.config.ts")
      fs.writeFileSync(config, 'import { defineConfig } from "vitest/config"\nexport default defineConfig({ test: { include: [".prototype/output/*.test.ts"] } })\n')
      const run = spawnSync(process.execPath, ["x", "--no-install", "vitest", "run", "--config", config], {
        cwd: dir, encoding: "utf8", timeout: 30000, killSignal: "SIGKILL"
      })
      fs.appendFileSync(log, (run.stdout ?? "") + (run.stderr ?? ""))
      if (run.error) fs.appendFileSync(log, run.error.stack ?? run.error.message)
      if (run.error || run.status !== 0) throw new Error("tests")
    }
    console.log(`OK   ${name.padEnd(10)} prototype main compile${fs.existsSync(expected) ? ", output" : ""}${test ? ", tests" : ""}${name === "file-tree" ? " (recursive E/R unknown)" : ""}`)
  } catch (error) {
    fs.appendFileSync(log, `\n${(error as Error).stack ?? String(error)}\n`)
    console.log(`FAIL ${name.padEnd(10)} ${(error as Error).message} (see examples/${name}/.prototype/check.log)`)
    failed = true
  }
}
process.exitCode = failed ? 1 : 0
