import { Data, Effect } from "effect"
import { trusted } from "vibelang-runtime"

export class PortError extends Data.TaggedError("PortError")<{ readonly text: string }> {}
let attempts = 0
export function parsePortImpl(text: string): number {
  attempts++
  if (text === "boom") throw new RangeError("unexpected")
  const port = Number(text)
  if (!Number.isInteger(port)) throw new PortError({ text })
  return port
}
export const parsePort = trusted(parsePortImpl, [PortError])
export const throwingArgument = trusted((): string => { throw "argument" }, [])
export const calls = () => Effect.sync(() => attempts)
