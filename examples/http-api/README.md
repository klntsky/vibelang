# HTTP users API

An Effect 4 HTTP router validates `POST /users` with `Schema.Struct` and
calls an abstract `Users` service. It returns 201, maps `NameTaken` to 409,
and maps schema failures to 400. Other failures retain their typed channel.
`HttpRouter.provideRequest` supplies request services from a shared layer.

From this directory, with `effeck` on `PATH`:
```sh
bun install
bun run build
bun run start
# In another terminal:
curl -i localhost:3000/users -H 'content-type: application/json' -d '{"name":"Ada"}'
bun run test
```

Vitest runs `src/main.test.effeck` through `effeck-vite` without a prior
build. Assertion stacks point to `.effeck` lines. The test routes requests through
`HttpRouter.toHttpEffect`, without a socket.
Importing `main.js` constructs layers; only `run.js` starts the server.
`main.effeck` passes Node's `createServer` to `NodeHttpServer.layer` as a value,
which runs nothing; a call to a platform API from `.effeck` code is EK4402.
The successful request path in Effect TypeScript is:
```ts
const createUser = Effect.fnUntraced(function* () {
  const input = yield* HttpServerRequest.schemaBodyJson(CreateUser)
  const users = yield* Users
  const user = yield* users.create(input.name)
  return yield* HttpServerResponse.json(user, { status: 201 })
})
```
The HTTP server runs until interrupted and has no output fixture.
