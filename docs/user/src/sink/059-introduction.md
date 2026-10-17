# Introduction

A Sink describes how to consume a Stream and return one result. Use a sink when a consumption rule should be reusable: take a header, count records, accumulate totals, or run work for every element. Building a Sink does not consume anything.

## Consumers and result types

```ts
const header: Sink.Sink<Array<string>, string, string, never, never> = Sink.take<string>(2)
```

`Sink.Sink<A, In, L, E, R>` returns `A`, accepts input elements `In`, and can leave unconsumed elements of type `L`. `E` describes its typed failures and `R` its required services. A sink can consume no elements, stop early, or read the entire source before returning.

Sink constructors and operators that return Sink values are trusted library calls. Call them directly to build or transform sinks; exceptions from plain library calls are defects. A producer also works as the source of `Stream.run`, because it converts to a Stream wherever one is expected.

## Running a consumer against a source

```ts
function parseHeader() {
  const source = Stream.make("format-v2", "length-64", "payload")
  const header = Sink.take<string>(2)
  return source.run(header)
}
// Array<string>!!
// Returns ["format-v2", "length-64"].
```

`Stream.run` runs at the call and returns the sink's result. Failures and services from both source and sink join the consuming function's contract. Provide those services around consumption, or supply them through the source and sink's own provision operators.

## Unconsumed elements

```ts
function headerWithLeftovers() {
  const sink = (Sink.take<string>(1)).mapEnd(([header, rest]) => [
    { header, rest: rest ?? [] }
  ] as const)
  return Stream.fromArray(["header", "first", "second"]).run(sink)
}
// Returns { header: ["header"], rest: ["first", "second"] }.
```

Streams deliver batches internally. When the sink stops within a batch, its unused suffix becomes leftovers. This differs from the entire unread tail of the source, which can include batches that were never requested. `Stream.run` returns only `A`; use `Sink.mapEnd` to move leftovers into `A` when they matter.

## Coming from Effect

Sink's five type parameters and operators keep their Effect meanings. Calls that construct a Sink build a value; `Stream.run(source, sink)` runs an Effect and returns the sink's success value directly.
