import { Context, Effect, Schedule, Schema } from "effect"

class ApiError extends Schema.TaggedError<ApiError>()("ApiError", {
  status: Schema.Number
}) {}

class Users extends Context.Service<Users, {
  name(id: string): Effect.Effect<string, ApiError>
}>()("app/Users") {}

const nameOf = Effect.fn("nameOf")(function* (id: string) {
  const users = yield* Users
  return yield* users.name(id).pipe(
    Effect.retry(Schedule.exponential("100 millis")),
    Effect.timeout("2 seconds"),
    Effect.orElseSucceed(() => "unknown")
  )
})

export const names = Effect.fn("names")(function* (ids: ReadonlyArray<string>) {
  const all = yield* Effect.forEach(ids, nameOf, { concurrency: 8 })
  yield* Effect.log(`Loaded ${all.length} names`)
  return all
})
