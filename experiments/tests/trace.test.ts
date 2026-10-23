// T1: suspension and resumption through nested calls, with Context and Scope.
//
// EffectScript source this test hand-lowers (operation spelling illustrative):
//
//   function checkout(cartId: string) {
//     scope {
//       using hold = Inventory.reserve(cartId)
//       const ok = requestApproval(cartId)
//       if (!ok) throw Declined { cartId }
//       return `charged ${cartId} holding ${hold}`
//     }
//   }
//
//   function requestApproval(cartId: string) {
//     Logger.info(`approval needed for ${cartId}`)
//     const ok = Approve(`checkout ${cartId}`)          // captures up to the handler
//     Logger.info(`approval ${ok ? "granted" : "denied"}`)
//     return ok
//   }
//
//   handle {
//     provide(Logger, RequestLogger) { checkout("c-42") }
//   } with {
//     Approve(request, k) {
//       Logger.info(`clause handling "${request}"`)      // AppLogger: outside the provide
//       const decision = Reviewers.decide(request)       // whole fiber suspends (handoff)
//       return resume(k, decision)
//     }
//   }
import { expect, test } from "bun:test"
import { Context, Deferred, Effect, Exit, Scope } from "effect"
import { handle, hooks, operation, resume } from "../../packages/runtime/src/kernel/kernel.ts"
import { compiled, contextLabels, stackLabels } from "../../packages/runtime/src/kernel/trace.ts"

class Logger extends Context.Service<Logger, {
  readonly name: string
  readonly info: (message: string) => Effect.Effect<void>
}>()("app/Logger") {}

class Inventory extends Context.Service<Inventory, {
  readonly reserve: (cartId: string) => Effect.Effect<string, never, Scope.Scope>
}>()("app/Inventory") {}

class Reviewers extends Context.Service<Reviewers, {
  readonly decide: (request: string) => Effect.Effect<boolean>
}>()("app/Reviewers") {}

const Approve = operation<string, boolean>("Approve")

const events: Array<string> = []
const note = (line: string) => Effect.sync(() => void events.push(line))

const makeLogger = (name: string) => Logger.of({ name, info: (message) => note(`[${name}] ${message}`) })

const requestApproval = compiled("requestApproval", function*(cartId: string) {
  const logger = yield* Logger
  yield* logger.info(`approval needed for ${cartId}`)
  const ok: boolean = yield* Approve(`checkout ${cartId}`)
  const after = yield* Logger
  yield* after.info(`approval ${ok ? "granted" : "denied"}`)
  return ok
})

const checkoutScope = compiled("checkout/scope", function*(cartId: string) {
  const inventory = yield* Inventory
  const hold = yield* inventory.reserve(cartId)
  const ok = yield* requestApproval(cartId)
  if (!ok) return yield* Effect.fail({ _tag: "Declined" as const, cartId })
  return `charged ${cartId} holding ${hold}`
})

const checkout = compiled("checkout", function*(cartId: string) {
  return yield* Effect.scoped(checkoutScope(cartId))
})

const describeContext = (context: Context.Context<never>) =>
  contextLabels(context, (key, value) => {
    if (key === "app/Logger") return `Logger=${value.name}`
    if (key === "effect/Scope") return `Scope(${value.state._tag})`
    if (key.startsWith("effectscript/operation/")) return `${key.slice(23)}->${value._tag}`
    return undefined
  }).join(", ")

test("T1 nested suspension and resumption preserve Context and Scope", async () => {
  const reviewStarted = Deferred.makeUnsafe<void>()
  const tickerRan = Deferred.makeUnsafe<void>()
  const machine: Array<string> = []
  hooks.onCapture = (fiber, k) => {
    machine.push(`capture ${k.op} on fiber ${fiber.id}: ${k.frameCount} frames`)
    machine.push(`  segment (bottom->top): ${stackLabels(k.segment!).join(" | ")}`)
    machine.push(`  saved context: ${describeContext(k.context)}`)
  }
  hooks.onResume = (fiber, k) => {
    machine.push(`resume ${k.op} on fiber ${fiber.id}`)
    machine.push(`  reinstated top: ${stackLabels(fiber._stack.slice(-k.handler.label.length - 6)).join(" | ")}`)
    machine.push(`  restored context: ${describeContext(fiber.context)}`)
  }

  const program = handle(
    "approvals",
    [Approve],
    checkout("c-42").pipe(Effect.provideService(Logger, makeLogger("RequestLogger"))),
    {
      Approve: (request: string, k) =>
        Effect.gen(function*() {
          const logger = yield* Logger
          yield* logger.info(`clause handling "${request}"`)
          const reviewers = yield* Reviewers
          const decision = yield* reviewers.decide(request)
          yield* logger.info(`clause resuming with ${decision}`)
          return yield* resume(k, decision)
        })
    }
  )

  const ticker = Effect.gen(function*() {
    yield* Deferred.await(reviewStarted)
    yield* note("other fiber ran while the main fiber was suspended")
    yield* Deferred.succeed(tickerRan, undefined)
  })

  const main = Effect.gen(function*() {
    yield* Effect.forkChild(ticker)
    return yield* program
  }).pipe(
    Effect.provideService(Logger, makeLogger("AppLogger")),
    Effect.provideService(Inventory, {
      reserve: (cartId: string) =>
        Effect.acquireRelease(
          Effect.as(note(`reserve ${cartId}`), "hold-1"),
          () => note("release hold-1")
        )
    }),
    Effect.provideService(Reviewers, {
      decide: (request: string) => Effect.gen(function*() {
        // Timer deadlines do not order fibers: a delayed child can otherwise be
        // interrupted when main completes before it records its event.
        yield* Deferred.succeed(reviewStarted, undefined)
        yield* Deferred.await(tickerRan)
        yield* note(`reviewer approved "${request}"`)
        return true
      })
    })
  )

  const exit = await Effect.runPromiseExit(main)
  hooks.onCapture = undefined
  hooks.onResume = undefined

  console.log(["", "--- T1 machine trace", ...machine, "--- T1 events", ...events].join("\n"))

  expect(Exit.isSuccess(exit) && exit.value).toBe("charged c-42 holding hold-1")
  expect(events).toEqual([
    "reserve c-42",
    "[RequestLogger] approval needed for c-42",
    "[AppLogger] clause handling \"checkout c-42\"",
    "other fiber ran while the main fiber was suspended",
    "reviewer approved \"checkout c-42\"",
    "[AppLogger] clause resuming with true",
    "[RequestLogger] approval granted",
    "release hold-1"
  ])
})
