// Probe 1 (not in the corpus): lambdas whose class differs between overload candidates, and
// lambdas whose expected type is a bare type parameter or absent. Each function isolates one
// construct; the comparison reports which ones the single pass and the prototype decide alike.
import { apply, attach, register, start } from "./helpers.ts"

abstract class Users {
  abstract find(id: string): Effect<{ readonly id: string }, never, never>
}

// Compiles to plain JavaScript (D10).
function double(n: number) {
  return n * 2
}

// No call in the body: every candidate's class is satisfiable by the plain form.
function plainBody() {
  return register((x) => x + 1)
}

// A plain call: overload 1 classifies the lambda plain, overload 2 an Effect function.
function plainCall() {
  return register((x) => Math.max(x, 0))
}

// A call to a D10 function: the same split.
function esCall() {
  return register((x) => double(x))
}

// A service call: only the Effect form type-checks; overload 1 fails, overload 2 wins.
function serviceCall() {
  return register((x) => Users.find(String(x)).id.length)
}

// The reverse overload order: the Effect callback comes first.
function effectFirst() {
  return attach((x) => x + 1)
}

// A program first, a thunk second.
function programOrThunk() {
  return start(() => Math.max(1, 2))
}

// A bare type parameter: the expected type is the lambda's own type.
function bare() {
  const f = apply((x: number) => Math.max(x, 0))
  return f(1)
}

function bareService() {
  const g = apply((id: string) => Users.find(id))
  return g("1")
}

// A returned lambda from an Effect function: no expected type.
function returned(n: number) {
  Effect.log("returned")
  return (x: number) => Math.max(x, n)
}
