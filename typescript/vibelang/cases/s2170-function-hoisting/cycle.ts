import named from "./named.js"

// Standard JavaScript calls the hoisted export while named.vibe is in its
// dependency cycle. The returned Effect remains cold until main runs it.
const early = named(2)
export const getEarly = () => early
