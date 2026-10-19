import * as Effect from "effect/Effect"
import * as Effectable from "effect/Effectable"

let runs = 0
export class Action extends Effectable.Class<number> {
  constructor(readonly value: number) { super() }
  /** @vibelang trusted */
  static make(value: number): Action { return new Action(value) }
  asEffect(): Effect.Effect<number> {
    return Effect.sync(() => { runs++; return this.value })
  }
}
export const implement = (execute: (n: number) => Effect.Effect<unknown>): Effect.Effect<unknown> =>
  Effect.suspend(() => execute(1))
/** @vibelang trusted */
export const runCount = (): number => runs
export const implementNumber = (execute: (n: number) => Effect.Effect<number>): Effect.Effect<number> =>
  Effect.suspend(() => execute(1))
export const implementFailure = (execute: (n: number) => Effect.Effect<unknown, string>): Effect.Effect<unknown, string> =>
  Effect.suspend(() => execute(1))
