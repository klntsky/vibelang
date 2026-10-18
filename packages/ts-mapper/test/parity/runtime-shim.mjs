// The runtime the parity harness runs the mapper's JavaScript against:
// effeck-runtime, plus the exports the executable rendering calls that the
// runtime does not have yet. runtime.ts installs this file as the package
// effeck-runtime in the runtime column's node_modules, beside the real
// runtime as effeck-runtime-real. It lives in the test only: when the
// runtime exports $isRun, $direct, and $fast, the real ones win (see the
// end of this file) and this shim adds nothing.
//
// The protocols, as the executable rendering (internal/effeckmapper,
// render_exec.go) writes their calls:
//
//   $isRun(v)    L3: v runs where a call stands. An Effect runs, except a
//                Config, a Context key, and an Error, which are data.
//   $direct      the property of an Effect wrapper that holds its plain
//                function (D10), read by a call from another module.
//   $fast(body)  the fast-call entry of a generator function: it runs the
//                body eagerly and returns its value, or an Effect the caller
//                yields when the body pauses or its value runs. An Exit the
//                body yields resolves in place, as effect's own
//                fnUntracedEager does. Past DEPTH nested eager runs it
//                defers the body to the fiber, so recursion keeps heap
//                frames (research/typedirected.md section 6.6).
import * as Real from "effeck-runtime-real"
import { Config, Context, Effect } from "effect"

export * from "effeck-runtime-real"

const isRunShim = (v) =>
  v !== null &&
  (typeof v === "object" || typeof v === "function") &&
  Effect.isEffect(v) &&
  !Config.isConfig(v) &&
  !Context.isKey(v) &&
  !(v instanceof globalThis.Error)

const directShim = Symbol.for("effeck-runtime/direct")

const DEPTH = 1000
let depth = 0
const EXIT = "~effect/Exit"

// The rest of a fast call's generator after it paused: the Effect it paused
// on, then the generator itself. The caller yields it with yield*, which
// delegates to this iterator, so the callee resumes inside the caller's own
// generator with no Effect of its own. It carries Effect's type id only so
// that $isRun tells the caller to yield it; it never leaves the call site.
class Resume {
  constructor(it, pending) {
    this.it = it
    this.pending = pending
    this.state = 0
  }
  next(v) {
    if (this.state === 0) {
      this.state = 1
      return { done: false, value: this.pending }
    }
    return this.it.next(v)
  }
  throw(e) {
    return this.it.throw(e)
  }
  return(v) {
    return this.it.return(v)
  }
  [Symbol.iterator]() {
    if (this.state !== 0) throw new Error("effeck: a fast call's continuation ran twice")
    return this
  }
}
Resume.prototype["~effect/Effect"] = true

const fastShim = (body) =>
  function () {
    if (depth >= DEPTH) {
      const self = this
      const args = arguments
      return Effect.suspend(() => Effect.gen(() => body.apply(self, args)))
    }
    depth++
    try {
      const it = body.apply(this, arguments)
      let value
      while (true) {
        const state = it.next(value)
        if (state.done) return isRun(state.value) ? Effect.succeed(state.value) : state.value
        const p = state.value
        if (p !== null && typeof p === "object" && EXIT in p && p._tag === "Success") {
          value = p.value
          continue
        }
        return new Resume(it, p)
      }
    } finally {
      depth--
    }
  }

export const $isRun = Real.$isRun ?? isRunShim
const isRun = $isRun
export const $direct = Real.$direct ?? directShim
export const $fast = Real.$fast ?? fastShim
