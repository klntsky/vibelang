import { expect, test } from "bun:test"
import { cheatSheetPairs, count, program } from "./token-counts"

const fence = (info: string, code: string) => ["```" + info, code, "```"].join("\n")

test("pairs each cheat-sheet entry's Effect blocks with its VibeLang blocks", () => {
  const markdown = [
    "# Cheat sheet",
    "",
    fence("vibe es", "error Shared"),
    "",
    "## Declarations",
    "",
    "**Errors** (D126).",
    "",
    fence("ts effect", "class Missing extends Data.TaggedError(\"Missing\") {}"),
    "",
    fence("vibe es", "error Missing"),
    "",
    "**Control operations** (D128). Effect has none.",
    "",
    fence("vibe long", "abstract class Review extends Control {}"),
    "",
    fence("vibe es", "control Review {}"),
    "",
    "**Plain TypeScript in hatches** (D148).",
    "",
    "**Built-in exceptions (D101).** More prose.",
    "",
    fence("ts effect", "const id = Effect.sync(() => 1)"),
    "",
    fence("vibe es", "// run: Console.log(hatch())\nfunction hatch() { return sync(() => 1) }\n// Output: 1"),
    "",
    fence("vibe", "// hatch.test.vibe\nit.live(\"runs\") { }")
  ].join("\n")
  expect(cheatSheetPairs(markdown)).toEqual([
    { label: "Errors", effect: ["class Missing extends Data.TaggedError(\"Missing\") {}"], vibe: ["error Missing"] },
    {
      label: "Plain TypeScript in hatches",
      effect: ["const id = Effect.sync(() => 1)"],
      vibe: ["// run: Console.log(hatch())\nfunction hatch() { return sync(() => 1) }\n// Output: 1", "// hatch.test.vibe\nit.live(\"runs\") { }"]
    }
  ])
})

test("a pair counts the program, not the docs checker's lines", () => {
  const code = "// run: Console.log(f())\nfunction f() { return 1 }\n// Output: 1\n// Output:"
  expect(program(code)).toBe("function f() { return 1 }")
  expect(count(program(code))).toBe(count("function f() { return 1 }"))
})

test("counts exact text with the o200k_base encoding", () => {
  expect(count("")).toBe(0)
  expect(count("hello world")).toBe(2)
  expect(count("<|endoftext|>")).toBeGreaterThan(1)
})
