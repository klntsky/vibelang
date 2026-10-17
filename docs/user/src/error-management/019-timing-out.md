# Timing Out

A timeout limits how long a caller waits for work to finish. Effect interrupts the timed computation, waits for its cleanup, and then produces the chosen timeout outcome.

[Upstream page](https://effect.website/docs/v4/error-management/timing-out)

## Limiting execution duration

```ts
function buildPreview() {
  Effect.sleep("3 seconds")
  return "Rendered preview"
}

function previewWithDeadline() {
  return Effect.timeout(buildPreview(), "500 millis")
}
// string!Cause.TimeoutError!
```

`Effect.timeout` adds `Cause.TimeoutError` to `E`; import `Cause` from `effect` if you write that type explicitly. A timely success returns its original value, and a domain failure before the deadline keeps its original error. The source is a program function so the timer surrounds the work itself.

```ts
function previewLabel() {
  try {
    return previewWithDeadline()
  } catch (error) {
    // error._tag is "TimeoutError"
    return "Preview still unavailable"
  }
}
```

The interruption used internally to stop the source is separate from the typed timeout error returned to its caller. Ordinary catch inside `buildPreview` cannot consume that interruption. Cleanup failures remain visible in the resulting cause.

## Representing timeout as absence

```ts
function optionalPreview() {
  const preview = Effect.timeoutOption(() => buildPreview(), "500 millis")
  return Option.isNone(preview) ? "No preview yet" : preview.value
}
```

`timeoutOption` returns `None` for a timeout and `Some(value)` for a timely success. Expected failures from the source stay in `E`. It does not treat every failed computation as absence.

## Choosing an alternative or custom failure

```ts
function previewOrPlaceholder() {
  return Effect.timeoutOrElse(buildPreview(), {
    duration: "500 millis",
    orElse: () => "Static placeholder"
  })
}

error PreviewDeadline {
  documentId: string
}

function requiredPreview(documentId: string) {
  return Effect.timeoutOrElse(buildPreview(), {
    duration: "500 millis",
    orElse: () => { throw new PreviewDeadline({ documentId }) }
  })
}
// string!PreviewDeadline!
```

The alternative runs lazily after the timer wins. It can use services, fail, or return another success type, and those channels join the overall contract. Use a typed failure for an expected deadline; a defect is appropriate only when timing out breaks an invariant.

## Interruption of protected work

```ts
function protectedPreview() {
  return Effect.timeout(
    () => Effect.uninterruptible(buildPreview()),
    "500 millis"
  )
}
```

Uninterruptible work defers cancellation, so this call can take longer than 500 milliseconds. `finally` and resource releases complete before timeout returns, including releases that take time. Model work that must outlive a request with an explicit fiber lifetime rather than assuming a timeout stops it instantly.
