import { ManagedRuntime } from "effect"
import { InventoryLive } from "./cart.vibe"

// One runtime for the app: it builds InventoryLive once and runs every program.
export const runtime = ManagedRuntime.make(InventoryLive)
