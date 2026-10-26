// Q2: runSync uses MixedScheduler("sync") and flushes only the root fiber's dispatcher.
import { Effect, Exit, Fiber } from "effect"
const show = (e: Exit.Exit<unknown, unknown>) => Exit.isSuccess(e) ? `Success(${String(e.value)})` : `Failure(${e.cause.reasons.map((r: any) => `${r._tag}:${String(r.defect ?? r.error).split("\n")[0]}`).join(",")})`
console.log("root yieldNow:                ", show(Effect.runSyncExit(Effect.as(Effect.yieldNow, "ok"))))
console.log("fork+join (child no yield):   ", show(Effect.runSyncExit(Effect.flatMap(Effect.forkChild(Effect.succeed(1)), Fiber.join))))
console.log("fork+join (child yields):     ", show(Effect.runSyncExit(Effect.flatMap(Effect.forkChild(Effect.as(Effect.yieldNow, 2)), Fiber.join))))
console.log("sleep(1):                     ", show(Effect.runSyncExit(Effect.as(Effect.sleep(1), 3))))
console.log("all concurrent w/ yields:     ", show(Effect.runSyncExit(Effect.all([Effect.as(Effect.yieldNow, 1), Effect.as(Effect.yieldNow, 2)], { concurrency: 2 }))))
