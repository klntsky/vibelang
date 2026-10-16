import { four } from "./helper.ts"

export function main() {
  return pipe(four, (n: number) => n + 1)
}
