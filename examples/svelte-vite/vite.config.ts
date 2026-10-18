import { svelte } from "@sveltejs/vite-plugin-svelte"
import { svelteTesting } from "@testing-library/svelte/vite"
import vibelang from "vibelang-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [vibelang(), svelte(), svelteTesting()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.vibe", "src/**/*.test.ts"]
  }
})
