import { Effect } from "vibelang-runtime";
export function work(x: number): Effect<number, never, never> {
  return Effect.succeed(x);
}
