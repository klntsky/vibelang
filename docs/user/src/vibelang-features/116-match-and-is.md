# Pattern matching with match and is

`match` picks the first arm whose pattern fits a value; `is` tests a value against one pattern. Both come from the TC39 [pattern matching](https://github.com/tc39/proposal-pattern-matching) proposal (stage 1) and compile to plain conditionals, so TypeScript narrows the value in each arm, and an arm can call Effect functions.

**`match` is exhaustive.** A match without a `default` arm over a finite union, such as a data type's variants, `error` classes or literals, must handle every case, or the compiler reports EK4701: `This match does not handle every case of Delivery: Delivery.Rejected is left.` See [Exhaustiveness](#exhaustiveness).

## match

```vibe
// run: Console.log(JSON.stringify([describe(new NotFound({ id: "7" })), describe(new Timeout()), describe(new Denied({ user: "Ada", reason: "admin" })), describe(new Denied({ user: "Ada", reason: "other" }))]))
error NotFound { id: string }
error Timeout
error Denied { user: string; reason: string }

type Failure = NotFound | Timeout | Denied

function describe(failure: Failure) {
  return match (failure) {
    when NotFound and { let id }: `no user ${id}`;
    when Timeout: "retry";
    when Denied and { reason: "root" or "admin" }: "denied by policy";
    when Denied: `denied ${failure.user}`;
  }
}
// Output: ["no user 7","retry","denied by policy","denied Ada"]
```

Each arm, `when pattern: expression;`, is tried in order; the first match gives the value. A `default: expression;` arm matches anything and must come last. `match` is an expression: it can follow `return`, initialize a `const`, or be an argument.

An arm is one expression. Use `do { }` for several statements; its value is its last statement's:

```vibe
// run: Console.log(retryDelay(new Timeout())); Console.log(retryDelay(new NotFound({ id: "7" })))
function retryDelay(failure: Failure) {
  return match (failure) {
    when Timeout: do {
      Console.log("timed out")
      "1 second"
    };
    default: "0 millis";
  }
}
// Output: timed out
// Output: 1 second
// Output: 0 millis
```

A function named `match` still works. Only `match (value) {` followed by `when`, `default`, or `}` starts a match.

## Patterns

| Pattern | Matches | Example |
| --- | --- | --- |
| Literal | An equal string, number, boolean, `null`, or `undefined` | `when 404:` |
| Binding | Anything, and names it with `let`, `const`, or `var` | `when let other:` |
| Array | An array of that length; `...` allows more | `when [let first, ...let rest]:` |
| Object | A value with those properties; `{ let id }` binds `id` | `when { status: 200, let body }:` |
| Matcher | An instance of a class, or what a custom matcher accepts | `when NotFound:` |
| Variant | A data type's variant, and the fields its braces list | `when Delivery.Sent { let tracking }:` |
| Extractor | A matcher's values, matched by the list in parentheses | `when Option.Some(let user):` |
| `and`, `or`, `not` | Conjunction, alternative, or negation | `when "a" or "e":` |
| Guard | A condition that is true | `when [let n] and if (n > 0):` |

```vibe
// run: Console.log(JSON.stringify([shape([]), shape([2]), shape([2, 3, 4]), shape({ kind: "box", size: 5 }), shape({ kind: "point", size: 1 }), shape({ kind: "other" })]))
function shape(value: ReadonlyArray<number> | { kind: string; size?: number }) {
  return match (value) {
    when []: "empty";
    when [let only]: `one: ${only}`;
    when [let first, ...let rest]: `${first} then ${rest.length} more`;
    when { kind: "box", let size }: `box of ${size}`;
    when { kind: "point", ...let others }: `point with ${Object.keys(others).length} more keys`;
    default: "other";
  }
}
// Output: ["empty","one: 2","2 then 2 more","box of 5","point with 1 more keys","other"]
```

An object pattern checks only the properties it names, and `...let rest` collects the others. A property that is not a valid name needs quotes, `{ "content-type": let type }`, and a computed key must be a name or a literal, `{ [key]: let value }`.

## Option, Exit, Result, and primitives

These matchers compile to the module's own test, so the bindings have exact types:

| Matcher | Test | Extracts |
| --- | --- | --- |
| `Option.Some`, `Option.None` | `Option.isSome`, `Option.isNone` | the value |
| `Exit.Success`, `Exit.Failure` | `Exit.isSuccess`, `Exit.isFailure` | the value, the cause |
| `Result.Success`, `Result.Failure` | `Result.isSuccess`, `Result.isFailure` | the success, the failure |
| `String`, `Number`, `Boolean`, `BigInt`, `Symbol` | `typeof` | the value |

```vibe
// run: Console.log(greeting(Option.some("Ada"))); Console.log(greeting(Option.none())); Console.log(report("ada")); Console.log(report("").includes("NotFound"))
function greeting(name: Option.Option<string>) {
  return match (name) {
    when Option.Some(let n): `Hello, ${n}`;
    when Option.None: "Hello";
  }
}

function report(id: string) {
  const outcome = exit(getUser(id))
  return match (outcome) {
    when Exit.Success(let user): user.name;
    when Exit.Failure(let cause): Cause.pretty(cause);
  }
}

function getUser(id: string): Effect<{ name: string }, NotFound> {
  if (id === "") throw new NotFound({ id })
  return { name: "Ada" }
}
// Output: Hello, Ada
// Output: Hello
// Output: Ada
// Output: true
```

## Data variants

A [`data` declaration](../data-types/073-data.md)'s variant is a pattern. `when Delivery.Sent:` tests the variant's tag, as `when { _tag: "Sent" }` does, and braces after it bind the fields they list:

```vibe
// run: Console.log(JSON.stringify([track(Delivery.Sent({ tracking: "parcel-8" })), track(Delivery.Sent({ tracking: "parcel-9", carrier: "rail" })), track(Delivery.Waiting())])); Console.log(carrierOf(Delivery.Sent({ tracking: "p", carrier: "rail" })))
data Delivery {
  Waiting
  Sent { tracking: string; carrier?: string }
  Rejected { reason: string }
}

function track(delivery: Delivery) {
  return match (delivery) {
    when Delivery.Sent { let tracking, carrier: "rail" }: `${tracking} by rail`;
    when Delivery.Sent { let tracking, let carrier }: `${tracking} by ${carrier ?? "post"}`;
    when Delivery.Waiting or Delivery.Rejected: "not sent";
  }
}

function carrierOf(delivery: Delivery) {
  if (delivery is Delivery.Sent { let carrier }) return carrier ?? "post"
  return "none"
}
// Output: ["parcel-8 by post","parcel-9 by rail","not sent"]
// Output: rail
```

A field in braces takes a pattern, `carrier: "rail"`, or binds its value, `let tracking`. The variant decides which fields a value has, so a field binds without a presence test: an optional field the value leaves out binds `undefined`. `is` narrows too: after `delivery is Delivery.Sent`, `delivery.tracking` is a `string`. A data type a `.ts` file declares with `Data.taggedEnum` matches the same way.

A variant's fields go in braces: `Delivery.Sent(let sent)` is EK4706, and so are braces after a matcher that is not a variant, `when NotFound { let id }`; write `when NotFound and { let id }`. A variant the subject's type has no case of is EK4707.

## Errors and custom matchers

An `error` class matches its own instances by `instanceof`, so two classes with the same tag stay distinct; as an extractor it gives the instance: `when NotFound(let error):`. Any other class also matches by `instanceof`.

A value with a `[customMatcher]` method decides for itself. `customMatcher` is the proposal's `Symbol.customMatcher`:

```vibe
// run: Console.log([parity(2), parity(3)].join(", "))
const Even = {
  [customMatcher](subject: unknown) {
    return typeof subject === "number" && subject % 2 === 0
  }
}

function parity(n: number) {
  return match (n) {
    when Even: "even";
    default: "odd";
  }
}
// Output: even, odd
```

For an extractor, the method receives `{ matchType: "extractor" }` as its second argument and returns `false` or the values to match against the list. Any other value matches when it is equal, with `NaN` matching `NaN`.

## Combining patterns

`and` requires both patterns, `or` accepts either, and `not` rejects its pattern. A guard, `if (condition)`, tests any expression and sees the bindings before it:

```vibe
// run: Console.log(JSON.stringify([classify({ status: "failed", attempts: 3 }), classify({ status: "failed", attempts: 1 }), classify({ status: "queued", attempts: 0 }), classify({ status: "done", attempts: 0 }), classify({ status: "cancelled", attempts: 0 })]))
function classify(job: { status: string; attempts: number }) {
  return match (job) {
    when { status: "failed", attempts: let n } and if (n >= 3): "escalate";
    when { status: "failed" }: "retry";
    when { status: not ("done" or "cancelled") }: "running";
    default: "leave";
  }
}
// Output: ["escalate","retry","running","leave","leave"]
```

`and`, `or`, and `not` cannot share a level: write `(a and b) or c`, not `a and b or c` (EK4705). An `or` or `not` pattern cannot bind names yet (EK4702).

## is

`value is pattern` is `true` when the pattern matches. It binds as tightly as `instanceof`:

```vibe
// run: Console.log(JSON.stringify([isRetryable(new Timeout()), isRetryable(new Denied({ user: "Ada", reason: "root" })), isRetryable(new NotFound({ id: "7" }))]))
function isRetryable(failure: Failure) {
  return failure is Timeout or Denied
}
// Output: [true,true,false]
```

In an `if` or `while` condition, joined by `&&`, the pattern can bind names, scoped to the rest of the condition and the `if`'s first branch or the loop's body:

```vibe
// run: Console.log(firstFailure("ada")); Console.log(firstFailure("").includes("NotFound"))
function firstFailure(id: string) {
  const outcome = exit(getUser(id))
  if (outcome is Exit.Failure(let cause) && Cause.hasFails(cause)) {
    return Cause.pretty(cause)
  }
  return "ok"
}
// Output: ok
// Output: true
```

Elsewhere, an `is` with bindings is EK4703.

## Exhaustiveness

`match` is exhaustive. A match without a `default` arm whose value is a finite union must handle every case: literals, `null`, `undefined`, a data type's variants, `error` classes, `Option`, `Exit`, `Result`, and other objects with a literal `_tag`. A missing case is a compile error, EK4701, `This match does not handle every case of {type}: {cases} is left.`:

```vibe
// expect-error: EK4701
type Phase = "draft" | "review" | "published"

function phaseLabel(phase: Phase) {
  return match (phase) {
    when "draft": "Draft";
    when "published": "Published";
  }
}
```

```text
error EK4701: This match does not handle every case of Phase: "review" is left.
```

For a data type, the message names the variants left:

```vibe
// expect-error: EK4701
data Shipment {
  Packed
  Shipped { carrier: string }
  Delivered
}

function shipmentLabel(shipment: Shipment) {
  return match (shipment) {
    when Shipment.Packed: "packed";
  }
}
```

```text
error EK4701: This match does not handle every case of Shipment: Shipment.Shipped | Shipment.Delivered is left.
```

A class pattern covers its class and its subclasses, never another class with the same fields: each package's `error NotFound` needs its own arm, and so does an error that has a family base's fields without extending it:

```vibe
// expect-error: EK4701
error AppError { _tag: string; code: string }
error Missing extends AppError { path: string }
error Other { code: string }

function describe(failure: Missing | Other) {
  return match (failure) {
    when AppError: failure.code;
  }
}
```

```text
error EK4701: This match does not handle every case of Missing | Other: Other is left.
```

A match that handles every case needs no `default`, and tests that reach every arm cover every branch. When its arms test only literals, shapes, `String` and the other primitive classes, `Option`, `Exit` or `Result`, its last arm runs without testing its pattern; a value outside the declared type, which only an unsound cast or `any` can produce, takes it, so write `default: throw …` to reject one at run time instead. A class, a custom matcher or another named matcher decides at run time what a type can't prove: a structural object of a class type is no instance. When an arm tests one, the last arm keeps its test without adding a branch, and a value that fails it throws a `TypeError`.

For any other type, a match without `default` that no arm fits throws a `TypeError` when it runs, as the proposal specifies.

## Errors

| Code | Cause | Fix |
| --- | --- | --- |
| EK4701 | A match without `default` misses a case of a finite union | Add an arm for each case left, or a `default` arm |
| EK4702 | A binding inside an `or` or `not` pattern | Bind outside the combinator, or give each alternative its own arm |
| EK4703 | An `is` with bindings outside an `if` or `while` condition | Move the test into the condition, or use `match` |
| EK4704 | A regular expression pattern, a relational pattern such as `< 10`, or a computed key other than a name or a literal | Use a guard: `and if (n < 10)` |
| EK4705 | `and`, `or`, and `not` at one level without parentheses | Add parentheses |
| EK4706 | Braces after a matcher that is not a data variant, or an argument list after a variant | `when NotFound and { let id }`; `when Delivery.Sent { let tracking }` |
| EK4707 | A data variant that the subject's type has no case of | Match one of the subject type's variants |

## Not yet supported

- Array patterns match arrays only, not other iterables.
- Object patterns do not match primitives, so `{ length: let n }` does not match a string.
- An extractor with a custom matcher types its bindings loosely; a class or a listed matcher types them exactly.
- `is` bindings are not in scope in an `else` branch or after the statement.
- A subject such as `user.role` is read once per test, not once, so a getter there runs more than once; assign it to a `const` first when that matters.
- A match inside a larger expression runs its arms in their own function, so an arm there cannot `yield`. A match after `return` or as a statement can.
- `async match` and the proposal's regular expression and relational patterns.
