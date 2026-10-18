# Fallback

A fallback runs another computation after an expected failure. Pass computations as functions so the fallback starts only when needed.

[Upstream page](https://effect.website/docs/v4/error-management/fallback)

## Recovering with another computation

```effeck
error SourceOffline

function latestTitle(): string!SourceOffline! {
  throw new SourceOffline()
}

function cachedTitle() {
  log("Reading saved title")
  return "Saved issue"
}

function title() {
  try {
    return latestTitle()
  } catch (error) {
    return cachedTitle()
  }
}
// string!!
```

The cache is read only if the primary computation fails. Errors from the fallback become errors of `title`; handling the primary does not handle the fallback too. Ordinary catch propagates defects, interruption, and mixed causes containing either.

```effeck
function titleInline() {
  return latestTitle() catch cachedTitle()
}
// string!!
```

`x catch fallback` is the expression form. The left side runs inside the catch, and the fallback runs only after an expected failure, so the cache is read only when needed. It recovers exactly what the statement recovers. `catch` binds looser than `??` and `||` and tighter than `? :`, so `a ?? b catch c` applies the default inside the catch.

`Effect.catch(() => latestTitle(), () => cachedTitle())` also works. Unlike source catch, Effect's recovery combinators can consume a mixed cause containing an expected failure. Use a catch statement or expression, narrowing and rethrowing, when defects and interruption must propagate intact; `Effect.catchTag`, `Effect.catchIf`, and `Effect.catchFilter` also select domain failures, with Effect's own recovery policy.

## Supplying a default success

```effeck
function titleOrPlaceholder() {
  return latestTitle() catch "No title available"
}
// string!!
```

`x catch value` is `Effect.orElseSucceed(x, () => value)`: the value is computed only after an expected failure, and every expected error of `x` is discarded. To keep some failures visible, take the failure and rethrow the rest: `latestTitle() catch (error) => match (error) { when SourceOffline: "No title available"; default: throw error; }`. The fallback is Effeck code, so it may also call functions that fail or need services; their errors become the expression's. `Effect.orElseSucceed(latestTitle(), () => "No title available")` still works, with a synchronous callback.

## Trying alternatives until one succeeds

```effeck
function bestTitle() {
  return Effect.firstSuccessOf([
    () => latestTitle(),
    () => cachedTitle(),
    () => "Untitled issue"
  ])
}
```

`firstSuccessOf` tries programs sequentially in priority order and stops on success. If all alternatives fail with expected errors, it reports the final error. A cause without an expected failure stops the search; a mixed cause containing one can activate the next alternative. An empty iterable is a defect, so construct at least one alternative.

A chain of catch expressions also tries alternatives in order: `latestTitle() catch cachedTitle() catch "Untitled issue"` tries the cache after the primary fails and settles on the title after the cache fails, because `catch` associates to the left. Unlike `firstSuccessOf`, it stops at a mixed cause, as every source catch does.

## Coming from Effect

Write alternatives as `() => fetchTitle()` rather than calling `fetchTitle()` while assembling the collection. A call runs immediately in Effeck.
