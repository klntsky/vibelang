import { expect, test } from "bun:test"
import { Effect, Exit, trusted } from "vibelang-runtime"

class PortError extends Error { readonly kind = "port" }
class HostError extends Error { readonly kind = "host" }

test("trusted: listed instances fail; other throws die", () => {
  const port = new PortError("port")
  const host = new HostError("host")
  for (const error of [port, host, new Error("unlisted"), "non-Error"]) {
    const call = trusted((): number => { throw error }, [PortError, HostError])
    expect(Effect.runSyncExit(call())).toEqual(
      error === port || error === host ? Exit.fail(error) : Exit.die(error)
    )
  }
})

test("trusted: calls are lazy, this-free, preserve parameters and return values", () => {
  let calls = 0
  const holder = { call: trusted(function (this: void, head: number, ...tail: string[]) {
    expect(this).toBeUndefined()
    calls++
    return { head, tail }
  }, []) }
  const detached = holder.call
  const program = detached(42, "a", "b")
  expect(calls).toBe(0)
  expect(Effect.runSync(program)).toEqual({ head: 42, tail: ["a", "b"] })
  expect(Effect.runSync(holder.call(7))).toEqual({ head: 7, tail: [] })
  expect(calls).toBe(2)
})
