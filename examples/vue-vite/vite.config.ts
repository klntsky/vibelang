import vue from "@vitejs/plugin-vue"
import vibelang from "vibelang-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [vibelang(), vue()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.vibe", "src/**/*.test.ts"]
  }
})
