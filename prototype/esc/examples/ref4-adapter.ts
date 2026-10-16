// Reference program 4, TypeScript side: a callback API adapted with the kernel's `suspend`
// (Chapter 6 "Adapting a callback API"). Boundary code, so it lives in TypeScript and may use timers.
import { Effect } from "effect"
import { suspend } from "../../../experiments/kernel/kernel.ts"

export const log: string[] = []

// A callback-based socket that connects after `delayMs`, like net.connect. Destroying it cancels
// the pending connection.
export class FakeSocket {
  destroyed = false
  private listener: (() => void) | undefined
  private readonly timer: ReturnType<typeof setTimeout>

  constructor(readonly url: string, delayMs: number) {
    log.push(`dial ${url}`)
    this.timer = setTimeout(() => this.listener?.(), delayMs)
  }

  once(_event: "connect", listener: () => void): void {
    this.listener = listener
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    clearTimeout(this.timer)
    log.push(`destroy ${this.url}`)
  }
}

// The proposal's adapter, with { interruptible: true } on the acquisition (Fable review finding 2):
// without it, cancellation cannot reach the pending wait and the canceller never runs.
export const connect = (url: string, delayMs: number) =>
  Effect.acquireRelease(
    suspend<FakeSocket>((waiter) => {
      const socket = new FakeSocket(url, delayMs)
      socket.once("connect", () => {
        if (!waiter.resume(socket)) socket.destroy()   // nobody is waiting anymore: release it
      })
      return () => socket.destroy()                    // the waiter was cancelled first
    }),
    (socket) => Effect.sync(() => socket.destroy()),
    { interruptible: true }
  )
