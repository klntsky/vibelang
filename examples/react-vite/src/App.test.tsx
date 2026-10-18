import { render, screen } from "@testing-library/react"
import { expect, it } from "vitest"
import { App } from "./App.tsx"

it("shows loading, then the user and the recovered error", async () => {
  render(<App />)
  expect(screen.getAllByText("Loading…")).toHaveLength(2)
  expect(await screen.findByText("Welcome, Ada Lovelace")).toBeDefined()
  expect(await screen.findByText("No user 404")).toBeDefined()
})
