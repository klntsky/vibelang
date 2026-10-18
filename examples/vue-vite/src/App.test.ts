import { mount } from "@vue/test-utils"
import { expect, it, vi } from "vitest"
import App from "./App.vue"

it("shows loading, then the board and the recovered error", async () => {
  const wrapper = mount(App)
  expect(wrapper.text()).toContain("Loading…")
  await vi.waitFor(() => expect(wrapper.text()).toContain("No todo #99"))
  expect(wrapper.findAll("li").map((item) => item.text())).toEqual([
    "[x] Declare the schema",
    "[x] Write the service",
    "[ ] Render the board"
  ])
  expect(wrapper.text()).toContain("Completed: Write the service")
})
