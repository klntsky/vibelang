import { Data, Effect } from "effect"

class Oops extends Data.TaggedError("Oops")<{ readonly n: number }> {}

const step = Effect.fnUntraced(function* (payload: number) {
  if (payload < 0) return yield* Effect.fail(new Oops({ n: payload }))
  return payload
})

export function label(s: "draft" | "review" | "published"): string {
  switch (s) {
    case "draft": return "D"
    case "review": return "R"
    case "published": return "P"
  }
}

export function sized(n: number): string {
  let kind: string
  switch (n) {
    case 0: kind = "zero"; break
    default: kind = "some"
  }
  return kind
}

export const describe = Effect.fnUntraced(function* (n: number) {
  const v = yield* step(n)
  switch (v) {
    case 0: return "zero"
    default: return "some"
  }
})
