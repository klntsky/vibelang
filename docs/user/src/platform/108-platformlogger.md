# PlatformLogger

File logging uses Effect's `Logger` module. Combine a string formatter with `Logger.toFile`, then install the logger with a layer whose scope owns the file and pending batches.

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

`formatLogFmt` produces string records; `toFile` creates a scoped logger that appends them to the path. The array holds a program, so the file opens when `Logger.layer` builds, not at module initialization. The layer replaces the installed loggers with its list.

In Effect 4.0.6, `toFile` batches with a one-second window by default; `batchWindow` changes it. Pending entries flush when the logger's scope closes. Opening the file can fail with `PlatformError`; later flush write failures are ignored, so no log call gets a durable-write acknowledgment.

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

`Logger.toFile` needs FileSystem and Scope: `Logger.layer` owns the scope, and `Layer.provide` supplies the file system. Reading after the provided program completes avoids racing the batch window; a successful read still doesn't prove every earlier write succeeded.

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

Install both loggers in one list: the console gets message text, the file gets logfmt. `Logger.consolePretty()` gives formatted console records with timestamps. To add to the loggers already installed, use `Logger.layer(loggers, { mergeWithExisting: true })`. Keep secrets in `Redacted`, and don't log raw request or customer payloads.

## Coming from Effect

Pass `Logger.toFile` to `Logger.layer` inside a no-argument function. Calling it earlier acquires the scoped logger at once instead of letting the layer build it.
