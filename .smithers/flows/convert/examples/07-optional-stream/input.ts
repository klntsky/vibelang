import { Effect, Option, Stream } from "effect"

export const summary = (prefix: Option.Option<string>) => Effect.gen(function* () {
  const heading = prefix.pipe(Option.map((s) => s.toUpperCase()), Option.getOrElse(() => "READINGS"))
  const readings = yield* Stream.make(3, 8, 13, 21).pipe(
    Stream.filter((n) => n > 5),
    Stream.map((n) => n * 10),
    Stream.take(2),
    Stream.runCollect
  )
  return `${heading}: ${Array.from(readings).join(", ")}`
})
