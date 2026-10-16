// expect: ES4201
// Reference program 6: a control operation rejected at a fork. The forked child cannot reach the Ask
// handler installed on this fiber.
import { Fiber } from "effect"

abstract class Ask extends Control {
  abstract ask(question: string): Effect<number, never, never>
}

function forked() {
  return handle(() => {
    const child = fork(() => Ask.ask("in a child fiber"))
    return Fiber.join(child)
  }, Ask, {
    ask(question, k) { return resume(k, question.length) }
  })
}
