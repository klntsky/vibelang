// Where Effect.all expects an iterable or a record of Effects, an iterable or
// a record of programs converts element by element (design D49): a tuple
// literal, a record literal, a Set, and a Map's values.
function prepare(n: number) {
  Effect.sleep("1 millis")
  return n * 2
}

function collected() {
  const tuple = Effect.all([() => 12, () => "ready"] as const)
  const object = Effect.all({ count: () => 12, status: () => "ready" })
  const iterable = new Set([() => prepare(3), () => prepare(5)])
  const set = Effect.all(iterable, { concurrency: 2 })
  const jobs = new Map([["a", () => prepare(1)], ["b", () => prepare(2)]])
  const values = Effect.all(jobs.values())
  return { tuple, object, set, values }
}

// expect-result: {"tuple":[12,"ready"],"object":{"count":12,"status":"ready"},"set":[6,10],"values":[2,4]}
export function main() {
  return collected()
}
