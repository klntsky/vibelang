import { ManagedRuntime } from "effect"
import { TodosLive } from "./todos.vibe"

// One runtime for the app: it builds TodosLive once and runs every program.
export const runtime = ManagedRuntime.make(TodosLive)
