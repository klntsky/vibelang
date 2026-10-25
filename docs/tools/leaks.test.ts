import { expect, test } from "bun:test"
import { leaks } from "./leaks"

test("private patterns come from VIBELANG_LEAK_PATTERNS and never appear in diagnostics", () => {
  const previous = process.env.VIBELANG_LEAK_PATTERNS
  try {
    process.env.VIBELANG_LEAK_PATTERNS = JSON.stringify(["build-only\\.example", "/private/workstation"])
    for (const value of ["build-only.example", "/private/workstation/project"]) {
      const problems = leaks(value)
      expect(problems).toHaveLength(1)
      expect(problems.join("\n")).not.toContain(value)
    }
    expect(leaks("ordinary public documentation")).toEqual([])
  } finally {
    if (previous === undefined) delete process.env.VIBELANG_LEAK_PATTERNS
    else process.env.VIBELANG_LEAK_PATTERNS = previous
  }
})

test("invalid private-pattern configuration fails closed without echoing its input", () => {
  const previous = process.env.VIBELANG_LEAK_PATTERNS
  try {
    for (const input of ["not-json", "{}", "[42]", '["["]', '[""]']) {
      process.env.VIBELANG_LEAK_PATTERNS = input
      expect(() => leaks("anything")).toThrow("VIBELANG_LEAK_PATTERNS")
    }
  } finally {
    if (previous === undefined) delete process.env.VIBELANG_LEAK_PATTERNS
    else process.env.VIBELANG_LEAK_PATTERNS = previous
  }
})

test("repository scanning distinguishes neutral examples from personal paths, including symbol IDs", () => {
  const personal = "/Users/" + "private-developer"
  expect(leaks("d" + personal + "/repo/file.ts#symbol", { repository: true }).length).toBeGreaterThan(0)
  expect(leaks("/home/" + "private-developer/repo", { repository: true }).length).toBeGreaterThan(0)
  for (const example of ["/Users/you/repo", "/home/projects/app", "https://example.org/home/article", "~/.ssh/key"]) {
    expect(leaks(example, { repository: true })).toEqual([])
  }
})
