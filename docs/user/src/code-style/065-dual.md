# Dual APIs

Many Effect APIs are dual: they take the data then the options (data-first), or the options then the data (data-last). VibeLang keeps both. With the pipe operator, `|>`, and method syntax on Effect's data types, data-first usually reads best.

## Data first or data last

```vibe
// run: Console.log(JSON.stringify(compare()))
const source = Stream.make(5, 10, 15)
const direct = source.map((n) => n / 5)
const stage = Stream.map((n: number) => n / 5)
const piped = pipe(source, stage)

function compare() {
  return [direct.runCollect(), piped.runCollect()]
}
// Both arrays are [1, 2, 3].
```

```output
[[1,2,3],[1,2,3]]
```

The direct form supplies `source` in the first call; the data-last form builds a stage, then `pipe` supplies `source`. Both build the same kind of Stream value and process nothing until consumed. The builders and their stages are trusted Effect library functions.

## Direct calls and program pipelines

```vibe
// run: Console.log(directLookup(), pipedLookup())
function lookup() {
  sleep("10 millis")
  return "found"
}

function directLookup() {
  return timeout(lookup(), "1 second")
}

function pipedLookup() {
  return lookup() |> retry(%, { times: 2 }) |> timeout(%, "1 second")
}
```

```output
found found
```

Each `%` stands for the expression before its `|>`. Where `%` is an argument that expects a program, such as the first argument of `retry` and `timeout`, the expression is passed by name: retry reruns `lookup()` on each attempt, and timeout limits the whole retried computation. Where `%` is an argument that expects a value, the expression runs once and `%` is its result.

## Choosing a form

```vibe
// run: Console.log(boundedLookup(), reusableLookup())
function boundedLookup() {
  return retry(lookup(), { times: 2 })
}

const reusableLookup = () => lookup() |> retry(%, { times: 2 }) |> timeout(%, "1 second")
```

```output
found found
```

A direct call suits one operator; a pipe keeps several readable. Wrap the whole pipe in a function for a reusable program instead of its result. To transform a completed value, ordinary statements are usually clearer than `Effect.map` or `Effect.flatMap`.
