# Path

`Path` provides Effect's platform-specific path service to Effeck. Use it to join, split, normalize, and convert paths without hard-coding host separators or importing a native path module.

## Requesting the path service

```ts
function pathSeparator() {
  const path = Effect.service(Path.Path)
  return path.sep
}
// Effect<string, never, Path.Path>
```

`Path` is the module and `Path.Path` is its service key. `Effect.service` obtains the implementation from the current environment. Reading `sep` returns that implementation's path separator.

## Manipulating paths portably

```ts
function describeReport() {
  const path = Effect.service(Path.Path)
  const joined = path.join("reports", "weekly.txt")
  const normalized = path.normalize("reports/./draft/../weekly.txt")
  const parsed = path.parse(joined)
  return {
    joined,
    normalized,
    base: path.basename(joined),
    directory: path.dirname(joined),
    extension: path.extname(joined),
    absolute: path.isAbsolute(joined),
    rebuilt: path.format(parsed)
  }
}
```

Most `Path` methods are plain synchronous TypeScript functions. Effect's own packages are trusted, so call these methods directly; any exceptions are defects, which ordinary `catch` does not handle. Calling a member through the key, such as `Path.Path.join("reports", "weekly.txt")`, looks up the service and adds `Path.Path` to R even though the member returns a string.

| Operation | Result |
| --- | --- |
| `basename`, `dirname`, `extname` | File name, directory, or extension |
| `join`, `normalize` | A combined or simplified path |
| `parse`, `format` | Structured path components or their string form |
| `isAbsolute` | Whether a path has an absolute root |
| `relative`, `resolve` | A relative relationship or an absolute resolution |
| `sep` | The implementation's separator |
| `toNamespacedPath` | A Windows namespace path where applicable |
| `toFileUrl`, `fromFileUrl` | Effectful file-URL conversion |

```ts
function asFileUrl(file: string) {
  const path = Effect.service(Path.Path)
  const absolute = path.resolve(file)
  const url = path.toFileUrl(absolute)
  return path.fromFileUrl(url)
}
```

URL conversions return Effects and run directly when called, reporting `BadArgument` on invalid input. Path string operations are lexical: normalizing or resolving a path does not check that a file exists. Use FileSystem when filesystem state matters.

## Supplying a platform path implementation

```ts
// main.effeck
import { NodeRuntime, NodeServices } from "@effect/platform-node"

function main() {
  const output = Path.Path.join("reports", "weekly.txt")
  Effect.log(output)
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeServices.layer))
```

The Node service bundle supplies host path behavior. `Path.layer` supplies the built-in POSIX implementation for programs that intentionally use POSIX paths, including deterministic tests. Use Bun's service bundle when the rest of the application runs against Bun platform services.

## Coming from Effect

Effect-returning methods run directly; synchronous methods from Effect's trusted packages return plain values. Both forms look up the service when called through its key, so both add the service to R. Value members are read from the implementation, for example `Path.Path().sep`.
