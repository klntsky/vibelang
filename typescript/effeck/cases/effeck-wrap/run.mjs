// JavaScript sees each lowered function as a function that returns an Effect.
import { Effect } from "effect"
import { answer, half } from "./out/main.js"

console.log(Effect.runSync(answer()), Effect.runSync(half(84)))
