// expect: ES3201 TS2322
// A runner's program may need only what the runner provides (D38). ES3201
// names each service that nothing provides, at a program runner and at a
// test, in place of TypeScript's error about the argument's type. A test
// that layer(…) registers may need that layer's services.
import { Effect } from "effect"
import { NodeRuntime } from "@effect/platform-node"
import { it, layer } from "@effect/vitest"

abstract class Todos {
  abstract list(): Effect.Effect<ReadonlyArray<string>, never, never>
}

abstract class Clock2 {
  abstract now(): Effect.Effect<number, never, never>
}

const TodosTest = Layer.succeed(Todos, { list: () => ["a"] })

function count() {
  return Todos.list().length
}

function stamped() {
  return `${count()} at ${Clock2.now()}`
}

NodeRuntime.runMain(() => {
  Effect.log(count())
})

Effect.runPromise(() => stamped())

it.effect("counts", () => {
  count()
})

layer(TodosTest)("with todos", (it) => {
  it.effect("counts", () => {
    count()
  })
})

it.effect("provides its own", () => {
  Effect.provide(() => count(), TodosTest)
})

it.effect("keeps the errors in its body", () => {
  const total: string = count()
})
