# Logging

VibeLang uses Effect's structured logging to record progress, failures, and diagnostic context. Log calls run where they appear, and the current logging configuration determines which records are emitted and where they go.

## Emitting structured records

```vibe
function reportProgress() {
  log("export started", { format: "csv", stage: "prepare" })
  logInfo("export completed", { rows: 240 })
}
```

`log` uses the Info level, and a call can contain several messages or structured values. Default output includes time, severity, fiber identity, and message content. Pass a `Cause` alongside a message to retain failure detail rather than flattening it to a string.

## Selecting severity thresholds

```vibe
function diagnostics() {
  Effect.provideService(References.MinimumLogLevel, "Debug") {
    Effect.logTrace("entered parser")
    logDebug("parsed header")
    logInfo("input accepted")
    logWarning("using fallback settings")
    logError("destination unavailable")
    Effect.logFatal("application cannot start")
  }
}
```

The default threshold is Info, which filters Debug and Trace messages. Providing Debug enables debug records in this region while still filtering Trace; the surrounding program keeps its own threshold. Severity describes a record and does not itself fail or terminate the program, even for `logFatal`.

## Annotating logs and scoped regions

```vibe
function annotatedExport() {
  Effect.annotateLogs({ operation: "weekly-export", format: "csv" }) {
    logInfo("preparing")
    Effect.annotateLogs(() => logInfo("writing"), "stage", "write")
  }
  logInfo("outside annotated operation")
}

function scopedAnnotations() {
  Effect.scoped {
    Effect.annotateLogsScoped({ component: "importer" })
    logInfo("reading input")
    logInfo("validating rows")
  }
  logInfo("outside importer scope")
}
```

`annotateLogs` attaches key/value metadata to every record inside its program, including nested calls and inherited child context. `annotateLogsScoped` changes annotations until the supplied runtime scope closes. A plain pair of braces does not supply that scope, so use `Effect.scoped` for this scoped API.

## Measuring log spans

```vibe
function timedExport() {
  Effect.withLogSpan("write-output") {
    logInfo("opening output")
    sleep("30 millis")
    logInfo("output written")
  }
}
```

Each record inside the log span includes the elapsed time since that span began. The first record measures time near the start, and the second includes the intervening wait. A log span enriches logs; use tracing spans when you need parent/child relationships and exportable traces.

## Disabling and configuring logging

```vibe
const Silent = Layer.succeed(References.MinimumLogLevel, "None")
const LogLevel = Config.LogLevel("LOG_LEVEL")

const ConfiguredLevel = Layer.unwrap {
  const minimum = LogLevel()
  return Layer.succeed(References.MinimumLogLevel, minimum)
}

function quiet() {
  provide Silent { logInfo("suppressed") }
}

function configured() {
  provide ConfiguredLevel { logDebug("configuration loaded") }
}
```

The None threshold suppresses logging in the provided region. `Config.LogLevel` builds a configuration value; `LogLevel()` reads and validates the severity when the layer is built, reporting configuration errors then. Supply the configuration layer once at the application boundary; applications with several entry points can include it in a shared `ManagedRuntime`.

## Defining a custom output handler

```vibe
function captureLogs() {
  const records: Array<{ level: string; message: unknown }> = []
  const capture = Logger.make(({ logLevel, message }) => {
    records.push({ level: logLevel, message })
  })
  provide Logger.layer([capture, Logger.tracerLogger]) {
    logInfo("batch accepted", { count: 3 })
    logWarning("one retry required")
  }
  return records
}
```

`Logger.make` takes a synchronous callback that receives the message, level, cause, emitting fiber, and date. The callback above only records values in memory, so it is eligible for a plain JavaScript callback. Do not call a pausing service inside it; use an Effect-aware logging destination such as an exporter layer for asynchronous output.

## Choosing text, structured, and JSON formats

```vibe
const TextLogs = Logger.layer([Logger.consoleLogFmt, Logger.tracerLogger])
const PrettyLogs = Logger.layer([Logger.consolePretty(), Logger.tracerLogger])
const ObjectLogs = Logger.layer([Logger.consoleStructured, Logger.tracerLogger])
const JsonLogs = Logger.layer([Logger.consoleJson, Logger.tracerLogger])

function jsonExample() {
  provide JsonLogs { logInfo("started", { workers: 4 }) }
}
```

The default logger emits readable text; LogFmt offers compact key/value output and Pretty adds visual formatting. Structured output preserves an object containing messages, level, timestamp, annotations, cause, log spans, and fiber identity; JSON serializes that structure for ingestion. The corresponding formatter loggers are `formatSimple`, `formatLogFmt`, `formatStructured`, and `formatJson` when a custom handler needs formatted output rather than console delivery.

## Sending records to multiple destinations

```vibe
function consoleAndCapture() {
  const auditMessages: unknown[] = []
  const audit = Logger.make(({ message }) => { auditMessages.push(message) })
  const destinations = Logger.layer([
    Logger.defaultLogger,
    audit,
    Logger.tracerLogger
  ])
  provide destinations { logInfo("job finished") }
  return auditMessages
}
```

Each active logger receives the record after threshold filtering. `Logger.layer` replaces the active logger set by default; its `mergeWithExisting: true` option adds destinations instead. Include `tracerLogger` when logs should also become events on the current tracing span.
