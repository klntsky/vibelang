# Users and layers

A directory-backed greeting uses abstract services, inferred contracts,
`TaggedError`, `throw`, typed `try`/`catch`, layers, and `wire`.
The directory is captured when `UsersLive` builds, so lookups need only `Users`.
Tests replace the live graph with `UsersTest` and check missing-user recovery.

From this directory, with `effeck` on `PATH`:
```sh
bun install
bun run build
bun run start
bun run test
```

`src/main.test.effeck` uses `it.effect`; Vitest runs the source
through `@effeck/vite`, with assertion stacks mapped to `.effeck` lines.
Assertions sit inside `Effect.sync`, an explicit boundary for foreign code.
The lookup in Effect TypeScript is:
```ts
const find = Effect.fnUntraced(function* (id: string) {
  const user = yield* directory.lookup(id)
  if (user === undefined) return yield* Effect.fail(new UserNotFound({ id }))
  return user
})
```

The CLI prints a welcome for Ada and a fallback for the missing user.
