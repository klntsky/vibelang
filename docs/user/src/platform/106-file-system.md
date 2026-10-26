# FileSystem

`FileSystem` exposes Effect's portable file operations, in place of `node:fs`. Import the service, call its operations, and provide a runtime implementation or test layer around the application.

## Requesting the file-system service

```vibe
// run: provide layerNoop({ readFileString: () => '{"theme":"dark"}' }) { Console.log(readSettings()) }
import { FileSystem, layerNoop } from "effect/FileSystem"

function readSettings() {
  return FileSystem.readFileString("./settings.json")
}
// Output: {"theme":"dark"}
```

`import { FileSystem } from "effect/FileSystem"` imports the service key, and `FileSystem.readFileString(path)` calls its operation, adding `FileSystem` to R. `readFileString` returns text or fails with an expected `PlatformError`. Without the import, `FileSystem` is the prelude's module, whose service key is `FileSystem.FileSystem`; `Effect.service(FileSystem.FileSystem)` returns the implementation.

## Performing file operations

```vibe
// run: provide NodeFileSystem.layer { Console.log(JSON.stringify(saveSummary("ready"))) }
import { NodeFileSystem } from "@effect/platform-node"
import { FileSystem } from "effect/FileSystem"

function saveSummary(text: string) {
  FileSystem.makeDirectory("./output", { recursive: true })
  FileSystem.writeFileString("./output/summary.txt", text)
  const content = FileSystem.readFileString("./output/summary.txt", "utf8")
  return { content, entries: FileSystem.readDirectory("./output") }
}
// Output: {"content":"ready","entries":["summary.txt"]}
```

Calls run in order: the write completes before the read starts. Expected I/O failures enter E, and `try`/`catch` handles them. `readFile` and `writeFile` take byte arrays where text encoding doesn't fit.

| Operations | Purpose |
| --- | --- |
| `access`, `exists` | Check accessibility or presence |
| `copy`, `copyFile` | Copy a tree or a single file |
| `chmod`, `chown` | Change permissions or ownership |
| `link`, `symlink`, `readLink` | Work with hard and symbolic links |
| `makeDirectory`, `readDirectory` | Create and list directories |
| `makeTempDirectory`, `makeTempFile` | Create temporary paths owned by the caller |
| `makeTempDirectoryScoped`, `makeTempFileScoped` | Create temporary paths with scope-managed removal |
| `open` | Open a scope-managed file handle |
| `readFile`, `readFileString` | Read bytes or text |
| `writeFile`, `writeFileString` | Write bytes or text |
| `stat`, `realPath` | Inspect metadata or resolve a canonical path |
| `remove`, `rename`, `truncate`, `utimes` | Remove, move, resize, or change timestamps |
| `stream`, `sink`, `watch` | Stream bytes, write through a sink, or observe changes |

```vibe
// run: provide NodeFileSystem.layer { using path = FileSystem.makeTempFileScoped(); FileSystem.writeFileString(path, "hello"); Console.log(String(fileSize(path))) }
import { NodeFileSystem } from "@effect/platform-node"
import { FileSystem } from "effect/FileSystem"

function fileSize(path: string) {
  using file = FileSystem.open(path)
  return file.stat().size
}
// Output: 5
```

`open` requires a Scope and registers the close itself. `using` gives the acquisition to the enclosing block, so the handle closes when the function exits, on failure or interruption too. Without `using`, the acquisition keeps Scope in R and belongs to the caller's scope.

## Supplying platform implementations

```vibe
// main.vibe
import { NodeFileSystem } from "@effect/platform-node"
import { FileSystem } from "effect/FileSystem"

export function main() {
  provide NodeFileSystem.layer {
    using path = FileSystem.makeTempFileScoped()
    FileSystem.writeFileString(path, "Project notes")
    Console.log(FileSystem.readFileString(path))
  }
}
// Output: Project notes
```

`NodeFileSystem.layer` supplies only the file system; `NodeServices.layer` supplies the whole Node platform bundle, and `BunServices.layer` from `@effect/platform-bun` Bun's. Application functions need only `FileSystem`, not an adapter package.

## Replacing behavior in tests

```vibe
// run: provide FilesTest { Console.log(readFixture()) }
import { FileSystem, layerNoop } from "effect/FileSystem"

function readFixture() {
  if (!FileSystem.exists("/fixtures/title.txt")) return "Untitled"
  return FileSystem.readFileString("/fixtures/title.txt")
}

const FilesTest = layerNoop({
  exists: (path) => path === "/fixtures/title.txt",
  readFileString: () => "Release notes"
})
// Output: Release notes
```

```vibe
// files.test.vibe
import { it } from "@effect/vitest"

it.effect("reads the configured title") {
  const title = Effect.provide(() => readFixture(), FilesTest)
  if (title !== "Release notes") throw new Error("Unexpected title")
}
// Output:
```

`layerNoop` supplies a complete service with the methods you override. Its other methods fail or die; it is not an in-memory file system. Override every operation the test exercises, so unexpected access exposes a missing test behavior.

## Coming from Effect

Test overrides are ordinary functions; contextual typing converts them to the Effect-returning methods the service interface expects, so `readFileString: () => "Release notes"` replaces a TypeScript `Effect.succeed` body.

