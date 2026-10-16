class Bad extends TaggedError("Bad")<{}> {}
abstract class Registry {
  abstract use(name: string): Effect<number, never, never>
  abstract pipe(name: string): Effect<number, never, never>
  abstract lookup(name: string): Effect<number, Bad, never>
}
export function definite(name: string) {
  let n: number
  try {
    n = Registry.lookup(name)
  } catch (e) {
    n = -1
  }
  return n + Registry.use(name) + Registry.pipe(name)
}
