import { expect, test } from "bun:test"
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { classify, defaultCompiler, extract } from "./check-snippets"

const marker = String.fromCharCode(96).repeat(3)
const fence = (code: string) => [marker + "ts effect", code, marker].join("\n")

test("complete Effect comparisons are checked and unresolved names need an explicit fragment", () => {
  const block = extract(fence('import { Console } from "effect"\nconst ready = Console.log("ready")'))[0]
  expect(block?.language).toBe("ts")
  expect(classify(block, [{ code: "TS2304", message: "Cannot find name work" }]).status).toBe("fail")
  expect(extract(fence("// fragment\nyield* work"))[0].fragment).toBe(true)
})

test.skipIf(!existsSync(defaultCompiler()))("a broken Effect namespace fails at its Markdown line while Console passes", async () => {
  const root = mkdtempSync(join(tmpdir(), "effect-comparison-"))
  try {
    const output = join(root, "report.json")
    writeFileSync(join(root, "page.md"), [
      fence('import { Effect } from "effect"\nconst ready = Effect.Console.log("ready")'),
      fence('import { Console } from "effect"\nconst ready = Console.log("ready")'),
      fence("// fragment\nconst value = yield* externalProgram")
    ].join("\n"))
    const child = Bun.spawn(["bun", join(import.meta.dir, "check-snippets.ts"), "--root", root, "--json", output], { stdout: "pipe", stderr: "pipe" })
    const [, stderr, status] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
    expect(stderr).toBe("")
    expect(status).toBe(1)
    const report = JSON.parse(readFileSync(output, "utf8"))
    expect(report.totals).toMatchObject({ blocks: 3, ok: 1, fail: 1, fragment: 1 })
    expect(report.pages[0].blocks[0].diagnostics[0]).toMatchObject({ code: "TS2339", line: 3 })
  } finally { rmSync(root, { recursive: true, force: true }) }
})
