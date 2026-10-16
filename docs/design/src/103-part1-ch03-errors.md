### 3. Errors: the E channel

#### Throwing a typed error

Errors are the same tagged error classes you use today. `TaggedError` is Effect's `Data.TaggedError`, in scope in every `.es` file; errors that must be serialized use `Schema.TaggedError`. `throw` adds the error to the function's E.

```ts effect
class InvalidId extends Data.TaggedError("InvalidId")<{ readonly raw: string }> {}

const parseId = Effect.fn("parseId")(function* (raw: string) {
  if (!/^\d+$/.test(raw)) return yield* new InvalidId({ raw })
  return Number(raw)
})
```

```ts es
class InvalidId extends TaggedError("InvalidId")<{ readonly raw: string }> {}

function parseId(raw: string) {
  if (!/^\d+$/.test(raw)) throw new InvalidId({ raw })
  return Number(raw)
}
// Effect<number, InvalidId, never>
```

Errors from callees flow upward the same way. A function's E is everything it throws plus everything its callees can fail with, minus what it catches.

Any value can be thrown, and it joins E with its static type: `throw new Error("bad")` adds `Error`, and `throw "bad"` adds `string`, as `Effect.fail` would. Tagged error classes are the convention because `catch` can tell them apart with `instanceof` or `_tag`.

#### Catching

`try`/`catch` handles typed errors. The caught value is typed as exactly the errors the `try` block can produce.

```ts effect
const lookup = Effect.fn("lookup")(function* (raw: string) {
  const id = yield* parseId(raw)
  const users = yield* Users
  return yield* users.find(String(id)).pipe(
    Effect.catchTag("UserNotFound", () => Effect.succeed(guest))
  )
})
```

```ts es
function lookup(raw: string) {
  const id = parseId(raw)
  try {
    return Users.find(String(id))
  } catch (e) {                // e: UserNotFound
    return guest
  }
}
// Effect<User, InvalidId, Users>
```

To handle some errors and pass the rest on, narrow and rethrow. TypeScript's narrowing works on the caught value:

```ts es
function lookupOrGuest(raw: string) {
  try {
    return Users.find(String(parseId(raw)))
  } catch (e) {                          // e: InvalidId | UserNotFound
    if (e instanceof UserNotFound) return guest
    throw e                              // e: InvalidId here, so E is InvalidId
  }
}
// Effect<User, InvalidId, Users>
```

Effect's combinators still work when an expression reads better than a block. Pass the program as a function:

```ts es
const user = Effect.catchTag(() => Users.find(id), "UserNotFound", () => guest)
```

#### Promising that nothing escapes

A contract with `never` in E says every error is handled inside. The compiler checks it.

```ts es
export function safeLookup(raw: string): Effect<User, never, Users> {
  try {
    return lookupOrGuest(raw)
  } catch (e) {                // e: InvalidId
    return guest
  }
}
```

Remove the `try` and you get `ES2001: InvalidId can escape safeLookup` (the same error as in Chapter 2).

#### Cleanup with `finally`

`finally` runs when the block exits in any way: success, failure, or interruption.

```ts effect
const report = Effect.fn("report")(
  function* (id: string) {
    const reports = yield* Reports
    return yield* reports.build(id)
  },
  Effect.ensuring(Metrics.use((metrics) => metrics.increment("reports")))
)
```

```ts es
function report(id: string) {
  try {
    return Reports.build(id)
  } finally {
    Metrics.increment("reports")
  }
}
```

A cleanup that acquires and releases a resource is better written with `using` (Chapter 6).

#### Failures, defects, and interruption

Effect distinguishes three ways a computation can stop, and EffectScript keeps them apart:

```ts es
function lookupAudited(id: string) {
  try {
    try {
      return Users.find(id)             // fails with UserNotFound
    } finally {
      Audit.flush()                     // and this crashes with a TypeError: a defect
    }
  } catch (e) {                         // does not run: the exit also carries a defect
    return guest
  }
}
```

`catch` handles an expected failure only when nothing else went wrong. Here the exit carries both the `UserNotFound` and the crash, so the `catch` block does not run and the whole failure propagates, crash included. Effect's own `Effect.catch` would recover from this exit and drop the crash.

| What happened | Example | In E? | Caught by `catch`? | Runs `finally`? |
| --- | --- | --- | --- | --- |
| Expected failure | `throw new InvalidId(…)`, a callee's E | Yes | Yes | Yes |
| Defect: a bug | `(undefined as any).name` throws a `TypeError` | No | No | Yes |
| Interruption | a timeout, a cancelled request, a parent shutting down | No | No | Yes |
| An expected failure plus a defect or an interruption | `lookupAudited` above | No: E describes causes made only of expected failures, so this exit can carry a failure outside E (Chapter 14) | No: the whole exit propagates | Yes |
| Several expected failures | both branches of `Effect.race` failed | Yes | Yes: `e` is the first | Yes |

A `catch` that caught bugs or cancellation would hide them, so it does not. To deal with defects deliberately, use Effect's tools at a boundary: `Effect.catchDefect(() => risky(), …)`. To declare that an expected failure cannot happen here, turn it into a defect: `const id = Effect.orDie(() => parseId(raw))`. To see every failure in an exit, use `Effect.catchCause` or `Effect.exit`. Chapter 17 states the full rules.

#### Coming from `Effect.gen`

Two things that look the same mean different things, on purpose.

**`throw` is `Effect.fail`.** Inside `Effect.gen`, `throw new Error("unreachable")` is a defect. In EffectScript, every `throw` is a typed failure: `Error` joins E, and a `catch` can handle it. To signal a bug, die explicitly: `Effect.die(new Error("unreachable"))`.

**`catch` sees expected failures only.** Inside `Effect.gen`, a JavaScript `catch` sees any synchronous exception from the code in its `try`. In EffectScript, an exception that no contract lists is a defect, and `catch` does not see it. That covers Effect's "cannot fail here" functions, such as `Option.getOrThrow` and `Schema.decodeUnknownSync`, whose exceptions are defects (Chapter 10). Each has a form for code that can fail: `Option.getOrElse`, `Effect.fromOption`, `Schema.decodeUnknownEffect`. A `catch` that therefore can never run is reported, as a warning by default:

```ts es
function firstAdmin(users: ReadonlyArray<User>) {
  try {
    return Option.getOrThrow(Option.fromNullishOr(users.find(isAdmin)))
  } catch (e) {
    return guest
  }
}
```

```text
warning ES2004: this catch can never run: nothing in its try block fails with an expected error.
  admins.es:3:12   return Option.getOrThrow(Option.fromNullishOr(users.find(isAdmin)))
                          ^^^^^^^^^^^^^^^^^ its exceptions are defects
  Use a form that can fail:   return users.find(isAdmin) ?? guest
  or bound the call:          Effect.try({ try: () => …, catch: (cause) => new NoAdmin({ cause }) })
```
