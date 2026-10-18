import { defineConfig } from "vitest/config"
import vibelang from "vibelang-vite"

export default defineConfig({
  plugins: [vibelang()],
  test: { include: ["src/**/*.test.vibe"] }
})
