// Run the law suite before publishing any derived specification claims.
import { readFileSync } from "node:fs"
import { join, resolve } from "node:path"
export function kernelFacts(): Record<string, string> {
  const repo = resolve(import.meta.dir, "../..")
  const experiments = join(repo, "experiments")
  const suite = Bun.spawnSync(["bun", "test", "--timeout=30000"], { cwd: experiments, stdout: "pipe", stderr: "pipe" })
  const suiteOutput = suite.stdout.toString() + suite.stderr.toString()
  const ran = /Ran (\d+) tests? across (\d+) files?/.exec(suiteOutput)
  const failed = /(\d+) fail/.exec(suiteOutput)
  if (suite.exitCode !== 0 || ran === null || failed === null || failed[1] !== "0") {
    console.error(suiteOutput.split("\n").slice(-30).join("\n"))
    throw new Error("the kernel law suite in experiments/ failed; refusing to build the docs site")
  }
  const lineCount = (path: string) => readFileSync(join(repo, path), "utf8").split("\n").length - 1
  const facts: Record<string, string> = {
    testCount: ran[1]!,
    testFiles: ran[2]!,
    kernelLines: String(lineCount("packages/runtime/src/kernel/kernel.ts") + lineCount("packages/runtime/src/kernel/internals.ts"))
  }
  console.log("Kernel: " + facts.testCount + " tests across " + facts.testFiles + " files passed.")
  return facts
}
