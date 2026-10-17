# Approval handlers

One release program asks a control operation for staging and production approval.
The auto-approve handler resumes both operations with true. The scripted reviewer
uses supplied answers and defaults to denial when the script runs out.
The business function depends on `Approval`, without choosing a handler.

From this directory, with `effeck` on `PATH`:
```sh
bun install
bun run build
bun run start
```

Features: a `control class` declaration, handle statements, and tail `resume`.
Effect alone does not provide delimited handlers. Its key control call, expressed
as Effect TypeScript against Effeck's emitted Approval key and runtime, is:
```ts
const release = Effect.fnUntraced(function* (version: string) {
  const approval = yield* Approval
  if (!(yield* approval.approve(`deploy ${version} to staging`))) return "staging declined"
  if (!(yield* approval.approve(`deploy ${version} to production`))) return "production declined"
  return `released ${version}`
})
```

The runtime owns each one-shot continuation; each clause resumes it once.
The fixture shows the same program succeeding automatically and declining production.
