import { Effect, Option } from "effect"

export const label = (value: Option.Option<number>) =>
  Effect.succeed(
    value.pipe(
      Option.map((n) => n + 1),
      Option.getOrElse(() => 0)
    )
  )
