# Path

`Path` is Effect's platform-specific path service. Use it to join, split, normalize, and convert paths without hard-coding host separators or importing a native path module.

## Requesting the path service

```vibe
// run: provide Path.layer { Console.log(pathSeparator()) }
function pathSeparator() {
  const path = Effect.service(Path.Path)
  return path.sep
}
// string!!Path.Path
// Output: /
```

`Path` is the module and `Path.Path` its service key. `Effect.service` gets the implementation from the current environment.

## Manipulating paths portably

```vibe
// run: provide Path.layer { Console.log(JSON.stringify(describeReport())) }
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
// Output: {"joined":"reports/weekly.txt","normalized":"reports/weekly.txt","base":"weekly.txt","directory":"reports","extension":".txt","absolute":false,"rebuilt":"reports/weekly.txt"}
```

Most `Path` methods are plain synchronous TypeScript functions. Effect's own packages are trusted, so call them directly; an exception escaping into Effect code is a defect, which Effect catch doesn't handle. A plain-contract function keeps JavaScript catch and recovers the exception as `unknown` (D260). Calling a member through the key, as in `Path.Path.join("reports", "weekly.txt")`, looks up the service and adds `Path.Path` to R even though the member returns a string.

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

```vibe
// run: provide Path.layer { Console.log(asFileUrl("/reports/weekly.txt")) }
function asFileUrl(file: string) {
  const path = Effect.service(Path.Path)
  const absolute = path.resolve(file)
  const url = path.toFileUrl(absolute)
  return path.fromFileUrl(url)
}
// Output: /reports/weekly.txt
```

URL conversions return Effects, run when called, and report `BadArgument` on invalid input. Path string operations are lexical: normalizing or resolving a path doesn't check that a file exists. Use FileSystem when file-system state matters.

## Supplying a platform path implementation

```vibe
// main.vibe
import { NodeRuntime, NodeServices } from "@effect/platform-node"

function main() {
  const output = Path.Path.join("reports", "weekly.txt")
  Console.log(output)
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeServices.layer))
// Output: reports/weekly.txt
```

The Node service bundle supplies host path behavior. `Path.layer` supplies the built-in POSIX implementation, for programs that use POSIX paths on purpose, deterministic tests included. Use Bun's service bundle when the rest of the application runs on Bun platform services.

## Coming from Effect

Effect-returning methods run directly; synchronous methods from Effect's trusted packages return plain values. Called through the key, both look up the service and add it to R. Read value members from the implementation, as in `Path.Path().sep`.
