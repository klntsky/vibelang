// pipe is effect's pipe on plain data (D8): generic stages such as Match's
// and Layer's infer from the value before them. With a program first, a
// generic error operator sees the program's failure type.
import { Context, Match } from "effect"

class Missing extends TaggedError("Missing")<{ readonly name: string }> {}

const describe = pipe(
  Match.type<number | string>(),
  Match.when(Match.number, (n) => `count ${n}`),
  Match.when(Match.string, (s) => `label ${s}`),
  Match.exhaustive
)

function phase(value: "draft" | "published") {
  return pipe(
    Match.value(value),
    Match.when("draft", () => "Draft"),
    Match.orElse((other) => `other ${other}`)
  )
}

class Reports extends Context.Service<Reports, { readonly title: string }>()("Reports") {}

const ReportsUnavailable = Layer.effect(Reports, () => {
  throw new Missing({ name: "reports" })
})

const Observed = pipe(
  ReportsUnavailable,
  Layer.tapError((error) => Effect.log(`unavailable: ${error.name}`))
)

const Resilient = Layer.catch(Observed, () => Layer.succeed(Reports, { title: "offline" }))

function requireSetting(name: string) {
  throw new Missing({ name })
}

function startup() {
  return pipe(
    () => requireSetting("region"),
    Effect.mapError((error) => `needs ${error.name}`),
    Effect.catch((message) => Effect.succeed(message))
  )
}

function title() {
  return Reports().title
}

// expect-result: {"numbers":"count 1","strings":"label a","phase":["Draft","other published"],"startup":"needs region","title":"offline"}
export function main() {
  return {
    numbers: describe(1),
    strings: describe("a"),
    phase: [phase("draft"), phase("published")],
    startup: startup(),
    title: Effect.provide(() => title(), Resilient)
  }
}
