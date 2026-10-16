# Data

`Data` works in EffectScript exactly as in Effect. Use it for classes, tagged unions, and errors with Effect's equality and hashing support; ordinary objects and arrays already compare structurally in Effect 4.

## Creating values with structural equality

```ts
import { Equal } from "effect"

const first = { route: "inbox", filters: { unread: true } }
const second = { route: "inbox", filters: { unread: true } }
Equal.equals(first, second) // true
Equal.equals(["inbox", 5] as const, ["inbox", 5] as const) // true
Equal.equals([2, 4, 6], [2, 4, 6]) // true

function makeRoute(name: string) {
  return { _tag: "Route" as const, name }
}
Equal.equals(makeRoute("inbox"), makeRoute("inbox")) // true
```

Plain records, tuples, arrays, maps, and sets need no `Data` wrapper for structural equality. Comparison descends into nested values. Treat compared data as immutable: equality and hashing cache information about objects.

## Building ordinary and tagged classes

```ts
import { Data, Equal } from "effect"

class Label extends Data.Class<{ readonly text: string }> {
  get upper() { return this.text.toUpperCase() }
}

class Archived extends Data.TaggedClass("Archived")<{
  readonly documentId: string
}> {}

const label = new Label({ text: "pending" })
label.upper // "PENDING"
Equal.equals(label, new Label({ text: "pending" })) // true
new Archived({ documentId: "doc-3" })._tag // "Archived"
```

`Class` supplies a field-object constructor plus equality and hashing. `TaggedClass` also supplies a fixed `_tag`, useful when narrowing union members. These concrete data classes are distinct from the abstract classes used to declare services.

## Defining tagged unions with matching helpers

```ts
import { Data } from "effect"

type Delivery = Data.TaggedEnum<{
  Waiting: {}
  Sent: { readonly tracking: string }
  Rejected: { readonly reason: string }
}>

const Delivery = Data.taggedEnum<Delivery>()
const sent = Delivery.Sent({ tracking: "parcel-8" })
const waiting = Delivery.Waiting()
const rejected = Delivery.Rejected({ reason: "address missing" })
const isSent = Delivery.$is("Sent")
isSent(sent) // true

const describe = Delivery.$match({
  Waiting: () => "Waiting for dispatch",
  Sent: ({ tracking }) => `Tracking: ${tracking}`,
  Rejected: ({ reason }) => `Rejected: ${reason}`
})
describe(sent) // "Tracking: parcel-8"
```

`TaggedEnum` describes the union from a mapping of tags to payloads. `taggedEnum` creates constructors that insert `_tag`, a `$is` guard factory, and a `$match` helper that requires a case for each variant. Matching callbacks here are synchronous value computations; run service work after narrowing in an ordinary function.

## Adding generic union cases

```ts
import { Data } from "effect"

type LoadState<A, E> = Data.TaggedEnum<{
  Idle: {}
  Ready: { readonly value: A }
  Failed: { readonly error: E }
}>

interface LoadStateDefinition extends Data.TaggedEnum.WithGenerics<2> {
  readonly taggedEnum: LoadState<this["A"], this["B"]>
}

const LoadState = Data.taggedEnum<LoadStateDefinition>()
const loaded = LoadState.Ready({ value: ["one", "two"] })
const failed = LoadState.Failed({ error: "offline" })
const idle = LoadState.Idle()
```

`WithGenerics<2>` gives the definition two type parameters, addressed as `A` and `B`. The generated constructors infer their payload types at each call. Keep the generic definition in a type or interface; it is not a generic service class.

## Defining ordinary and tagged errors

```ts
import { Data } from "effect"

class InvalidDocument extends Data.Error<{
  readonly message: string
  readonly line: number
}> {}

class DocumentMissing extends TaggedError("DocumentMissing")<{
  readonly id: string
}> {}

function requireDocument(id: string) {
  if (id === "") throw new DocumentMissing({ id })
  return { id }
}
// Effect<{ id: string }, DocumentMissing, never>

function documentOrDraft(id: string) {
  try {
    return requireDocument(id)
  } catch (error) {
    if (error instanceof DocumentMissing) return { id: "draft" }
    throw error
  }
}
```

`Data.Error` creates a native error with additional fields. The global `TaggedError` is Effect's `Data.TaggedError`, with an automatically populated `_tag`; use `throw` to raise it as an expected failure. Use `Schema.TaggedError` when the error needs a schema for serialization.

## Preserving native error causes

```ts
class DocumentDecodeError extends TaggedError("DocumentDecodeError")<{
  readonly cause: unknown
}> {}

function decodeDocument(text: string) {
  return Effect.try({
    try: () => JSON.parse(text),
    catch: (cause) => new DocumentDecodeError({ cause })
  })
}
// Effect<any, DocumentDecodeError, never>
```

A `cause` field preserves the original exception using native error-cause support. It is separate from Effect's `Cause`, which describes an entire execution failure. Preserve the original error for diagnosis, and avoid adding document contents or credentials to diagnostic fields.

## Coming from Effect

Raise a data error with `throw new DocumentMissing({ id })`. Generator yielding and `Effect.fail` are unnecessary for ordinary EffectScript error propagation.
