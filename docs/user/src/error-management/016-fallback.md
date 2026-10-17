# Fallback

A fallback runs another computation after an expected failure. Pass computations as functions so the fallback starts only when needed.

[Upstream page](https://effect.website/docs/v4/error-management/fallback)

## Recovering with another computation

```ts
error SourceOffline

function latestTitle() {
  throw new SourceOffline()
}

function cachedTitle() {
  Effect.log("Reading saved title")
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

The expression form is `Effect.catch(() => latestTitle(), () => cachedTitle())`. Unlike source catch, Effect's recovery combinators can consume a mixed cause containing an expected failure. Use narrowing and rethrowing in ordinary catch when defects and interruption must propagate intact; `Effect.catchTag`, `Effect.catchIf`, and `Effect.catchFilter` also select domain failures, with Effect's own recovery policy.

## Supplying a default success

```ts
function titleOrPlaceholder() {
  return Effect.orElseSucceed(latestTitle(), () => "No title available")
}
// string!!
```

`orElseSucceed` computes its value lazily after a typed failure. It discards every typed error from the source; select one error first when other failures need to remain visible. Its value callback is synchronous, so use an effectful recovery function when producing the fallback needs services or waiting.

## Trying alternatives until one succeeds

```ts
function bestTitle() {
  return Effect.firstSuccessOf([
    () => latestTitle(),
    () => cachedTitle(),
    () => "Untitled issue"
  ])
}
```

`firstSuccessOf` tries programs sequentially in priority order and stops on success. If all alternatives fail with expected errors, it reports the final error. A cause without an expected failure stops the search; a mixed cause containing one can activate the next alternative. An empty iterable is a defect, so construct at least one alternative.

## Coming from Effect

Write alternatives as `() => fetchTitle()` rather than calling `fetchTitle()` while assembling the collection. A call runs immediately in Effeck.
