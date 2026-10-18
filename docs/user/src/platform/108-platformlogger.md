# PlatformLogger

File logging uses Effect's `Logger` module in VibeLang. Combine a string formatter with `Logger.toFile`, then install the resulting logger with a layer whose scope owns the file and pending batches.

## Writing formatted logs to a file

```vibe
// run: provide NodeFileSystem.layer { provide FileLogger { application() }; const text = FileSystem.FileSystem.readFileString("./application.log"); Console.log(text.includes("Index build started") && text.indexOf("Index build started") < text.indexOf("Index build completed")) }
import { NodeFileSystem } from "@effect/platform-node"
const FileLogger = Logger.layer([
  () => Logger.toFile(Logger.formatLogFmt, "./application.log", {
    batchWindow: "250 millis"
  })
])

function application() {
  log("Index build started")
  log("Index build completed")
}
// Output: true
```

`formatLogFmt` produces string log records, and `toFile` creates a scoped logger that appends those records to the chosen path. The array contains a program, so the file opens when `Logger.layer` builds, not at module initialization. The layer replaces the installed loggers with its supplied list.

In Effect 4.0.5, `toFile` batches by default with a one-second window; `batchWindow` changes that interval. Pending entries flush when the logger's scope closes. Opening the file can fail with `PlatformError`; this implementation ignores later flush write failures, so it does not provide a durable-write acknowledgment for each log call.

## Supplying file-system dependencies

```vibe
// main.vibe
import { NodeFileSystem, NodeRuntime } from "@effect/platform-node"
const LoggerLive = Layer.provide(
  Logger.layer([
    () => Logger.toFile(Logger.formatLogFmt, "./application.log")
  ]),
  NodeFileSystem.layer
)

function main() {
  provide LoggerLive {
    log("Report generated")
  }
  // LoggerLive's scope has closed and its final batch has flushed.
  const fs = Effect.service(FileSystem.FileSystem)
  const text = fs.readFileString("./application.log")
  Console.log(text.includes("Report generated"))
  return text
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeFileSystem.layer))
// Output: true
```

`Logger.toFile` needs FileSystem and Scope. `Logger.layer` owns the logger's scope, and `Layer.provide` supplies its file-system dependency. Reading after the provided program completes avoids racing the batch window; a successful read still does not establish that every earlier write succeeded.

## Combining file and console output

```vibe
// main.vibe
import { NodeFileSystem, NodeRuntime } from "@effect/platform-node"
const LoggerLive = Layer.provide(
  Logger.layer([
    Logger.withConsoleLog(Logger.make(({ message }) => String(message))),
    () => Logger.toFile(Logger.formatLogFmt, "./application.log", {
      batchWindow: "500 millis"
    })
  ]),
  NodeFileSystem.layer
)

function main() {
  log("Worker ready")
}

NodeRuntime.runMain(() => Effect.provide(() => main(), LoggerLive))
// Output: Worker ready
```

Use `Logger.consolePretty()` for formatted console records with timestamps. Include both loggers in the installed list: the console gets message text, while the file gets logfmt output. For a layer that adds to whatever loggers are already installed, use `Logger.layer(loggers, { mergeWithExisting: true })`. Keep secrets wrapped in `Redacted` and avoid logging raw request or customer payloads.

## Coming from Effect

Wrap `Logger.toFile` in a no-argument function when handing it to `Logger.layer`. Calling it earlier would acquire the scoped logger immediately instead of letting the layer build it.
