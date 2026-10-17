# Control Flow Operators

Effeck uses ordinary branches and loops for sequential work. Effect's operators remain useful when you need concurrent execution, a collected set of results, or a policy that applies to whole programs. Pass functions to operators so they decide when each computation starts.

## Branching and conditional computations

```ts
error InvalidSize {
  size: number
}

function optionalSize(size: number) {
  if (size < 0) return Option.none<number>()
  return Option.some(size)
}

function requiredSize(size: number) {
  if (size < 0) throw new InvalidSize({ size })
  return size
}
// number!InvalidSize!

function announceIfEnabled(enabled: boolean) {
  return Effect.when(() => enabled) {
    log("notifications enabled")
    return "announced"
  }
}
```

`if`, early `return`, and `throw` work inside an Effeck function, including after a service call. `Effect.when` evaluates its condition program first, runs its source only for `true`, and returns an `Option` indicating whether it ran. `false` produces `None`; a condition failure propagates instead of skipping work.

## Combining two results

```ts
function amount() {
  sleep("30 millis")
  return 18
}

function currency() {
  sleep("10 millis")
  return "USD"
}

function sequentialPair() {
  const value = amount()
  const unit = currency()
  return [value, unit] as const
}

function concurrentPair() {
  return Effect.zip(amount(), currency(), { concurrent: true })
}

function formattedPair() {
  return Effect.zipWith(
    () => amount(),
    () => currency(),
    (value, unit) => `${value} ${unit}`
  )
}
```

`zip` returns a tuple and `zipWith` applies a synchronous combining callback. Their default execution is left then right; `{ concurrent: true }` starts both branches together. Concurrent completion order does not change tuple positions. A failed branch interrupts its sibling and waits for cleanup before the combined operation finishes.

## Repeating work in loops

```ts
function inspectPages() {
  const inspected: number[] = []
  let page = 1
  while (page <= 3) {
    sleep("10 millis")
    inspected.push(page)
    page++
  }
  return inspected // [1, 2, 3]
}

function logAttempts() {
  for (let attempt = 1; attempt <= 3; attempt++) {
    log(`attempt ${attempt}`)
  }
}
```

A loop can call services, pause, fail, and use `break` or `continue`. Keep accumulated results only when the caller needs them; a loop without an explicit return completes with `void`. Local state is recreated when the function runs again.

```ts
function operatorLoop() {
  let index = 0
  Effect.whileLoop({
    while: () => index < 3,
    body: () => log(`iteration ${index}`),
    step: () => { index++ }
  })
}
```

In Effect 4.0.5, `whileLoop` accepts one options object and returns `void`. Its `body` lazily supplies an Effect, and `step` synchronously updates state after a successful iteration. It has no initial-state argument or result-collection option; use local state and an array when you need results.

## Traversing collections and discarding values

```ts
function prepare(id: number) {
  sleep("10 millis")
  return `prepared-${id}`
}

function prepareAll() {
  return forEach([3, 5, 8], (id, index) => {
    log(`preparing position ${index}`)
    return prepare(id)
  }, { concurrency: 2 })
}

function logAll() {
  forEach([3, 5, 8], (id) => log(`id ${id}`), { discard: true })
}
```

`forEach` returns results in input order and defaults to sequential execution. A concurrency limit overlaps work while bounding the number of active callbacks. `{ discard: true }` runs all work without retaining callback results. A failure stops traversal; concurrent callbacks already running are interrupted and cleaned up.

## Collecting programs

```ts
function collected() {
  const tuple = all([() => 12, () => "ready"] as const)
  const object = all({ count: () => 12, status: () => "ready" })
  const iterable = new Set([() => prepare(3), () => prepare(5)])
  const array = all(iterable, { concurrency: 2 })
  return { tuple, object, array }
}
// tuple: [12, "ready"]
// object: { count: 12, status: "ready" }
// array contains the two prepared values in iteration order.
```

`all` accepts tuples, iterables, objects, and records, returning the corresponding result shape. Each entry is a program, so building the collection performs no work. The default is sequential; supply `concurrency` explicitly for overlap. `discard: true` suppresses the combined result when only completion matters.

## Early termination and alternate collection modes

```ts
error Unavailable {
  id: number
}

function load(id: number) {
  if (id === 5) throw new Unavailable({ id })
  log(`loaded ${id}`)
  return `item-${id}`
}

function stopAtFailure() {
  return all([load(3), load(5), load(8)])
}
// Sequentially loads 3, then fails with Unavailable; 8 is not started.

function collectOutcomes() {
  return all([load(3), load(5), load(8)], { mode: "result" })
}
// [Result.succeed("item-3"), Result.fail(Unavailable), Result.succeed("item-8")]
```

The default mode stops on a typed failure and keeps that failure in the consuming function's `E`. `mode: "result"` runs every program and puts each typed outcome in a Result value instead. It does not turn defects or interruption into ordinary successful outcomes. Combine it with a concurrency limit when independent jobs should all report their expected outcomes.

## Coming from Effect

Ordinary statements replace sequential mapping and binding. Keep program combinators for concurrency and collection policies, and always pass `() => work()` when an operator expects a program.
