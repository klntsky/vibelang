# FileSystem

`FileSystem` exposes Effect's portable file operations in Effeck. Request the service inside a function, call its operations, and provide a runtime implementation or test layer around the application.

## Requesting the file-system service

```ts
function readSettings() {
  const fs = Effect.service(FileSystem.FileSystem)
  return fs.readFileString("./settings.json")
}
// string!PlatformError.PlatformError!FileSystem.FileSystem
```

`FileSystem` is the imported module; `FileSystem.FileSystem` is its service key. `Effect.service` retrieves the implementation and adds that service to R. `readFileString` returns text on success and reports a `PlatformError` as an expected failure.

## Performing file operations

```ts
function saveSummary(text: string) {
  const fs = Effect.service(FileSystem.FileSystem)
  fs.makeDirectory("./output", { recursive: true })
  fs.writeFileString("./output/summary.txt", text)
  const content = fs.readFileString("./output/summary.txt", "utf8")
  return { content, entries: fs.readDirectory("./output") }
}
```

Calls run sequentially: the write completes before the read starts. Expected I/O failures enter E and can be handled with `try`/`catch`. `readFile` and `writeFile` handle byte arrays when text encoding is not appropriate.

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

```ts
function fileSize(path: string) {
  const fs = Effect.service(FileSystem.FileSystem)
  using file = fs.open(path)
  return file.stat().size
}
```

`open` requires a Scope and registers closing automatically. `using` makes that acquisition belong to the enclosing block, so the handle closes when the function exits, including on failure or interruption. Without `using`, the acquisition keeps Scope in R and belongs to the caller's scope.

## Supplying platform implementations

```ts
// main.effeck
import { NodeFileSystem, NodeRuntime } from "@effect/platform-node"
function main() {
  const fs = Effect.service(FileSystem.FileSystem)
  Effect.log(fs.readFileString("./README.md"))
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeFileSystem.layer))
```

`NodeFileSystem.layer` supplies just the file-system implementation. `NodeServices.layer` supplies the broader Node platform bundle, and Bun applications use the corresponding Bun adapter. Application functions need only `FileSystem`, not a concrete adapter package.

## Replacing behavior in tests

```ts
import { it } from "@effect/vitest"
function readFixture() {
  const fs = Effect.service(FileSystem.FileSystem)
  if (!fs.exists("/fixtures/title.txt")) return "Untitled"
  return fs.readFileString("/fixtures/title.txt")
}

const FilesTest = FileSystem.layerNoop({
  exists: (path) => path === "/fixtures/title.txt",
  readFileString: () => "Release notes"
})

it.effect("reads the configured title", () => {
  const title = Effect.provide(() => readFixture(), FilesTest)
  if (title !== "Release notes") throw new Error("Unexpected title")
})
```

`layerNoop` supplies a complete service with selected method overrides. Its defaults fail or die for unsupported operations; it is not an in-memory file system. Override every operation the test exercises and let unexpected access expose a missing test behavior.

## Coming from Effect

Test overrides use ordinary Effeck functions. Contextual typing converts them to the Effect-returning methods the service interface expects, so `readFileString: () => "Release notes"` replaces a TypeScript `Effect.succeed` body.
