# Pattern Matching

`Match` provides Effect's pattern matching for EffectScript values. Build cases for shapes, literals, predicates, or tags, then choose how the matcher handles unmatched input. Match callbacks are synchronous value computations; use ordinary `if` or `switch` when a branch performs Effect work.

Match constructors, case builders, finalizers, and returned matchers are trusted Effect library functions. Call them directly; exceptions from those calls are defects. An ordinary `catch` does not recover those exceptions.

## Matchers from types or values

```ts
import { Match } from "effect"

const describe = pipe(
  Match.type<number | string>(),
  Match.when(Match.number, (n) => `count ${n}`),
  Match.when(Match.string, (s) => `label ${s}`),
  Match.exhaustive
)

function describeInput(input: number | string) {
  return describe(input)
}
// Effect<string, never, never>

function describeJob(input: { readonly state: string; readonly attempts: number }) {
  return pipe(
    Match.value(input),
    Match.when({ state: "queued" }, (job) => `waiting: ${job.attempts}`),
    Match.orElse((job) => `state: ${job.state}`)
  )
}
```

`Match.type<T>()` builds a matcher without choosing an input yet; its finalizer produces a plain function. `Match.value(input)` chooses the input first, so the finalizer directly computes the matched value. Cases are tried in order and the first matching case wins.

## Shapes, predicates, and tags

```ts
import { Match } from "effect"

function classifyRetry(input: { readonly status: string; readonly attempts: number }) {
  return pipe(
    Match.value(input),
    Match.when({ status: "failed", attempts: (n: number) => n >= 3 }, () => "escalate"),
    Match.when({ status: "failed" }, () => "retry"),
    Match.orElse(() => "leave unchanged")
  )
}

type Job =
  | { readonly _tag: "Queued" }
  | { readonly _tag: "Running"; readonly worker: string }
  | { readonly _tag: "Done"; readonly output: string }

function jobLabel(job: Job) {
  return pipe(
    Match.value(job),
    Match.tag("Queued", () => "waiting"),
    Match.tag("Running", (job) => `worker ${job.worker}`),
    Match.tag("Done", (job) => job.output),
    Match.exhaustive
  )
}
```

An object pattern checks its named properties without requiring the whole object to be written out. Predicate patterns check values; built-in predicates include `Match.string`, `Match.number`, `Match.boolean`, and `Match.symbol`. `Match.tag` narrows a union by `_tag`, and accepts several tags before the handler when cases share a result.

Other patterns include `Match.nonEmptyString`, `Match.bigint`, `Match.date`, `Match.record`, `Match.null`, `Match.undefined`, `Match.defined`, and `Match.any`. `Match.is(...values)` selects listed literals and `Match.instanceOf(Class)` selects class instances. Keep custom predicates free of Effect operations and expected failures so they can run as JavaScript callbacks.

## Negating patterns

```ts
import { Match } from "effect"

function availability(status: "active" | "paused" | "closed") {
  return pipe(
    Match.value(status),
    Match.not("closed", () => "can reopen"),
    Match.orElse(() => "archived")
  )
}
```

`Match.not` selects inputs that do not match its pattern. The fallback receives values excluded by that negation and any earlier cases. Order still matters: a broad negated case can consume inputs that a later case was meant to handle.

## Fixing the return type

```ts
import { Match } from "effect"

function display(input: number | boolean) {
  return pipe(
    Match.value(input),
    Match.withReturnType<string>(),
    Match.when(Match.number, (n) => n.toFixed(1)),
    Match.when(Match.boolean, (b) => b ? "enabled" : "disabled"),
    Match.exhaustive
  )
}
```

Place `withReturnType<T>()` before all cases. Each handler must return `T`, which catches accidental mismatches such as returning a number from one branch of a text formatter. Without it, the result normally becomes the union of handler return types.

## Exhaustive handling

```ts
import { Match } from "effect"

type Phase = "draft" | "published"

function phaseLabel(phase: Phase) {
  return pipe(
    Match.value(phase),
    Match.when("draft", () => "Draft"),
    Match.when("published", () => "Published"),
    Match.exhaustive
  )
}
```

`exhaustive` requires the remaining input type to be `never`. Adding a member to `Phase` exposes a missing branch at the finalizer. A predicate that tests an arbitrary condition does not necessarily prove that all values of a type have been covered.

## Defaults, optional outputs, and result outputs

```ts
import { Match } from "effect"

type Command = "start" | "stop" | "help"

function withDefault(command: Command) {
  return pipe(
    Match.value(command),
    Match.when("start", () => "starting"),
    Match.orElse((unmatched) => `unhandled: ${unmatched}`)
  )
}

function optionalCommand(command: Command) {
  return pipe(
    Match.value(command),
    Match.when("start", () => "starting"),
    Match.option
  )
}

function commandResult(command: Command) {
  return pipe(
    Match.value(command),
    Match.when("start", () => "starting"),
    Match.result
  )
}
```

`orElse` computes a fallback from the unmatched input. `option` returns `Some` on a match and `None` otherwise. `result` returns a successful Result on a match, or a failed Result containing the unmatched input. That failed Result is a returned value, not a typed Effect failure.
