import { defineConfig } from "vitest/config"
import effeck from "effeck-vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
