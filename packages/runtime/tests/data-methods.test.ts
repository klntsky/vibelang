import { expect, test } from "bun:test"
import { Cause, Effect, Exit, Fiber, HashMap, Option } from "effect"
import { $method } from "../src/data-methods.js"

test("Option dispatch is self-first and keeps predicate results", () => {
  const some = Option.some(3)
  expect($method(some, "isSome", [], [["Option", Option.isSome]])).toBe(true)
  expect($method(Option.none(), "isSome", [], [["Option", Option.isSome]])).toBe(false)
  expect($method(some, "getOrElse", [() => 0], [["Option", Option.getOrElse]])).toBe(3)
  expect($method(some, "map", [(n: number) => n + 1], [["Effect", Effect.map], ["Exit", Exit.map], ["Option", Option.map]])).toEqual(Option.some(4))
})

test("only matching candidate modules count, even when an Effect is also an Exit", async () => {
  const value = Effect.succeed(2)
  expect("~effect/Exit" in value && "~effect/Effect" in value).toBe(true)
  let calls = 0
  const mapped = $method(value, "map", [(n: number) => { calls++; return n + 3 }], [["Effect", Effect.map]])
  expect(calls).toBe(0)
  expect(await Effect.runPromise(mapped)).toBe(5)
  expect(calls).toBe(1)
  expect($method(value, "map", [(n: number) => n + 3], [["Exit", Exit.map]])).toEqual(Exit.succeed(5))
  expect(() => $method(value, "map", [(n: number) => n + 3], [["Effect", Effect.map], ["Exit", Exit.map]])).toThrow(TypeError)
  expect(await Effect.runPromise($method(value, "match", [{onSuccess: (n: number) => n + 1, onFailure: () => 0}], [["Effect", Effect.match]]))).toBe(3)
  expect(await Effect.runPromise($method(value, "isSuccess", [], [["Effect", Effect.isSuccess]]))).toBe(true)
  expect($method(value, "isSuccess", [], [["Exit", Exit.isSuccess]])).toBe(true)
  expect(() => $method(value, "isSuccess", [], [["Effect", Effect.isSuccess], ["Exit", Exit.isSuccess]])).toThrow(TypeError)
})

test("Effect-only methods work on succeed, fail, failCause, die, void and succeedNone", async () => {
  expect(await Effect.runPromise($method(Effect.succeed(1), "flatMap", [(n: number) => Effect.succeed(n + 1)], [["Effect", Effect.flatMap]]))).toBe(2)
  let taps = 0
  const tapped = $method(Effect.succeed(1), "tap", [() => Effect.sync(() => { taps++ })], [["Effect", Effect.tap]])
  expect(taps).toBe(0)
  expect(await Effect.runPromise(tapped)).toBe(1)
  expect(taps).toBe(1)
  for(const failed of [Effect.fail("bad"), Effect.failCause(Cause.fail("bad"))]) {
    expect(await Effect.runPromise($method(failed, "catch", [() => Effect.succeed("recovered")], [["Effect", Effect.catch]]))).toBe("recovered")
    expect(Exit.isFailure(await Effect.runPromiseExit($method(failed, "retry", [{times: 0}], [["Effect", Effect.retry]])))).toBe(true)
  }
  expect(Exit.isFailure(await Effect.runPromiseExit($method(Effect.die("boom"), "timeout", [1000], [["Effect", Effect.timeout]])))).toBe(true)
  expect(await Effect.runPromise($method(Effect.void, "flatMap", [() => Effect.succeed(7)], [["Effect", Effect.flatMap]]))).toBe(7)
  expect(await Effect.runPromise($method(Effect.succeedNone, "flatMap", [() => Effect.succeed(8)], [["Effect", Effect.flatMap]]))).toBe(8)
  const error = new Cause.NoSuchElementError()
  expect(() => $method(error, "map", [(n: never) => n], [["Effect", Effect.map], ["Cause", Cause.map]])).toThrow(TypeError)
  expect(await Effect.runPromise($method(error, "catch", [() => Effect.succeed("cause recovered")], [["Effect", Effect.catch]]))).toBe("cause recovered")
})

test("Fiber join returns the module's runnable Effect", async () => {
  const fiber = Effect.runFork(Effect.succeed(7))
  expect(await Effect.runPromise($method(fiber, "join", [], [["Fiber", Fiber.join]]))).toBe(7)
  expect(Fiber.isFiber(fiber)).toBe(true)
})

test("own members and inherited user class methods win with their receiver", () => {
  class User { count = 4; isSome() { return this.count } }
  expect($method(new User(), "isSome", [], [["Option", Option.isSome]])).toBe(4)
  const value = Object.assign(Option.some(1), { isSome() { return this.value + 8 } })
  expect($method(value, "isSome", [], [["Option", Option.isSome]])).toBe(9)
  expect(() => $method({ isSome: 1 }, "isSome", [], [["Option", Option.isSome]])).toThrow(TypeError)
})

test("arrays, strings, plain objects and uncovered values use JavaScript", () => {
  expect($method([1, 2], "map", [(n: number) => n * 2], [["Option", Option.map]])).toEqual([2, 4])
  expect($method("abc", "slice", [1], [])).toBe("bc")
  const plain = { value: 3, map(n: number) { return this.value + n } }
  expect($method(plain, "map", [2], [["Option", Option.map]])).toBe(5)
  expect(() => $method({ _tag: "Some", value: 1 }, "isSome", [], [["Option", Option.isSome]])).toThrow(TypeError)
  expect(() => $method(null, "isSome", [], [["Option", Option.isSome]])).toThrow(TypeError)
})

test("dispatch uses only supplied candidates and never a different module", () => {
  expect(() => $method(Option.some(1), "isSome", [], [])).toThrow(TypeError)
  expect(() => $method(Option.some(1), "map", [(n: number) => n], [["Exit", Exit.map]])).toThrow(TypeError)
})

test("identities outside the candidate intersection do not cause ambiguity", () => {
  const ambiguous = Object.assign(Option.some(1), { "~effect/Chunk": "~effect/Chunk" })
  expect($method(ambiguous, "isSome", [], [["Option", Option.isSome]])).toBe(true)
  expect(() => $method(ambiguous, "isSome", [], [["Option", Option.isSome], ["Chunk", () => false]])).toThrow(TypeError)
})


test("a callable member wins, but a numeric HashMap size dispatches", () => {
  const map = HashMap.make(["a", 1])
  expect(typeof (map as any).size).toBe("number")
  expect($method(map, "size", [], [["HashMap", HashMap.size]])).toBe(1)
  const value = Object.assign(Option.some(1), { isSome: 0 })
  expect($method(value, "isSome", [], [["Option", Option.isSome]])).toBe(true)
  let reads = 0
  const own = { count: 4, get isSome() { reads++; return function(this: { count: number }) { return this.count } } }
  expect($method(own, "isSome", [], [["Option", Option.isSome]])).toBe(4)
  expect(reads).toBe(1)
})

test("the runtime barrel does not expose the generated-code dispatcher", async () => {
  expect("$method" in await import("../src/index.js")).toBe(false)
})
