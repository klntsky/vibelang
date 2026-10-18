import vue from "@vitejs/plugin-vue"
import effeck from "effeck-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [effeck(), vue()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.effeck", "src/**/*.test.ts"]
  }
})
