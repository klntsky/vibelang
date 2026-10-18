// A TypeScript module imports the .vibe module as ./welcome.js, as it imports
// a .ts module (D31), and sees welcome as a function that returns an Effect.
import { Effect } from "effect"
import { welcome } from "./welcome.js"

export const greetTwice = (name: string): Effect.Effect<string, unknown> =>
  Effect.map(Effect.zip(welcome(name), welcome(name)), ([a, b]) => `${a} ${b}`)
