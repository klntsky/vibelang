import { expect, test } from "bun:test"
import { Context, Effect, Exit, Layer } from "effect"
import { handle, operation, resume } from "../../kernel/kernel.ts"
import { note, sync, value } from "./helpers.ts"

class Resource extends Context.Service<Resource, string>()("conformance/Resource") {}

for (const consume of ["resume", "abandon"] as const) {
  test(`S01-${consume} §1.5/1.7 scoped acquireRelease stays open while parked and releases once in LIFO order`, () => {
    const Ask = operation<void, number>(`S01${consume}`)
    const seen: string[] = []
    const acquire = (name: string) => Effect.acquireRelease(Effect.as(note(seen, `acquire:${name}`), name),
      (_r, ex) => Effect.withFiber(f => note(seen, `release:${name}:${ex._tag}:${f.interruptible}`)))
    const body = Effect.scoped(Effect.gen(function*() {
      yield* acquire("one")
      yield* acquire("two")
      return yield* Ask()
    }))
    const exit = sync(handle("h", [Ask], body, {
      [Ask.name]: (_, k) => Effect.andThen(note(seen, "clause"), consume === "resume" ? resume(k, 42) : Effect.succeed(42))
    }))
    expect(value(exit)).toBe(42)
    expect(seen).toEqual(["acquire:one", "acquire:two", "clause",
      `release:two:${consume === "resume" ? "Success" : "Failure"}:false`,
      `release:one:${consume === "resume" ? "Success" : "Failure"}:false`])
  })

  test(`S02-${consume} §1.5/1.7 Layer resource scope crosses capture and releases once`, () => {
    const Ask = operation<void, string>(`S02${consume}`)
    const seen: string[] = []
    const live = Layer.effect(Resource, Effect.acquireRelease(Effect.as(note(seen, "build"), "live"),
      () => note(seen, "release")))
    const body = Effect.provide(Effect.gen(function*() {
      yield* Ask()
      return yield* Resource
    }), live)
    const exit = sync(handle("h", [Ask], body, {
      [Ask.name]: (_, k) => Effect.andThen(note(seen, "clause"), consume === "resume" ? resume(k, "x") : Effect.succeed("stop"))
    }))
    expect(value(exit)).toBe(consume === "resume" ? "live" : "stop")
    expect(seen).toEqual(["build", "clause", "release"])
  })
}
