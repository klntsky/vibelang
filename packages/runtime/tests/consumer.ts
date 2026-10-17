// Compile a consumer against the installed package export and emitted .d.ts files.
import { $bind, $program, $produce, $region, $using, $yield, Effect, Layer, Scope, suspend, type Producer } from "@effeck/runtime"

const bound: Effect.Effect<number> = $bind(42)
const program: Effect.Effect<number> = $program(() => bound)
const producer: Producer<number, never, never> = $produce(() => $yield(42))
const acquire: Effect.Effect<string, never, Scope.Scope> = Effect.acquireRelease(Effect.succeed("resource"), () => Effect.void)
const region: Effect.Effect<string> = $region(scope => $using(scope, acquire))
const callback: Effect.Effect<number> = suspend<number>(resumer => { resumer.resume(42) })
const layer: Layer.Layer<never> = Layer.empty
void [program, producer, region, callback, layer]
