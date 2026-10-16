import { Cause, Effect, Exit } from "effect"

export const run = (effect: Effect.Effect<any, any, any>) => Effect.runPromiseExit(effect as any)
export const sync = (effect: Effect.Effect<any, any, any>) => Effect.runSyncExit(effect as any)
export const value = (exit: Exit.Exit<any, any>) => Exit.isSuccess(exit) ? exit.value : summary(exit)
export const summary = (exit: Exit.Exit<any, any>) => Exit.isSuccess(exit)
  ? { success: exit.value }
  : { failure: exit.cause.reasons.map(r => r._tag === "Die"
    ? { die: (r.defect as any)?._tag ?? String(r.defect) }
    : r._tag === "Fail" ? { fail: r.error } : { interrupt: r.fiberId }) }
export const defect = (exit: Exit.Exit<any, any>) => Exit.isFailure(exit)
  ? (exit.cause.reasons.find(Cause.isDieReason)?.defect as any)?._tag : undefined
export const note = (events: any[], entry: any) => Effect.sync(() => { events.push(entry) })
