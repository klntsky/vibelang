import { expect, test } from "bun:test"
import { mapperEntry } from "./mapper-options"

test("D34 tracing survives stripping fork compiler options", () => {
  for (const tracing of ["all", "exported"]) {
    const config = { compilerOptions: { effeck: { tracing } } }
    const entry = mapperEntry(config)
    delete (config.compilerOptions as any).effeck
    expect(entry).toEqual({ package: "effeck-ts-mapper", extensions: [".effeck"], options: { tracing } })
  }
})

test("a project without tracing keeps the ordinary mapper entry", () => {
  expect(mapperEntry({ compilerOptions: {} })).toEqual({ package: "effeck-ts-mapper", extensions: [".effeck"] })
})
