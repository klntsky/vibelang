// Each contract of main.vibe, checked exactly: a wrong channel is TS2322 here.
import type { Effect, Fiber, Scope } from "effect"
import { Audit, Down, Pool, audited, background, both, forkedLease, lease, query, scopedFork } from "./main.js"

type Exact<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false

export const contracts: [
  Exact<ReturnType<typeof query>, Effect.Effect<number, Down, Pool>>,
  Exact<ReturnType<typeof lease>, Effect.Effect<number, Down, Pool | Scope.Scope>>,
  Exact<ReturnType<typeof both>, Effect.Effect<number, Down, Pool | Scope.Scope>>,
  Exact<ReturnType<typeof audited>, Effect.Effect<string, never, Audit>>,
  Exact<ReturnType<typeof forkedLease>, Effect.Effect<string, never, Pool | Scope.Scope>>,
  Exact<ReturnType<typeof background>, Effect.Effect<Fiber.Fiber<void, never>, never, Audit | Scope.Scope>>,
  Exact<ReturnType<typeof scopedFork>, Effect.Effect<number, Down, Audit | Pool | Scope.Scope>>
] = [true, true, true, true, true, true, true]
