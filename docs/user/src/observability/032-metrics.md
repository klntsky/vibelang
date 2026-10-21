# Metrics in VibeLang

Effect's `Metric` module counts events, records current values, and measures distributions over time. Metric constructors build values exactly as in Effect; updates, state reads, and instrumented programs run when called.

## Recording counters and gauges

```vibe
// run: Console.log(JSON.stringify(recordBatch())); Console.log(String(Metric.value(bytes).count)); Console.log(JSON.stringify(recordBatch()))
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

```output
{"requests":4,"queued":5}
4096
{"requests":8,"queued":5}
```

A counter accumulates changes; a gauge keeps its latest value. `incremental: true` ignores negative counter inputs; without it, a counter also decrements. Counters and gauges both have bigint variants. Updates to the same metric identity share state, so the totals above accumulate across runs.

```vibe
// run: Console.log(instrumentedValue(), Metric.value(completions).count)
const completions = Metric.withConstantInput(
  Metric.counter("guide_completions", { incremental: true }),
  1
)

function instrumentedValue() {
  return pipe(() => "processed", Effect.trackSuccesses(completions))
}
```

```output
processed 1
```

`trackSuccesses` sends a successful return value to a metric and leaves the program's result unchanged. `withConstantInput` replaces that input with one, so this metric counts successes whatever the string.

## Measuring distributions

```vibe
// run: const result = recordSizes(); Console.log(JSON.stringify({ count: result.histogram.count, sum: result.histogram.sum, min: result.histogram.min, max: result.histogram.max, buckets: result.histogram.buckets })); Console.log(result.summary.count, result.summary.sum, result.summary.min, result.summary.max)
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

```output
{"count":4,"sum":147,"min":8,"max":91,"buckets":[[20,2],[40,3],[60,3],[80,3],[null,4]]}
4 147 8 91
```

A histogram sorts observations into configured buckets and tracks count, sum, minimum, and maximum; its boundaries set precision and allow aggregation across instances. A summary keeps recent samples, bounded by `maxAge` and `maxSize`, and computes quantiles over that window; don't average its quantiles across instances.

## Timing computations

```vibe
// run: provide TestClock.layer() { const worker = fork(() => render()); TestClock.adjust("30 millis"); Console.log(join worker); const state = Metric.value(renderTime); Console.log(state.count, state.sum, state.min, state.max) }
import { TestClock } from "effect/testing"

const renderTime = Metric.timer("guide_render_duration", {
  boundaries: [5, 20, 50, 100, 500]
})

function render() {
  return Effect.trackDuration(renderTime) {
    sleep("30 millis")
    return "rendered"
  }
}
```

```output
rendered
1 30 30 30
```

`trackDuration` records the wrapped computation's duration, time spent waiting included, and keeps its return value. `Metric.timer` is a duration-input histogram whose bucket boundaries and stored values are milliseconds.

## Counting categorical events

```vibe
// run: Console.log(JSON.stringify(Array.from(recordOutcomes()).sort(([a], [b]) => Order.String(a, b))))
const outcomes = Metric.frequency("guide_validation_outcomes")

function recordOutcomes() {
  Metric.update(outcomes, "accepted")
  Metric.update(outcomes, "rejected")
  Metric.update(outcomes, "accepted")
  return Metric.value(outcomes).occurrences
}
```

```output
[["accepted",2],["rejected",1]]
```

A frequency metric counts each observed string. Use a bounded set of categories, such as result kinds or error tags; user and request IDs are unbounded and belong in logs or span attributes.

## Attaching attributes to metrics and regions

```vibe
// run: const result = attributed(); Console.log(result.requests.count, result.failures.count); Console.log(Metric.value(apiRequests).count, Metric.value(failures).count)
const apiRequests = Metric.withAttributes(
  Metric.counter("guide_api_requests", { incremental: true }),
  { route: "/reports" }
)
const failures = Metric.counter("guide_api_failures", { incremental: true })

function attributed() {
  return Effect.provideService(Metric.CurrentMetricAttributes, { environment: "staging" }) {
    Metric.update(apiRequests, 1)
    Metric.update(failures, 1)
    return {
      requests: Metric.value(apiRequests),
      failures: Metric.value(failures)
    }
  }
}
```

```output
1 1
0 0
```

`withAttributes` adds attributes to one metric. `CurrentMetricAttributes` adds attributes to every metric operation in a region, reads included, so read state under the attributes you updated it with. Each attribute combination is a separate series; prefer bounded categories such as deployment and route.
