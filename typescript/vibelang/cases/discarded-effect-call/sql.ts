import { Effect } from "effect"
export const events: string[] = []
export interface Statement extends Effect.Effect<void, { readonly _tag: "SqlError" }> { readonly sql: string }
export const sql = {
  /** @vibelang trusted */
  unsafe(text: string): Statement {
    return Object.assign(Effect.suspend(() => {
      if (text === "fail") return Effect.fail({ _tag: "SqlError" as const })
      events.push(text)
      return Effect.void
    }), { sql: text })
  }
}
