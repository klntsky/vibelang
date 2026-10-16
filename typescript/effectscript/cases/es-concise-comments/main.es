// A concise arrow body that the lowering turns into a block keeps the
// comment after it outside the block.
import { Effect } from "effect"

abstract class Settings {
  abstract readonly greeting: string
}

function findName(id: number) {
  return id === 1 ? Effect.succeed("ada") : Effect.succeed("grace")
}

export const greeting = () => Settings().greeting
// Effect<string, never, Settings>

export const nameLength = () => findName(1).length
// Effect<number, never, never>
