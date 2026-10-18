// Compiles every golden and example expected output with the converter's compile worker
// and pinned project, so neither set can hold an answer that does not compile.
//   node .smithers/flows/convert/score.mjs            compile in-process (needs the worker's project installed)
//   node .smithers/flows/convert/score.mjs --worker http://host:8080   use a running worker
// Exit 1 when any expected output fails. The model's outputs are scored the same way once the
// Smithers convert flow exists (see README.md).
import { fileURLToPath } from "node:url"
import { casesIn } from "./cases.mjs"
import { heldoutProblems } from "./check-heldout.mjs"

const problems = heldoutProblems()
if (problems.length) {
  console.error(problems.join("\n"))
  process.exit(1)
}
const suites = ["golden", "examples"].map((name) => ({ name, cases: casesIn(fileURLToPath(new URL(`./${name}/`, import.meta.url))) }))

const at = process.argv.indexOf("--worker")
const worker = at > 0 ? process.argv[at + 1] : undefined

async function compileOne(source) {
  if (worker) {
    const response = await fetch(new URL("/compile", worker), { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ source }) })
    return response.json()
  }
  const { compile, settings } = await import("../../../packages/converter-worker/compile.mjs")
  return compile(source, settings())
}

let failed = 0
for (const suite of suites) {
  let passed = 0
  for (const c of suite.cases) {
    const result = await compileOne(c.expected)
    const ok = result.exitCode === 0 && !result.timedOut && !(result.diagnostics ?? []).some((d) => d.severity === "error")
    if (ok) passed++
    else failed++
    console.log(`${ok ? "compiled" : "FAILED  "} ${suite.name}/${c.name}${ok ? "" : ` ${JSON.stringify(result).slice(0, 1000)}`}`)
  }
  console.log(`${passed} of ${suite.cases.length} ${suite.name} expected outputs compile`)
}
process.exit(failed ? 1 : 0)
