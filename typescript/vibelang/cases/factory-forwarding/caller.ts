import { Effect } from "effect"
import { wrap } from "./main.js"

export function check(): string {
  const calls: string[] = []
  let runs = 0
  const base = {
    read(path: string) {
      calls.push(path)
      return Effect.sync(() => ++runs)
    }
  }
  const program = wrap(base).read("abc")
  if (JSON.stringify(calls) !== '["abc"]' || runs !== 0) {
    throw new Error("forwarding must call the delegate immediately, without running it")
  }
  if (Effect.runSync(program) !== 1 || Effect.runSync(program) !== 2) {
    throw new Error("the returned program must run on every run")
  }
  if (JSON.stringify(calls) !== '["abc"]') throw new Error("running must not call the delegate again")
  return "delegate: call time; program: twice"
}
