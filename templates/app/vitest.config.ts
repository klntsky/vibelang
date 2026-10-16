import { defineConfig } from "vitest/config"
import effectscript from "@effectscript/vite"

export default defineConfig({
  plugins: [effectscript()],
  test: { include: ["src/**/*.test.es"] }
})
