// A callback parameter type without a contract accepts any contract
// (design D12): mapAll's E and R come from each call's callback.
class NotFound extends TaggedError("NotFound")<{ readonly id: string }> {}

function find(id: string) {
  if (id === "") throw new NotFound({ id })
  Effect.sleep("1 millis")
  return `user-${id}`
}

export function mapAll<A, B>(items: ReadonlyArray<A>, f: (a: A) => B) {
  const out: B[] = []
  for (const item of items) out.push(f(item))
  return out
}

// A contract on the callback restricts it: this callback cannot fail.
export function mapSafe<A, B>(items: ReadonlyArray<A>, f: (a: A) => Effect<B, never, never>) {
  return items.length === 0 ? [] : [f(items[0])]
}

export function withRetry<A>(times: number, program: () => A) {
  return Effect.retry(program, { times })
}

let attempts = 0

function flaky() {
  attempts++
  if (attempts < 3) throw new NotFound({ id: `attempt ${attempts}` })
  return attempts
}

// expect-result: {"names":["user-1","user-2"],"sizes":[1,3],"safe":[2],"retried":3,"caught":"NotFound"}
export function main() {
  const names = mapAll(["1", "2"], (id) => find(id))
  const sizes = mapAll(["a", "bcd"], (w) => w.length)
  const safe = mapSafe([1], (n) => n * 2)
  const retried = withRetry(5, () => flaky())
  let caught = "none"
  try {
    mapAll([""], (id) => find(id))
  } catch (e) {
    caught = e._tag
  }
  return { names, sizes, safe, retried, caught }
}
