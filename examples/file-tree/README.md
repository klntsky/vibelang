# File tree

A recursive function snapshots regular files, then a producer yields their paths.
`for...of` consumes the producer; `Effect.forEach` reads files with concurrency 4.
The recursion accumulates into an array and returns void, so its contract is inferred.
`FileSystem` and `Path` come from Effect, backed by `NodeServices.layer`.
Canonical directory paths prevent revisiting a directory through symbolic links.

Run from this directory so `fixtures` resolves to the bundled sample tree:
```sh
bun install
bun run build
bun run start
```

The ordered output counts lines in the two fixture files.
The concurrent read in Effect TypeScript is:
```ts
const lengths = Effect.forEach(files, (path) => Effect.gen(function* () {
  const fs = yield* FileSystem
  const paths = yield* Path
  const text = yield* fs.readFileString(path)
  return { path: paths.relative(root, path), lines: text.trimEnd().split("\n").length }
}), { concurrency: 4 })
```

The prototype runs this program, but widens recursive E and R to `unknown`.
