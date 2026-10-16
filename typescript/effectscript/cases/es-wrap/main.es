import { Console } from "effect"

// A function that makes a call becomes an Effect. Calls do not run yet, so
// the Console.log below builds an Effect and drops it.
export function answer(): number {
  Console.log("computing the answer")
  return 42
}

export const half = (n: number): number => {
  Console.log(`halving ${n}`)
  return n / 2
}
