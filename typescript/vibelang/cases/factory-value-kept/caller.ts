import { Effect } from "effect"
import { nested, setups } from "./main.js"

export function checkNested(): boolean {
  const before = setups
  const outer = nested()
  if (setups !== before) throw new Error("an existing nested Effect must stay cold")
  const inner = Effect.runSync(outer)
  if (setups !== before + 1 || Effect.runSync(inner) !== 6) throw new Error("nested program changed meaning")
  return true
}
