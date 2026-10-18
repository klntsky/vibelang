// Compile a consumer against the installed package export and emitted .d.ts files.
import { $bind, $direct, $done, $fast, $isRun, $program, $settle, $produce, $region, $using, $yield, Effect, Layer, Scope, suspend, type Producer } from "effeck-runtime"

const bound: Effect.Effect<number> = $bind(42)
const program: Effect.Effect<number> = $program(() => bound)
const producer: Producer<number, never, never> = $produce(() => $yield(42))
const acquire: Effect.Effect<string, never, Scope.Scope> = Effect.acquireRelease(Effect.succeed("resource"), () => Effect.void)
const region: Effect.Effect<string> = $region(scope => $using(scope, acquire))
const callback: Effect.Effect<number> = suspend<number>(resumer => { resumer.resume(42) })
const layer: Layer.Layer<never> = Layer.empty
const runs: boolean = $isRun(bound)
const key: typeof $direct = Symbol.for("effeck/direct") as typeof $direct
const body = function* (n: number) { return n + ((yield* Effect.fail("Bad" as const)) as number) }
const entry: (n: number) => number | Effect.Effect<number, "Bad", never> = $fast(body)
const it = body(1), s = it.next()
const step: number | Effect.Effect<number, "Bad", never> = s.done ? $done(s.value) : $settle(it, s)
const settled: Effect.Effect<number, "Bad"> = Effect.gen(function* () { const t = entry(1); return Effect.isEffect(t) ? yield* t : t })
void [program, producer, region, callback, layer, runs, key, step, settled]
