// A control operation is an abstract class that extends Control (D2). It
// compiles like a service whose key also registers one kernel operation per
// abstract method, and performing it joins R until a handler answers it.
import { Effect } from "effect"
import { Control, handle, resume } from "@effectscript/runtime"

abstract class Approval extends Control {
  abstract approve(request: string): Effect.Effect<boolean, never, never>
}

export function checkout(cartId: string) {
  const ok = Approval.approve(`charge cart ${cartId}`)
  return ok ? `charged ${cartId}` : `declined ${cartId}`
}

const requests: string[] = []

function approved() {
  return handle(() => checkout("c42"), Approval, {
    approve: (request: string, k) => {
      requests.push(request)
      return resume(k, true)
    }
  })
}

function declined() {
  return handle(() => checkout("c7"), Approval, { approve: (_request: string, k) => resume(k, false) })
}

// expect-result: {"approved":"charged c42","declined":"declined c7","requests":["charge cart c42"]}
export function main() {
  return { approved: approved(), declined: declined(), requests }
}
