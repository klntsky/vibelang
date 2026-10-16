import { defineConfig } from "vitest/config"
import effectscript from "../../src/index.js"

export default defineConfig({
  plugins: [effectscript({ compiler: "../../../compiler/bin/esc" })],
  test: { include: [process.env.FAILING_TEST ? "failure.test.es" : "passing.test.es"] }
})
