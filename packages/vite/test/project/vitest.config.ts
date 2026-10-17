import { defineConfig } from "vitest/config"
import effeck from "../../src/index.js"

export default defineConfig({
  plugins: [effeck({ compiler: "../../../compiler/bin/effeck" })],
  test: { include: [process.env.FAILING_TEST ? "failure.test.effeck" : "passing.test.effeck"] }
})
