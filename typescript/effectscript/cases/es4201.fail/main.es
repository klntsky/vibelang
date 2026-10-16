// expect: ES4201
// A control operation inside a combinator that runs it on another fiber
// cannot reach the handler on this fiber (chapter 9). The calls in allowed()
// stay on this fiber or handle the operation inside the branch.
import { Data, Effect } from "effect"
import { Control, handle, resume } from "@effectscript/runtime"

class Busy extends Data.TaggedError("Busy")<{}> {}

abstract class Approval extends Control {
  abstract approve(request: string): Effect.Effect<boolean, Busy, never>
}

export function concurrent(ids: ReadonlyArray<string>) {
  return Effect.forEach(ids, (id) => Approval.approve(id), { concurrency: 4 })
}

export function raced() {
  return Effect.race(() => Approval.approve("a"), () => Approval.approve("b"))
}

export function timed() {
  return Effect.timeout(() => Approval.approve("slow"), "1 second")
}

export function allowed(ids: ReadonlyArray<string>) {
  const sequential = Effect.forEach(ids, (id) => Approval.approve(id))
  const one = Effect.forEach(ids, (id) => Approval.approve(id), { concurrency: 1 })
  const inside = Effect.forEach(ids, (id) =>
    handle(() => Approval.approve(id), Approval, {
      approve: (_request, k) => resume(k, true)
    }),
    { concurrency: 4 })
  return [sequential, one, inside]
}
