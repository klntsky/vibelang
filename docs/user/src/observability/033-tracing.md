# Tracing in Effeck

Tracing records an operation's duration and its relationship to other operations. Effeck uses Effect's spans, attributes, and exporters, so traces can connect work across services. Functions are untraced by default; add spans around operations that you want to observe.

## Relating spans to traces

```ts
function requestTrace() {
  return withSpan(() => {
    sleep("10 millis")
    return "response"
  }, "report-request")
}
```

A span describes one operation with a name, identifiers, timestamps, status, and attributes. A trace groups related spans under one trace ID, usually beginning with a request's root span. Parent span IDs connect the operations into a tree that a trace viewer displays as a waterfall.

## Adding spans around computations

```ts
function renderReport(format: string) {
  return withSpan(() => {
    sleep("20 millis")
    return `report.${format}`
  }, "render-report", { attributes: { "report.format": format } })
}
// string!!
```

`withSpan` starts the span before the program runs and ends it when that program exits. It preserves the success and failure behavior of the program. A stable operation name groups comparable work; put variable details into attributes instead of inventing a new span name per request.

## Exporting spans and SDK boundaries

```ts
import * as FetchHttpClient from "effect/http/FetchHttpClient"
import * as OtlpSerialization from "effect/observability/OtlpSerialization"
import * as OtlpTracer from "effect/observability/OtlpTracer"

const Tracing = pipe(
  OtlpTracer.layer({
    url: "http://localhost:4318/v1/traces",
    resource: { serviceName: "report-worker" },
    exportInterval: "1 second"
  }),
  Layer.provide(OtlpSerialization.layerJson),
  Layer.provide(FetchHttpClient.layer)
)

function exportedReport() {
  with Tracing { return renderReport("csv") }
}
```

This source version provides direct OTLP/HTTP export under `effect/observability`. The layer batches ended spans, uses the supplied HTTP client and serialization, and flushes during scope shutdown. Provide it around the application's lifetime so each request does not create and close its own exporter.

An application that already owns an OpenTelemetry SDK tracer provider can initialize its compatible bridge in a TypeScript bootstrap or adapter and provide the resulting Effect layer to Effeck. Effect's own packages are trusted; a plain third-party SDK call inside an Effeck function needs an explicit boundary such as `try`, or a TypeScript declaration tagged `/** @effeck trusted */`. A synchronous SDK context callback must compile to plain JavaScript, so it cannot call services or perform other Effect operations.

## Inspecting failures and attributes

```ts
error RenderFailed {
  format: string
}

function failingRender() {
  const exit = Effect.exit(withSpan(() => {
    Effect.annotateCurrentSpan("report.format", "unknown")
    throw new RenderFailed({ format: "unknown" })
  }, "render-report"))
  if (exit.isFailure()) logError("render failed", exit.cause)
  return exit
}
```

An exported failure includes error status and failure details, while attributes identify the operation's context. `annotateCurrentSpan` enriches the active span from inside its body. Recording an exit as data lets a diagnostic boundary inspect failure, defects, and interruption without changing the span's already recorded outcome.

## Representing logs as span events

```ts
function tracedLogs() {
  Effect.provide(() => withSpan(() => {
    logInfo("input validated")
    sleep("10 millis")
    logInfo("artifact written")
  }, "generate-artifact"), Logger.layer([
    Logger.defaultLogger,
    Logger.tracerLogger
  ]))
}
```

The tracer logger records logs within an active span as timestamped events. Those events provide an internal timeline alongside the span's start and finish. Replacing the logger set without retaining `tracerLogger` stops this event forwarding.

## Nesting operations

```ts
function loadTemplate() {
  return withSpan(() => {
    sleep("10 millis")
    return "weekly-template"
  }, "load-template")
}

function generate() {
  return withSpan(() => {
    const template = loadTemplate()
    const output = renderReport("csv")
    return { template, output }
  }, "generate-report")
}
```

The inner spans inherit the outer span as parent. Their trace IDs match, while each span has its own ID and duration. Child fibers inherit the tracing context too, allowing concurrent operations to appear as siblings in the same trace.

## Viewing traces locally

```ts
function demonstration() {
  with Tracing { generate() }
}
```

Run an OTLP/HTTP collector and a trace viewer on the endpoints configured above. For a local Grafana LGTM container, run `docker run --rm --name effeck-telemetry --publish 3000:3000 --publish 4318:4318 grafana/otel-lgtm`. It supplies a collector and Tempo for trace storage, with Grafana for inspection. Run `demonstration` through the application runner, let the exporter close cleanly, then open `http://localhost:3000/explore` and search Tempo for service `report-worker`.

Open a `generate-report` trace to compare parent duration, child durations, attributes, events, and failure status. If nothing arrives, check the trace URL, HTTP transport, serialization, and exporter diagnostics before adding more instrumentation.

## Connecting external tracing services

```ts
import * as FetchHttpClient from "effect/http/FetchHttpClient"
import * as OtlpSerialization from "effect/observability/OtlpSerialization"
import * as OtlpTracer from "effect/observability/OtlpTracer"

const TraceEndpoint = Config.String("TRACE_ENDPOINT")

const VendorTracing = Layer.unwrap(() => {
  const url = TraceEndpoint()
  return pipe(
    OtlpTracer.layer({ url, resource: { serviceName: "report-worker" } }),
    Layer.provide(OtlpSerialization.layerJson),
    Layer.provide(FetchHttpClient.layer)
  )
})

function productionReport() {
  with VendorTracing { return generate() }
}
```

`Config.String` builds a configuration value; `TraceEndpoint()` reads it when the layer is built. Point the exporter at an authenticated collector or a service's documented OTLP traces endpoint, and configure required headers through `OtlpTracer.layer` when needed. A collector can route these spans to a hosted tracing backend. For Sentry or another service that already installs its own SDK, use that service's compatible SDK bridge or documented OTLP route; exporting successful spans does not also create error-monitoring issues.

## Coming from Effect

`withSpan` still supplies tracing explicitly, but its program is an ordinary function. Do not add a span to every helper merely to reproduce function boundaries; instrument the operations you need to diagnose.
