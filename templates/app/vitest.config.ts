import { defineConfig } from "vitest/config"
import vibelang from "vibelang-vite"

export default defineConfig({
  plugins: [vibelang({ project: "tsconfig.test.json" })],
  test: { include: ["src/**/*.test.vibe"] }
})
