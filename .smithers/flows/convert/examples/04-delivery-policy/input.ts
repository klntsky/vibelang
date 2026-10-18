import { Data, Effect, Schedule } from "effect"

class QueueBusy extends Data.TaggedError("QueueBusy")<{ readonly queue: string }> {}

declare const enqueue: (job: string) => Effect.Effect<string, QueueBusy>

export const submit = (job: string) => enqueue(job).pipe(
  Effect.retry(Schedule.recurs(4)),
  Effect.timeout("750 millis"),
  Effect.orElseSucceed(() => "pending")
)

export const heartbeat = Effect.log("queue alive").pipe(Effect.repeat(Schedule.recurs(5)))
