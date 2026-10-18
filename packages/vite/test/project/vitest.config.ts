import { defineConfig } from "vitest/config"
import vibelang from "../../src/index.js"

export default defineConfig({
  plugins: [vibelang({ compiler: "../../../compiler/bin/vibelang" })],
  test: { include: [process.env.FAILING_TEST ? "failure.test.vibe" : "passing.test.vibe"] }
})
