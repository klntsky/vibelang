# Data

`Data` works in VibeLang exactly as in Effect. Use it for classes, tagged unions, and errors with Effect's equality and hashing.

## Creating values with structural equality

```vibe
// run: Console.log(Equal.equals(first, second), Equal.equals(["inbox", 5] as const, ["inbox", 5] as const), Equal.equals([2, 4, 6], [2, 4, 6]), Equal.equals(makeRoute("inbox"), makeRoute("inbox")))
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

```output
true true true true
```

In Effect 4, plain records, tuples, arrays, maps, and sets already compare structurally, nested values included, without a `Data` wrapper. Treat compared data as immutable: equality and hashing cache information about objects.

## Building ordinary and tagged classes

```vibe
// run: Console.log(label.upper, Equal.equals(label, new Label({ text: "pending" })), new Archived({ documentId: "doc-3" })._tag)
class Label extends Data.Class<{ text: string }> {
  get upper() { return this.text.toUpperCase() }
}

class Archived extends Data.TaggedClass("Archived")<{
  documentId: string
}> {}

const label = new Label({ text: "pending" })
label.upper // "PENDING"
Equal.equals(label, new Label({ text: "pending" })) // true
new Archived({ documentId: "doc-3" })._tag // "Archived"
```

```output
PENDING true Archived
```

`Class` supplies a constructor taking a field object, plus equality and hashing; `TaggedClass` adds a fixed `_tag` for narrowing union members. These concrete data classes are not the abstract classes that declare services.

## Defining tagged unions with matching helpers

```vibe
// run: Console.log(isSent(sent)); Console.log(describe(waiting)); Console.log(describe(sent)); Console.log(describe(rejected))
type Delivery = Data.TaggedEnum<{
  Waiting: {}
  Sent: { tracking: string }
  Rejected: { reason: string }
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

```output
true
Waiting for dispatch
Tracking: parcel-8
Rejected: address missing
```

`TaggedEnum` describes the union as a map from tags to payloads. `taggedEnum` creates constructors that insert `_tag`, a `$is` guard factory, and a `$match` helper requiring a case per variant. Match callbacks compute values synchronously; for service work, narrow in an ordinary function first.

## Adding generic union cases

```vibe
// run: Console.log(JSON.stringify([loaded, failed, idle]))
type LoadState<A, E> = Data.TaggedEnum<{
  Idle: {}
  Ready: { value: A }
  Failed: { error: E }
}>

interface LoadStateDefinition extends Data.TaggedEnum.WithGenerics<2> {
  taggedEnum: LoadState<this["A"], this["B"]>
}

const LoadState = Data.taggedEnum<LoadStateDefinition>()
const loaded = LoadState.Ready({ value: ["one", "two"] })
const failed = LoadState.Failed({ error: "offline" })
const idle = LoadState.Idle()
```

```output
[{"value":["one","two"],"_tag":"Ready"},{"error":"offline","_tag":"Failed"},{"_tag":"Idle"}]
```

`WithGenerics<2>` gives the definition two type parameters, `A` and `B`; the constructors infer payload types per call. Keep the generic definition in a type or interface; it is not a generic service class.

## Defining ordinary and tagged errors

```vibe
// run: Console.log(new InvalidDocument({ message: "invalid", line: 3 }).line); Console.log(JSON.stringify([documentOrDraft("doc-3"), documentOrDraft("")])); try { requireDocument("") } catch (error) { Console.log(error._tag) }
class InvalidDocument extends Data.Error<{
  message: string
  line: number
}> {}

error DocumentMissing {
  id: string
}

function requireDocument(id: string) {
  if (id === "") throw new DocumentMissing({ id })
  return { id }
}
// { id: string }!DocumentMissing!

function documentOrDraft(id: string) {
  try {
    return requireDocument(id)
  } catch (error) {
    return { id: "draft" }
  }
}
```

```output
3
[{"id":"doc-3"},{"id":"draft"}]
DocumentMissing
```

`Data.Error` creates a native error with extra fields. The global `TaggedError` is Effect's `Data.TaggedError`, which fills in `_tag`; `throw` raises it as an expected failure. Use `Schema.TaggedError` when the error needs a schema for serialization.

## Preserving native error causes

```vibe
// run: Console.log(JSON.stringify(decodeDocument("{\"id\":3}"))); try { decodeDocument("{") } catch (error) { Console.log(error._tag, error.cause instanceof SyntaxError) }
error DocumentDecodeError {
  cause: unknown
}

function decodeDocument(text: string) {
  return JSON.parse(text) catch (cause) => throw new DocumentDecodeError({ cause })
}
// any!DocumentDecodeError!
```

```output
{"id":3}
DocumentDecodeError true
```

A `cause` field keeps the original exception through native error-cause support; it is not Effect's `Cause`, which describes a whole execution failure. Keep the original error for diagnosis, and keep document contents and credentials out of diagnostic fields.

## Coming from Effect

Raise a data error with `throw new DocumentMissing({ id })`; ordinary error propagation needs no generator yield or `Effect.fail`.
