// A lambda with no parameters where Effect expects an Effect is a program
// (design D7): Effect's combinators get a program they can run again,
// race, time out, or provide, and nothing runs until they do.
import { Context, Effect, Layer, Schedule } from "effect"

class Greeter extends Context.Service<Greeter, { readonly greet: (name: string) => Effect.Effect<string> }>()("Greeter") {}

let attempts = 0

function flaky() {
  attempts++
  if (attempts < 3) Effect.fail("not yet")
  return `ok after ${attempts}`
}

function slow(label: string) {
  Effect.sleep("200 millis")
  return label
}

function fast(label: string) {
  Effect.sleep("1 millis")
  return label
}

function retried() {
  return Effect.retry(() => flaky(), Schedule.recurs(5))
}

function raced() {
  return Effect.race(() => slow("slow"), () => fast("fast"))
}

function timedOut() {
  return Effect.catch(() => Effect.timeout(() => slow("slow"), "5 millis"), (e) => e._tag)
}

// An array and a record of programs where Effect.all expects Effects.
function both() {
  const list = Effect.all([() => fast("a"), () => fast("b")])
  const record = Effect.all({ c: () => fast("c"), d: () => fast("d") })
  return `${list.join("")}${record.c}${record.d}`
}

// A lambda stored in a variable is an Effect function: calling it runs it,
// from scratch, each time.
function stored() {
  let runs = 0
  const job = () => {
    runs++
    return fast(`run ${runs}`)
  }
  const first = job()
  const second = job()
  return `${first}, ${second}`
}

// An Effect function nested in another, each decided by its own position.
function grid(xs: ReadonlyArray<number>, ys: ReadonlyArray<number>) {
  return Effect.forEach(xs, (x) => Effect.forEach(ys, (y) => fast(`${x}${y}`)).join(" ")).join(" | ")
}

// The object literal's property is expected to be a function returning an
// Effect, through Layer.succeed's NoInfer<S>: the lambda is an Effect function.
const GreeterLive = Layer.succeed(Greeter, {
  greet: (name) => `hello ${name}`
})

function greeted() {
  return Effect.flatMap(Greeter, (greeter) => greeter.greet("ada"))
}

// expect-result: {"retried":"ok after 3","raced":"fast","timedOut":"TimeoutError","both":"abcd","stored":"run 1, run 2","grid":"13 14 | 23 24","greeted":"hello ada"}
export function main() {
  return {
    retried: retried(),
    raced: raced(),
    timedOut: timedOut(),
    both: both(),
    stored: stored(),
    grid: grid([1, 2], [3, 4]),
    greeted: Effect.provide(() => greeted(), GreeterLive)
  }
}
