import { ManagedRuntime } from "effect"
import { UsersLive } from "./users.effeck"

// One runtime for the app: it builds UsersLive once and runs every program.
export const runtime = ManagedRuntime.make(UsersLive)
