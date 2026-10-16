// expect: ES4403
// D5: clock and randomness reads go through Clock, DateTime, and Random so
// tests control them. ES4403 is an error by default (D34).
export function stamp() {
  return Date.now()
}

export function today() {
  return new Date()
}

export function dice() {
  return Math.floor(Math.random() * 6)
}

export function id() {
  return crypto.randomUUID()
}

export function tick() {
  return performance.now()
}

export function legacy() {
  return Date()
}

export const startedAt = Date.now()

// Deterministic Date parsing and other built-ins are allowed.
export function parsed(text: string) {
  return new Date(text).getTime() + Date.parse(text) + Date.UTC(2026, 9, 16) + Math.floor(1.5)
}
