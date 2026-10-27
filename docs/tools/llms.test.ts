import { test } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { buildMarkdown } from "./markdown"

test("site llms tests pass against freshly generated Markdown", () => {
  const dir = mkdtempSync(join(tmpdir(), "vibelang-llms-"))
  try {
    buildMarkdown(dir, { testCount: "123", testFiles: "4", kernelLines: "567" })
    const result = Bun.spawnSync(["node", "--test", resolve(import.meta.dir, "../../site/scripts/llms.test.mjs")], {
      env: { ...process.env, VIBELANG_DOCS_TEST_OUTPUT: dir },
      stdout: "pipe", stderr: "pipe"
    })
    if (result.exitCode !== 0) throw new Error(result.stdout.toString() + result.stderr.toString())
  } finally { rmSync(dir, { recursive: true, force: true }) }
})
