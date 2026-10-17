import { expect, test } from "bun:test"
import { $extract, $is, $noMatch, $patternError, $rest, customMatcher, TaggedError } from "effeck-runtime"

class NotFound extends TaggedError("NotFound")<{ readonly id: string }> {}
class Timeout extends TaggedError("Timeout")<{}> {}

test("match: $is tests classes, predicates, primitives, custom matchers, and values", () => {
  expect($is(NotFound, new NotFound({ id: "1" }))).toBe(true)
  expect($is(NotFound, new Timeout())).toBe(false)
  expect($is(Date, new Date())).toBe(true)
  expect($is(String, "a")).toBe(true)
  expect($is(Number, "a")).toBe(false)
  expect($is((x: unknown) => x === 3, 3)).toBe(true)
  const even = { [customMatcher]: (x: unknown) => typeof x === "number" && x % 2 === 0 }
  expect($is(even, 4)).toBe(true)
  expect($is(even, 5)).toBe(false)
  expect($is(NaN, NaN)).toBe(true)
  expect($is(0, -0)).toBe(true)
  expect($is("a", "a")).toBe(true)
})

test("match: tagged errors carry the proposal's custom matcher", () => {
  const error = new NotFound({ id: "7" })
  expect((NotFound as any)[customMatcher](error, { matchType: "boolean" })).toBe(true)
  expect((NotFound as any)[customMatcher](new Timeout(), { matchType: "boolean" })).toBe(false)
  expect($extract(NotFound, error)).toEqual([error])
  expect($extract(NotFound, new Timeout())).toBe(false)
})

test("match: $extract follows a custom matcher's result", () => {
  const pair = { [customMatcher]: (x: unknown) => (Array.isArray(x) && x.length === 2 ? x : false) }
  expect($extract(pair, [1, 2])).toEqual([1, 2])
  expect($extract(pair, [1])).toBe(false)
  const always = { [customMatcher]: () => true }
  expect($extract(always, 1)).toEqual([])
  const set = { [customMatcher]: () => new Set([1, 2]) }
  expect($extract(set, 1)).toEqual([1, 2])
  expect($extract(String, "s")).toEqual(["s"])
  expect(() => $extract(3, 1)).toThrow(TypeError)
})

test("match: $rest keeps the properties the clauses do not name; $noMatch and $patternError throw", () => {
  expect($rest({ a: 1, b: 2, c: 3 }, ["a"])).toEqual({ b: 2, c: 3 })
  expect(() => $noMatch({ _tag: "Timeout" })).toThrow(TypeError)
  expect(() => $patternError(4705)).toThrow(TypeError)
})
