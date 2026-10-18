# Producers

A producer yields values one at a time and can run Effects between yields. Use `function*` to declare one and `for...of` to consume it.

## Yield values on demand

```effeck
function* readings() {
  try {
    for (const celsius of [18, 21, 25, 19, 28, 30]) {
      yield celsius
    }
  } finally {
    Console.log("sensor closed")
  }
}

function firstAlert() {
  for (const celsius of readings()) {
    if (celsius >= 25) return celsius
  }
  return undefined
}
// number | undefined!!
```

Calling `readings()` constructs a lazy source. Its body starts when a consumer asks for a value. Each `yield` hands one value to the loop, and the producer waits until the consumer continues.

Here the consumer returns at 25. Before `firstAlert` returns, the producer's `finally` logs `sensor closed`. `break`, a return, or a failure in the loop body closes the source and runs its cleanup. Iterating the source again starts a new run.

In `.effeck`, `function*` declares a producer. Do not use it to write `Effect.gen` or other generator-based APIs. Generator methods on objects remain JavaScript generators.

## Call services between yields

```effeck
import { FileSystem } from "effect/FileSystem"
import { Path } from "effect/Path"

function* fileTexts(directory: string) {
  const names = FileSystem.readDirectory(directory).sort()
  for (const name of names) {
    const path = Path.join(directory, name)
    const info = FileSystem.stat(path)
    if (info.type === "File") {
      yield { path, text: FileSystem.readFileString(path) }
    }
  }
}

function firstLargeFile(directory: string) {
  for (const file of fileTexts(directory)) {
    if (file.text.length > 1000) return file.path
  }
  return undefined
}
```

Service calls run as they do in an ordinary function. The producer's failures and requirements join the consuming function's contract. Provide `NodeServices.layer` at the application boundary to supply `FileSystem` and `Path`, as in `examples/file-tree`.

A producer can acquire resources with `using`. They stay open between yields and close when consumption finishes or stops early. This example reads one whole file at a time; stopping avoids reading the remaining files.

## Use a producer as a Stream

```effeck
function alerts() {
  const selected = readings().filter((value) => value >= 25).map((value) => `${value} C`).take(2)
  const result: string[] = []
  for (const alert of selected) result.push(alert)
  return result
  // ["25 C", "28 C"]; sensor closed
}
```

A producer has type `Stream.Stream<Value, Error, Requirements>`. Stream operators accept it directly, and TypeScript consumers see a Stream in its declaration file. The `examples/pipeline` program uses this filter, map, and take pattern.

When Stream operators consume a producer, the runtime starts it on its own fiber and uses a handoff with no buffer. The producer yields only when the Stream pulls; closing the Stream interrupts the producer and runs cleanup. Handle control operations inside that producer before sending it to Stream operators.

## Iterate any Stream

```effeck
function sumStream() {
  let total = 0
  for (const value of Stream.fromIterable([1, 2, 3])) {
    total += value
  }
  return total
}
```

`for...of` pulls an Effect Stream and closes it when the loop exits, including on `break`. Arrays, maps, sets, and strings keep JavaScript's ordinary iteration behavior.

Use `for (const value of source)` for producers. This preview does not support `for (using value of source)` over a producer, and `Array.from(source)` is not a Stream consumer. Use `Stream.runCollect(source)` when you need all the elements as a `Chunk`.

## Coming from Effect

A generator wrapped in `Stream.unwrap` becomes a `function*`. Write calls between `yield` statements; reserve `yield` for the values the producer emits. Use Stream's existing operators whenever they express the transformation you need.
