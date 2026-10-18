// TypeScript functions with each trust level that D5 and D249 describe.
import { Data } from "effect"
import { trusted } from "effeck-runtime"

export class PortError extends Data.TaggedError("PortError")<{ readonly text: string }> {}

/**
 * The implementation of parsePort, a trusted contract (D249): .effeck code
 * calls parsePort bare, and PortError joins E. Any other exception is a defect.
 */
export function parsePortImpl(text: string): number {
  if (text === "boom") throw new RangeError("unexpected")
  const port = Number(text)
  if (!Number.isInteger(port)) throw new PortError({ text })
  return port
}

/** A plain TypeScript function without a contract: .effeck code calls it through a boundary. */
export function legacyParse(text: string): number {
  const port = Number(text)
  if (!Number.isInteger(port)) throw new Error(`not a port: ${text}`)
  return port
}

/** A library function, trusted as parseJson below. */
export const parseJsonImpl: (text: string) => unknown = JSON.parse

export class Ports {
  constructor(readonly base: number) {}

  /** A method, trusted through the receiver adapter offsetPort below. */
  offset(by: number): number {
    const port = this.base + by
    if (port > 65535) throw new PortError({ text: String(port) })
    return port
  }
}

// D249: executable contracts beside the original TypeScript implementations.
export const parsePort = trusted(parsePortImpl, [PortError])
export const parseJson = trusted(parseJsonImpl, [SyntaxError])
export const offsetPort = trusted((base: number, by: number) => new Ports(base).offset(by), [PortError])
