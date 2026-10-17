// A lambda takes the form the overload candidate under check declares
// (design D56): a plain function, an Effect function, or a program. The first
// candidate that accepts it in that form wins, as TypeScript's first-applicable
// rule says.
import { Effect } from "effect"
import { apply, attach, find, register, start } from "./helpers.ts"

// No call in the body: overload 1 accepts the plain form.
function plainBody() {
  return register((x) => x + 1)
}

// A plain call: still overload 1.
function plainCall() {
  return register((x) => Math.max(x, 0))
}

// The body runs an Effect, which a plain function cannot do, so overload 1
// rejects the lambda and overload 2 accepts it as an Effect function.
function effectCall() {
  return register((x) => find(String(x)).id.length)
}

// The Effect callback comes first, so it wins even for a body with no call.
function effectFirst() {
  return attach((x) => x + 1)
}

// A program comes first: a lambda with no parameters is a program there.
function programFirst() {
  return start(() => Math.max(1, 2))
}

// No expected type: the lambda is plain because its body runs nothing (D10).
function bare() {
  const f = apply((x: number) => Math.max(x, 0))
  return f(-3)
}

// No expected type and a body that runs an Effect: an Effect function.
function bareEffect() {
  const f = apply((id: string) => find(id).id)
  return f("z")
}

// A lambda a function returns has no expected type: plain, because its body
// runs nothing, even inside an Effect function.
function returned(n: number) {
  Effect.sleep("1 millis")
  return (x: number) => Math.max(x, n)
}

// Array.prototype.map declares a plain callback; Effect.forEach an Effect one.
function traversals(ids: ReadonlyArray<string>) {
  const trimmed = ids.map((id) => id.trim())
  const found = Effect.forEach(trimmed, (id) => find(id).id)
  return found.join("")
}

// expect-result: {"plainBody":"plain 2","plainCall":"plain 1","effectCall":"effect 1","effectFirst":"effect 2","programFirst":"program 2","bare":0,"bareEffect":"z","traversals":"ab","returned":5}
export function main() {
  return {
    plainBody: plainBody(),
    plainCall: plainCall(),
    effectCall: effectCall(),
    effectFirst: effectFirst(),
    programFirst: programFirst(),
    bare: bare(),
    bareEffect: bareEffect(),
    traversals: traversals([" a", "b "]),
    returned: returned(5)(3)
  }
}
