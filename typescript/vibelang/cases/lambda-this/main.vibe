// A lowered arrow function keeps its enclosing function's this, as an arrow
// function does in JavaScript: a program, an Effect function, and a forked
// program in a method all reach the instance.
import { Fiber } from "effect"

class Counter {
  count = 0
  label = "counter"

  bump() {
    Effect.sleep("1 millis")
    this.count++
    return this.count
  }

  // A program, passed to Effect.retry.
  retried() {
    return Effect.retry(() => this.bump(), { times: 1 })
  }

  // An Effect function, passed to Effect.forEach.
  each(names: string[]) {
    return Effect.forEach(names, (name) => `${this.label}:${name}:${this.bump()}`)
  }

  // A forked program.
  forked() {
    const fiber = fork(() => this.bump())
    return Fiber.join(fiber)
  }
}

// expect-result: {"retried":1,"each":["counter:a:2","counter:b:3"],"forked":4}
export function main() {
  const counter = new Counter()
  const retried = counter.retried()
  const each = counter.each(["a", "b"])
  const forked = counter.forked()
  return { retried, each, forked }
}
