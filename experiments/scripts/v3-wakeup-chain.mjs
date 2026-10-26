// F10 regression check: the same Deferred wake-up chain on effect@3.22.3.
// Run from experiments/: node scripts/v3-wakeup-chain.mjs 100000
// Install the reference runtime: npm install --prefix .cache/effect-v3 effect@3.22.3
// Or set EFFECT_V3_NODE_MODULES to a node_modules directory with effect 3.22.3.
import { join } from "node:path"
import { fileURLToPath, pathToFileURL } from "node:url"
const modules = process.env.EFFECT_V3_NODE_MODULES ?? fileURLToPath(new URL("../.cache/effect-v3/node_modules", import.meta.url))
const Effect = await import(pathToFileURL(join(modules, "effect/dist/esm/Effect.js")).href)
const Deferred = await import(pathToFileURL(join(modules, "effect/dist/esm/Deferred.js")).href)
const N = Number(process.argv[2] ?? 5000)
const result = await Effect.runPromise(Effect.gen(function*() {
  const gates = yield* Effect.forEach(Array.from({ length: N + 1 }), () => Deferred.make())
  const fibers = []
  for (let i = 0; i < N; i++) {
    fibers.push(yield* Effect.fork(Effect.flatMap(Deferred.await(gates[i]), (n) => Deferred.succeed(gates[i + 1], n + 1))))
  }
  yield* Effect.yieldNow()
  yield* Deferred.succeed(gates[0], 0)
  yield* Effect.sleep("200 millis")
  return { finished: yield* Deferred.isDone(gates[N]) }
}))
console.log(`effect 3.22.3, N=${N}:`, JSON.stringify(result))
