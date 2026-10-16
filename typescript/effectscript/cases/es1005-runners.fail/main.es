// expect: ES1005
// D38: a runner may be called only where no fiber runs the code: at the
// module's top level, and in a plain callback that such a place passes to a
// TypeScript function. Anywhere else it is ES1005.
import { Context, Effect, Layer, ManagedRuntime } from "effect"
import { NodeRuntime } from "@effect/platform-node"
import { describe, it, layer } from "@effect/vitest"
import { onRequest } from "./server.ts"

function refresh(id: string) {
  Effect.sleep("1 millis")
  return id
}

function refreshAll(ids: ReadonlyArray<string>) {
  for (const id of ids) Effect.runFork(() => refresh(id))
}

const runtime = ManagedRuntime.make(Layer.empty)

function viaRuntime() {
  return runtime.runPromise(() => refresh("r"))
}

function withContext() {
  return Effect.runSyncWith(Context.empty())(() => refresh("w"))
}

function nestedMain() {
  NodeRuntime.runMain(() => refresh("m"))
}

const handlers: Array<() => void> = []

function register(f: () => void) {
  handlers.push(f)
}

register(() => {
  Effect.runFork(() => refresh("e"))
})

it.effect("a test", () => {
  it.effect("nested", () => refresh("n"))
  return refresh("t")
})

// Allowed: the module's top level, and plain callbacks passed from there to
// TypeScript functions.
Effect.runFork(() => refresh("top"))
NodeRuntime.runMain(() => refresh("main"))
onRequest(() => {
  Effect.runFork(() => refresh("request"))
})
describe("suite", () => {
  it.effect("works", () => refresh("ok"))
  it.effect.each([1, 2])("each", (n) => refresh(String(n)))
})
layer(Layer.empty)("with a layer", (it) => {
  it.effect("works", () => refresh("layer"))
})
