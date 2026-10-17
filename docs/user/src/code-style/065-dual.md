# Dual APIs

Many Effect APIs accept either the data followed by options or options followed by the data. These are the data-first and data-last forms of a dual API. Effeck preserves both forms; when the data is a program, pass a zero-argument function to keep its execution under the operator's control.

## Data first or data last

```ts
const source = Stream.make(5, 10, 15)
const direct = source.map((n) => n / 5)
const stage = Stream.map((n: number) => n / 5)
const piped = pipe(source, stage)

function compare() {
  return [direct.runCollect(), piped.runCollect()]
}
// Both arrays are [1, 2, 3].
```

The direct form supplies `source` in the first call. The data-last form first builds a stage, then `pipe` supplies `source`. Both build the same kind of Stream value and defer stream processing until consumption. These builders and their returned stages are trusted Effect library functions.

## Direct calls and program pipelines

```ts
function lookup() {
  sleep("10 millis")
  return "found"
}

function directLookup() {
  return timeout(lookup(), "1 second")
}

function pipedLookup() {
  return pipe(
    () => lookup(),
    retry({ times: 2 }),
    timeout("1 second")
  )
}
```

The first argument to the program pipeline is a function, so retry and timeout control the whole computation. `Effect.retry({ times: 2 })` builds a stage; applying that stage in `pipe` runs the resulting program. Writing `lookup()` as the initial argument would run it before the operators receive it.

## Choosing a form

```ts
function boundedLookup() {
  return retry(lookup(), { times: 2 })
}

const reusableLookup = () => pipe(
  () => lookup(),
  retry({ times: 2 }),
  timeout("1 second")
)
```

A direct call suits one operator; a pipeline keeps several operators readable. Wrap the complete pipeline in a function when you want a reusable program rather than its result. For transformations that only use a completed value, ordinary statements are usually clearer than `Effect.map` or `Effect.flatMap`.
