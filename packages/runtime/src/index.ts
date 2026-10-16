import "./version.js"
// Runtime ABI used by generated EffectScript modules.
export * from "./calls.js"
export * from "./regions.js"
export * from "./fork.js"
export { $Context, $control, $reference, $service, Control, wire } from "./services.js"
export * from "./handlers.js"
export * from "./producers.js"
export * from "./boundaries.js"
export * from "./prelude.js"

// TypeScript adapters for asynchronous sources (D43).
export { suspend, handoff, HandoffProtocolViolation, type Resumer } from "./kernel/kernel.js"
