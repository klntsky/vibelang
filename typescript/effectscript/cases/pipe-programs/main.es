// pipe takes the program first (design D8): when its first argument is a
// function with no parameters and the first stage takes an Effect, that
// function is the program. On plain data, pipe is effect's ordinary pipe.
import { Array as Arr, Schedule } from "effect"

let attempts = 0

function flaky() {
  attempts++
  if (attempts < 3) throw new Error(`attempt ${attempts}`)
  return `ok after ${attempts}`
}

function piped() {
  return pipe(
    () => flaky(),
    Effect.retry({ times: 3 }),
    Effect.timeout("1 second"),
    Effect.withSpan("flaky")
  )
}

// A stored program works the same way.
function stored() {
  const job = () => flaky()
  return pipe(job, Effect.retry(Schedule.recurs(1)))
}

// Plain data flows through plain stages.
function data() {
  return pipe([1, 2, 3], Arr.map((n) => n * 2), Arr.reduce(0, (sum, n) => sum + n))
}

// expect-result: {"piped":"ok after 3","stored":"ok after 4","data":12}
export function main() {
  return { piped: piped(), stored: stored(), data: data() }
}
