// A catch handles a failure only when the cause holds expected failures alone
// (D13): a defect, an interruption, or a mixed cause passes through it. A
// finally runs on every exit, and a failure inside it is never dropped (D15).
import { Data, Effect, Exit } from "effect"

class Expected extends Data.TaggedError("Expected")<{}> {}
class Cleanup extends Data.TaggedError("Cleanup")<{}> {}

const events: string[] = []

function reasons(exit: Exit.Exit<unknown, unknown>) {
  return Exit.isFailure(exit) ? exit.cause.reasons.map((reason) => reason._tag) : ["Success"]
}

// A JavaScript TypeError is a defect: the catch does not run, the finally does.
function defect() {
  try {
    Effect.sleep("1 millis")
    const broken: any = undefined
    return broken.name
  } catch (e) {
    events.push("catch saw a defect")
    return "caught"
  } finally {
    events.push("finally after a defect")
  }
}

// An expected failure plus a defect from cleanup is a mixed cause.
function mixed() {
  try {
    try {
      throw new Expected()
    } finally {
      Effect.die("release-defect")
    }
  } catch (e) {
    events.push("catch saw a mixed cause")
    return "recovered"
  }
}

// A failure inside finally is kept; when the body failed too, both are.
function failingFinally(bodyFails: boolean) {
  try {
    if (bodyFails) throw new Expected()
    return "body"
  } finally {
    throw new Cleanup()
  }
}

// Interruption runs finally and skips catch.
function slow() {
  try {
    Effect.sleep("1 second")
    return "finished"
  } catch (e) {
    events.push("catch saw an interruption")
    return "caught"
  } finally {
    events.push("finally after interruption")
  }
}

// expect-result: {"defect":["Die"],"mixed":["Fail","Die"],"cleanup":["Fail"],"both":["Fail","Fail"],"timeout":["Fail"],"events":["finally after a defect","finally after interruption"]}
export function main() {
  return {
    defect: reasons(Effect.exit(() => defect())),
    mixed: reasons(Effect.exit(() => mixed())),
    cleanup: reasons(Effect.exit(() => failingFinally(false))),
    both: reasons(Effect.exit(() => failingFinally(true))),
    timeout: reasons(Effect.exit(() => Effect.timeout(() => slow(), "10 millis"))),
    events
  }
}
