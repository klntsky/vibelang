# Metrics in Effeck

Effeck uses Effect's `Metric` module to count events, record current values, and measure distributions over time. Metric constructors build values exactly as in Effect; updates, state reads, and instrumented programs run when called.

## Recording counters and gauges

```ts
const requests = Metric.counter("guide_requests", { incremental: true })
const queued = Metric.gauge("guide_queued")
const bytes = Metric.counter("guide_bytes", { bigint: true })

function recordBatch() {
  Metric.update(requests, 1)
  Metric.update(requests, 3)
  Metric.update(queued, 12)
  Metric.update(queued, 5)
  Metric.update(bytes, 4096n)
  return {
    requests: Metric.value(requests).count,
    queued: Metric.value(queued).value
  }
}
```

A counter accumulates changes, while a gauge retains its most recent value. `incremental: true` ignores negative counter inputs; counters without it also support decrements, and both counters and gauges support bigint variants. Metric state is shared across updates to the same metric identity, so totals above accumulate across repeated runs.

```ts
const completions = Metric.withConstantInput(
  Metric.counter("guide_completions", { incremental: true }),
  1
)

function instrumentedValue() {
  return pipe(() => "processed", Effect.trackSuccesses(completions))
}
```

`trackSuccesses` sends a successful return value to a metric without changing the program's result. `withConstantInput` replaces that input with one, so this metric counts successful completions regardless of the returned string.

## Measuring distributions

```ts
const batchSizes = Metric.histogram("guide_batch_size", {
  boundaries: Metric.linearBoundaries({ start: 0, width: 20, count: 6 })
})
const recentSizes = Metric.summary("guide_recent_batch_size", {
  maxAge: "10 minutes",
  maxSize: 200,
  quantiles: [0.5, 0.9, 0.99]
})

function recordSizes() {
  for (const size of [12, 36, 8, 91]) {
    Metric.update(batchSizes, size)
    Metric.update(recentSizes, size)
  }
  return {
    histogram: Metric.value(batchSizes),
    summary: Metric.value(recentSizes)
  }
}
```

A histogram groups observations into configured buckets and tracks count, sum, minimum, and maximum. Bucket boundaries control its precision and support aggregation across instances. A summary retains bounded recent samples and computes quantiles over that window, with `maxAge` and `maxSize` limiting retention; its calculated quantiles are not values to average across instances.

## Timing computations

```ts
const renderTime = Metric.timer("guide_render_duration", {
  boundaries: [5, 20, 50, 100, 500]
})

function render() {
  return pipe(() => {
    sleep("30 millis")
    return "rendered"
  }, Effect.trackDuration(renderTime))
}
```

`trackDuration` measures the wrapped computation and records its duration without replacing its return value. `Metric.timer` is a duration-input histogram whose bucket boundaries and stored values use milliseconds. The measurement includes time spent waiting inside the wrapped program.

## Counting categorical events

```ts
const outcomes = Metric.frequency("guide_validation_outcomes")

function recordOutcomes() {
  Metric.update(outcomes, "accepted")
  Metric.update(outcomes, "rejected")
  Metric.update(outcomes, "accepted")
  return Metric.value(outcomes).occurrences
}
```

A frequency metric keeps a count per observed string. Use a bounded set of categories such as result kinds or error tags. User IDs and request IDs produce an unbounded number of categories and belong in logs or span attributes instead.

## Attaching attributes to metrics and regions

```ts
const apiRequests = Metric.withAttributes(
  Metric.counter("guide_api_requests", { incremental: true }),
  { route: "/reports" }
)
const failures = Metric.counter("guide_api_failures", { incremental: true })

function attributed() {
  return Effect.provideService(() => {
    Metric.update(apiRequests, 1)
    Metric.update(failures, 1)
    return {
      requests: Metric.value(apiRequests),
      failures: Metric.value(failures)
    }
  }, Metric.CurrentMetricAttributes, { environment: "staging" })
}
```

`withAttributes` adds attributes to one metric. `CurrentMetricAttributes` supplies attributes to every metric operation in a region, including reads; read state under the same attributes used to update it. Each attribute combination identifies a separate series, so prefer bounded categories such as deployment and route.
