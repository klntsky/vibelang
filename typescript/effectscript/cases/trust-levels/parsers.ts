// TypeScript functions with each trust level that D5 and D23 describe.
import { Data } from "effect"

export class PortError extends Data.TaggedError("PortError")<{ readonly text: string }> {}

/**
 * A trusted contract (D23): .es code calls it bare, and PortError joins E.
 * Any other exception is a defect.
 *
 * @effectscript trusted
 * @throws {PortError} when the text is not an integer
 */
export function parsePort(text: string): number {
  if (text === "boom") throw new RangeError("unexpected")
  const port = Number(text)
  if (!Number.isInteger(port)) throw new PortError({ text })
  return port
}

/** A plain TypeScript function without a contract: .es code calls it through a boundary. */
export function legacyParse(text: string): number {
  const port = Number(text)
  if (!Number.isInteger(port)) throw new Error(`not a port: ${text}`)
  return port
}

/**
 * A library function trusted by re-exporting it with the tag (D23). Its
 * SyntaxError is a global class, so the generated code imports nothing.
 *
 * @effectscript trusted
 * @throws {SyntaxError} when the text is not JSON
 */
export const parseJson: (text: string) => unknown = JSON.parse

/** @effectscript trusted */
export class Ports {
  constructor(readonly base: number) {}

  /**
   * A trusted method: its contract is on the method.
   *
   * @effectscript trusted
   * @throws {PortError} when the result leaves the port range
   */
  offset(by: number): number {
    const port = this.base + by
    if (port > 65535) throw new PortError({ text: String(port) })
    return port
  }
}
