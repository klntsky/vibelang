// A parameter that accepts a function or an Effect (design D39): a lambda
// with parameters is a function; one without is a program when it can
// pause, and a function when it compiles to plain JavaScript.
import { route } from "./helpers.ts"

function health() {
  Effect.sleep("1 millis")
  return "healthy"
}

// expect-result: {"withParameter":"function: got req","pausing":"program: healthy","plain":"function: static"}
export function main() {
  return {
    withParameter: route((request) => `got ${request}`),
    pausing: route(() => health()),
    plain: route(() => "static")
  }
}
