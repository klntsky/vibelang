import { Effect, Stream } from "effect"

export const hot = Stream.make(18, 25, 30).pipe(
  Stream.filter((c) => c >= 25),
  Stream.map((c) => `${c} C`),
  Stream.runCollect,
  Effect.map((chunk) => Array.from(chunk))
)
