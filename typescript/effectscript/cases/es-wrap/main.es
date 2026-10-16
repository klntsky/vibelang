import { Console } from "effect"

// Each function that runs an Effect becomes one. Calling Console.log runs it.
export function answer(): number {
  Console.log("computing the answer")
  return 42
}

export const half = (n: number): number => {
  Console.log(`halving ${n}`)
  return n / 2
}
