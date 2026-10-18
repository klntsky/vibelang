import react from "@vitejs/plugin-react"
import effeck from "effeck-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [effeck(), react()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.effeck", "src/**/*.test.tsx"]
  }
})
