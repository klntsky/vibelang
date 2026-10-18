import { svelte } from "@sveltejs/vite-plugin-svelte"
import { svelteTesting } from "@testing-library/svelte/vite"
import effeck from "effeck-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [effeck(), svelte(), svelteTesting()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.effeck", "src/**/*.test.ts"]
  }
})
