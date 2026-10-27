# Path

`Path` is Effect's platform-specific path service, in place of `node:path`. Use it to join, split, normalize, and convert paths without hard-coding host separators or importing a native path module.

## Requesting the path service

```vibe
// run: provide NodePath.layer { Console.log(pathSeparator()) }
import { NodePath } from "@effect/platform-node"
import { Path } from "effect/Path"

function pathSeparator() {
  return Path().sep
}
// Output: /
```

`import { Path } from "effect/Path"` imports the service key. `Path()` returns the implementation from the current environment, whose `sep` is a value, and `Path.join(...)` calls one of its methods. Without the import, `Path` is the prelude's module, whose service key is `Path.Path`.

## Manipulating paths portably

```vibe
// run: provide NodePath.layer { Console.log(JSON.stringify(describeReport())) }
import { NodePath } from "@effect/platform-node"
import { Path } from "effect/Path"

function describeReport() {
  const joined = Path.join("reports", "weekly.txt")
  const normalized = Path.normalize("reports/./draft/../weekly.txt")
  const parsed = Path.parse(joined)
  return {
    joined,
    normalized,
    base: Path.basename(joined),
    directory: Path.dirname(joined),
    extension: Path.extname(joined),
    absolute: Path.isAbsolute(joined),
    rebuilt: Path.format(parsed)
  }
}
// Output: {"joined":"reports/weekly.txt","normalized":"reports/weekly.txt","base":"weekly.txt","directory":"reports","extension":".txt","absolute":false,"rebuilt":"reports/weekly.txt"}
```

Most `Path` methods are plain synchronous TypeScript functions. Effect's own packages are trusted, so call them directly; an exception escaping into Effect code is a defect, which Effect catch doesn't handle. A plain function's catch doesn't see it either (D282); recover it with the `try` hatch. Calling a member through the key, as in `Path.join("reports", "weekly.txt")`, looks up the service and adds `Path` to R even though the member returns a string.

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
// run: provide NodePath.layer { Console.log(asFileUrl("/reports/weekly.txt")) }
import { NodePath } from "@effect/platform-node"
import { Path } from "effect/Path"

function asFileUrl(file: string) {
  const absolute = Path.resolve(file)
  const url = Path.toFileUrl(absolute)
  return Path.fromFileUrl(url)
}
// Output: /reports/weekly.txt
```

URL conversions return Effects, run when called, and report `BadArgument` on invalid input. Path string operations are lexical: normalizing or resolving a path doesn't check that a file exists. Use FileSystem when file-system state matters.

## Supplying a platform path implementation

```vibe
// main.vibe
import "vibelang/platform/node"
import { Path } from "effect/Path"

export function main() {
  Console.log(Path.join("reports", "weekly.txt"))
}
// Output: reports/weekly.txt
```

`import "vibelang/platform/node"` makes the entry provide the Node service bundle, which supplies host path behavior, to `main` (D312); `"vibelang/platform/bun"` provides Bun's, `BunServices.layer`. `layer` from `effect/Path`, the prelude's `Path.layer`, supplies the built-in POSIX implementation, for programs that use POSIX paths on purpose, deterministic tests included: `provide Path.layer { ... }` inside `main` wins over the bundle.

## Coming from Effect

Effect-returning methods run directly; synchronous methods from Effect's trusted packages return plain values. Called through the key, both look up the service and add it to R. Read value members from the implementation, as in `Path().sep`.
