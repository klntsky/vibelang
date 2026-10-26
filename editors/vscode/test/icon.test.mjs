import { readFileSync } from "node:fs"
import { test } from "node:test"
import { assertLanguageIcons } from "./icon-assertions.mjs"

const root = new URL("../", import.meta.url)
test("both language icons use the text-free VibeLang site mark", () => {
  const manifest = JSON.parse(readFileSync(new URL("package.json", root), "utf8"))
  assertLanguageIcons(manifest, (file) => readFileSync(new URL(file, root), "utf8"))
})
