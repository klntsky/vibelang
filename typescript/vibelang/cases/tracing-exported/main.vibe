// "tracing": "exported" compiles each exported function with an Effect.fn
// span named after it (D34). inner is not exported, so it runs in the span
// of the exported function that calls it.
function inner() {
  return Effect.currentSpan().name
}

export function outer() {
  return inner()
}

// expect-result: ["outer","main"]
export function main() {
  const fromOuter = outer()
  return [fromOuter, Effect.currentSpan().name]
}
