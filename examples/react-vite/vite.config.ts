import react from "@vitejs/plugin-react"
import vibelang from "vibelang-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [vibelang(), react()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.vibe", "src/**/*.test.tsx"]
  }
})
