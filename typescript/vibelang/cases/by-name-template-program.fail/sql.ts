// A Statement-like tag: an Effect subtype with structure, like effect/sql's Statement.
import { Effect } from "effect"
export const ran: string[] = []
export interface Statement extends Effect.Effect<ReadonlyArray<string>> { readonly sql: string }
export const sql = (parts: TemplateStringsArray, ...values: ReadonlyArray<unknown>): Statement => {
  const text = parts.reduce((all, part, index) => all + part + (index < values.length ? String(values[index]) : ""), "")
  return Object.assign(Effect.sync(() => { ran.push(text); return [text] }), { sql: text })
}
/** A program parameter, like DurableWriter.write. */
export const write = <A, E, R>(effect: Effect.Effect<A, E, R>): Effect.Effect<A, E, R> => effect
