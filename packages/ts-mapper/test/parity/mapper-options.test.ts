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
