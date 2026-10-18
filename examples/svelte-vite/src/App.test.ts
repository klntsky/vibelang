import { render, screen } from "@testing-library/svelte"
import { expect, it } from "vitest"
import App from "./App.svelte"

it("shows loading, then the quote and the recovered error", async () => {
  render(App)
  expect(screen.getByText("Loading…")).toBeDefined()
  expect(await screen.findByText("2 x Green tea: $8.00")).toBeDefined()
  expect(await screen.findByText("Only 1 coffee left")).toBeDefined()
})
