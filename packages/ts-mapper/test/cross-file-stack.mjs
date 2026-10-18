// Exercise Emit's imported-call boundary against the runtime built by e2e.sh.
// Keep locals live across the call, as runtime/tests/fastcalls.test.ts does:
// an eager cross-file cycle overflows far below the 100,000-call assertion.
import assert from "node:assert/strict"
import fs from "node:fs"
import path from "node:path"
import { spawnSync } from "node:child_process"

const [mapper, work] = process.argv.slice(2)
// Any supported Node: an eager cross-file cycle overflows near 1,000 levels on each.
assert.ok(Number(process.versions.node.split(".")[0]) >= 22, "Node 22 or later")
const locals = Array.from({ length: 80 }, (_, i) => `let v${i} = n + ${i}`).join(";\n")
const sum = Array.from({ length: 80 }, (_, i) => `v${i}`).join(" + ")

// body retains all 80 locals while descending and checks every return frame.
// probe is unknown to the mapper but returns plain data: it establishes
// impurity without adding a suspension that could hide an eager-call bug.
function body(name, callee, decrement) {
  return `export function ${name}(n: number) {
  probe(n)
  if (n === 0) return 0
  ${locals}
  const r = ${callee}(n${decrement ? " - 1" : ""})
  return r + ${decrement ? 1 : 0} + (${sum}) - (${sum})
}\n`
}

for (const helpers of [false, true]) {
  const dir = path.join(work, helpers ? "cross-file-helpers" : "cross-file")
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, "probe.js"), "export const probe = n => n\n")
  for (const [file, own, other, target] of [["a", "f", "b", "g"], ["b", "g", "a", "f"]]) {
    const source = `import { ${target} } from "./${other}.vibe"\nimport { probe } from "./probe.js"\n` +
      body(own, helpers ? `${own}Helper` : target, !helpers) +
      (helpers ? body(`${own}Helper`, target, true) : "")
    const input = path.join(dir, `${file}.vibe`)
    const output = path.join(dir, `${file}.js`)
    fs.writeFileSync(input, source)
    const emitted = spawnSync(mapper, ["--emit", input, "--out", output], { encoding: "utf8" })
    assert.equal(emitted.status, 0, emitted.stdout + emitted.stderr)
    assert.equal(emitted.stderr, "", "Emit diagnostics")
  }
  fs.writeFileSync(path.join(dir, "run.mjs"), `
import { Effect } from "effect"
import { f } from "./a.js"
import assert from "node:assert/strict"
assert.equal(await Effect.runPromise(f(100000)), 100000)
console.log("ok   cross-file ${helpers ? "with acyclic helpers" : "direct"}: 100000")
`)
  const ran = spawnSync(process.execPath, [path.join(dir, "run.mjs")], { encoding: "utf8", timeout: 60000 })
  assert.equal(ran.status, 0, ran.stdout + ran.stderr)
  process.stdout.write(ran.stdout)
}
