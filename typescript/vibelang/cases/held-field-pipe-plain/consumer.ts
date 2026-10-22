import { Effect } from "effect"
import { make } from "./lib.js"

// A TypeScript caller sees what the declaration says: a record whose field is
// a program, not an Effect that builds the record.
export const consumerSeesRecord = (): boolean => !Effect.isEffect(make()) && Effect.isEffect(make().changes)
