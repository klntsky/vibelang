// E and R of functions that call each other are a fixpoint over their call
// cycle (D52). Each function here learns the other's failure only through the
// cycle, whichever the checker types first.
export class Even extends TaggedError("Even")<{ readonly n: number }> {}
export class Odd extends TaggedError("Odd")<{ readonly n: number }> {}

export function isOdd(n: number): Effect<boolean, infer E, infer R> {
  if (n === 7) throw new Odd({ n })
  if (n === 0) return false
  return isEven(n - 1)
}

export function isEven(n: number): Effect<boolean, infer E, infer R> {
  if (n === 100) throw new Even({ n })
  if (n === 0) return true
  return isOdd(n - 1)
}

// A caller outside the cycle reads the solved contract.
export function parity(n: number) {
  try {
    return isEven(n) ? "even" : "odd"
  } catch (e) {
    return e._tag
  }
}

// Self-recursion whose success type does not depend on itself needs no
// contract: total is declared as a number.
interface Entry {
  readonly size: number
  readonly children: ReadonlyArray<string>
}

export class Missing extends TaggedError("Missing")<{ readonly path: string }> {}

const tree = new Map<string, Entry>([
  ["/", { size: 0, children: ["a", "b"] }],
  ["/a", { size: 10, children: [] }],
  ["/b", { size: 0, children: ["c"] }],
  ["/b/c", { size: 32, children: [] }]
])

function stat(path: string) {
  const entry = tree.get(path)
  if (entry === undefined) throw new Missing({ path })
  return entry
}

export function totalSize(path: string) {
  const entry = stat(path)
  let total = entry.size
  for (const child of entry.children) total += totalSize(path === "/" ? `/${child}` : `${path}/${child}`)
  return total
}

// expect-result: {"parity":["even","odd","Odd","Even"],"size":42}
export function main() {
  return { parity: [parity(4), parity(3), parity(8), parity(100)], size: totalSize("/") }
}
