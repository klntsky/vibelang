// Compiles every golden case's expected Effeck with the converter's compile worker code and the
// pinned project, so the golden set can never hold an answer that does not compile.
//   node .smithers/flows/convert/score.mjs            compile in-process (needs the worker's project installed)
//   node .smithers/flows/convert/score.mjs --worker http://host:8080   use a running worker
// Exit 1 when any expected output fails. The model's outputs are scored the same way once the
// Smithers convert flow exists (see README.md).
import { goldenCases } from "./build-prompt.mjs"

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
for (const c of goldenCases()) {
  const result = await compileOne(c.expected)
  const ok = result.exitCode === 0 && !result.timedOut && !(result.diagnostics ?? []).some((d) => d.severity === "error")
  if (!ok) failed++
  console.log(`${ok ? "compiled" : "FAILED  "} ${c.name}${ok ? "" : ` ${JSON.stringify(result.diagnostics ?? result).slice(0, 300)}`}`)
}
console.log(`${goldenCases().length - failed} of ${goldenCases().length} expected outputs compile`)
process.exit(failed ? 1 : 0)
