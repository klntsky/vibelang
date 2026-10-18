// The compile worker's HTTP surface. Smithers' convert flow calls it from a fixed activity; nothing
// else should reach it (see deploy/network-policy.yaml).
//
//   POST /compile  {"source": "<VibeLang>"}  ->  {exitCode, timedOut, diagnostics[], truncated,
//                                               vibelangVersion, compilerDigest, dependenciesDigest}
//   GET  /healthz                          ->  {ok, vibelangVersion, compilerDigest, dependenciesDigest, network}
//
// One compile runs at a time: the container's one CPU and 512 MiB are the per-compile limits. The
// server refuses to start when the compile environment can reach the internet.
import { createServer } from "node:http"
import { compile, identity, maxSourceBytes, proveNoEgress, settings } from "./compile.mjs"

const config = settings()
const id = identity(config)
if (!proveNoEgress(config)) {
  console.error(`converter-worker: the compile environment reached the internet (network mode ${config.network}); refusing to start`)
  process.exit(1)
}

let busy = Promise.resolve()
const serial = (task) => {
  const run = busy.then(task, task)
  busy = run.catch(() => {})
  return run
}

const send = (res, status, body) => {
  res.writeHead(status, { "content-type": "application/json", "cache-control": "no-store" })
  res.end(JSON.stringify(body))
}

const server = createServer((req, res) => {
  if (req.method === "GET" && req.url === "/healthz") {
    return send(res, 200, { ok: true, vibelangVersion: id.vibelangVersion, compilerDigest: id.compilerDigest, dependenciesDigest: id.dependenciesDigest, network: config.network })
  }
  if (req.method !== "POST" || req.url !== "/compile") return send(res, 404, { error: "not_found" })
  let size = 0
  const chunks = []
  req.on("data", (chunk) => {
    size += chunk.length
    if (size > maxSourceBytes * 2 + 1024) { send(res, 413, { refusal: "too_large" }); req.destroy() } else chunks.push(chunk)
  })
  req.on("end", async () => {
    if (res.writableEnded) return
    let source
    try { source = JSON.parse(Buffer.concat(chunks).toString("utf8")).source } catch { return send(res, 400, { error: "invalid_json" }) }
    if (typeof source !== "string") return send(res, 400, { error: "source_required" })
    let result
    try {
      result = await serial(() => compile(source, config))
    } catch {
      // Worker faults must not terminate the HTTP process or expose input,
      // filesystem paths, credentials, or exception messages to clients.
      console.error(JSON.stringify({ event: "compile_failed", stage: "compile" }))
      return send(res, 500, { error: "compile_failed" })
    }
    if (result.refusal) return send(res, 422, { refusal: result.refusal })
    // Logs carry stage outcomes only: never the source, the diagnostics' text or a client address.
    console.log(JSON.stringify({ event: "compile", exitCode: result.exitCode, timedOut: result.timedOut, diagnostics: result.diagnostics.length }))
    send(res, 200, { ...result, vibelangVersion: id.vibelangVersion, compilerDigest: id.compilerDigest, dependenciesDigest: id.dependenciesDigest })
  })
})

const port = Number(process.env.PORT ?? 8080)
server.listen(port, () => console.log(JSON.stringify({ event: "listening", port, vibelangVersion: id.vibelangVersion, network: config.network })))
