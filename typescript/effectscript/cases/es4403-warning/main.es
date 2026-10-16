// A project that sets ES4403 to "warning" (D34) gets a warning, and the read
// compiles as a trusted sync call: no ES4402 for crypto, no boundary needed.
import { Effect } from "effect"

function stamp() {
  Effect.sleep("1 millis")
  return Date.now()
}

function uuid() {
  return crypto.randomUUID()
}

// expect-result: [true,36]
export function main() {
  return [stamp() > 0, uuid().length]
}
