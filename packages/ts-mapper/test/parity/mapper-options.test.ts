import { expect, test } from "bun:test"
import { mapperEntry } from "./mapper-options"

test("D34 tracing survives stripping fork compiler options", () => {
  for (const tracing of ["all", "exported"]) {
    const config = { compilerOptions: { vibelang: { tracing } } }
    const entry = mapperEntry(config)
    delete (config.compilerOptions as any).vibelang
    expect(entry).toEqual({ package: "vibelang-ts-mapper", extensions: [".vibe"], options: { tracing } })
  }
})

test("a project without tracing keeps the ordinary mapper entry", () => {
  expect(mapperEntry({ compilerOptions: {} })).toEqual({ package: "vibelang-ts-mapper", extensions: [".vibe"] })
})

test("preserves legacy top-level tracing while translating stock compiler options", () => {
  expect(mapperEntry({ vibelang: { tracing: "all" } }).options).toEqual({ tracing: "all" })
})

test("recovers tracing from the matching content mapper, ignoring unrelated packages", () => {
  expect(mapperEntry({ contentMappers: [
    { package: "other", options: { tracing: "exported" } },
    { package: "vibelang-ts-mapper", options: { tracing: "all" } }
  ] }).options).toEqual({ tracing: "all" })
})

test("fork options win over legacy options and mapper defaults, including explicit none", () => {
  for (const tracing of ["none", "exported", "all"]) {
    const config = {
      compilerOptions: { vibelang: { tracing } },
      vibelang: { tracing: "exported" },
      contentMappers: [{ package: "vibelang-ts-mapper", options: { tracing: "all" } }]
    }
    const before = JSON.stringify(config)
    expect(mapperEntry(config).options).toEqual({ tracing })
    expect(JSON.stringify(config)).toBe(before)
  }
})

test("legacy tracing wins over the content mapper fallback", () => {
  expect(mapperEntry({ vibelang: { tracing: "none" }, contentMappers: [
    { package: "vibelang-ts-mapper", options: { tracing: "all" } }
  ] }).options).toEqual({ tracing: "none" })
})

test("missing optional configuration never invents a tracing option", () => {
  for (const config of [{}, { compilerOptions: {} }, { compilerOptions: { vibelang: {} } },
    { contentMappers: [] }, { contentMappers: [{ package: "other", options: { tracing: "all" } }] },
    { contentMappers: [{ package: "vibelang-ts-mapper" }] }]) {
    expect(mapperEntry(config)).toEqual({ package: "vibelang-ts-mapper", extensions: [".vibe"] })
  }
})
