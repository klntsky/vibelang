# Building an HTTP API

Build routes with Effect's HTTP modules and write each handler as an Effeck function. This example creates users, validates JSON bodies, and maps expected failures to HTTP status codes.

## Define the body and the service

```ts
// users.effeck
export schema CreateUser { name: string }
export error NameTaken { name: string }

export service class Users {
  create(name: string): { id: string; name: string }!NameTaken
}

export const UsersLive = Layer.effect(Users) {
  const names = new Set<string>()
  let nextId = 0
  return {
    create(name) {
      if (names.has(name)) throw new NameTaken({ name })
      names.add(name)
      nextId++
      return { id: String(nextId), name }
    }
  }
}
```

`schema CreateUser { name: string }` declares a struct schema and its type: it accepts a JSON object with a string `name`. `Users.create` declares `NameTaken` as an expected failure. The layer owns the set of names and the ID counter, so each layer build starts with isolated state.

## Decode the request and map failures

```ts
// routes.effeck
import { HttpRouter, HttpServerRequest, HttpServerResponse } from "effect/http"
import { CreateUser, NameTaken, Users, UsersLive } from "./users.effeck"

export function createUser() {
  try {
    const input = HttpServerRequest.schemaBodyJson(CreateUser)
    const user = Users.create(input.name)
    return HttpServerResponse.json(user, { status: 201 })
  } catch (error: NameTaken) {
    return HttpServerResponse.text(`Name already registered: ${error.name}`, { status: 409 })
  } catch (error: Schema.SchemaError) {
    return HttpServerResponse.text("Expected a JSON object with a string name", { status: 400 })
  }
}

export const Routes = HttpRouter.add("POST", "/users", createUser())
export const App = HttpRouter.provideRequest(UsersLive)(Routes)
```

The JSON decoder runs at the call and returns the validated body. `Users.create` runs next, and the response is JSON with status 201. A repeated name becomes 409; a body that fails the schema becomes 400. Each typed catch clause handles the failures with its type's `_tag`, so a failure that neither names propagates to the HTTP boundary.

`catch` sees expected failures only. It does not catch defects or interruption. Keep error-to-status mapping explicit so a programming defect cannot become a successful response.

`HttpRouter.add` receives `createUser()` by name, as a program that runs on each request. `HttpRouter.provideRequest` supplies `Users` to route execution. Calling `HttpRouter.toHttpEffect` in a test will therefore use the same service implementation as the server.

## Start the Node server

```ts
// server.effeck
import { HttpRouter } from "effect/http"
import { NodeHttpServer } from "@effect/platform-node"
import { createServer } from "node:http"
import { App } from "./routes.effeck"

const Server = NodeHttpServer.layer(createServer, { port: 3000 })
const Live = Layer.provide(HttpRouter.serve(App), Server)

export function main() {
  Layer.launch(Live)
}
```

Pass `createServer` as a value. `NodeHttpServer.layer` runs the Node API and owns its lifetime; calling `createServer()` directly in `.effeck` is a platform API error. `HttpRouter.serve` builds the serving layer, and `Layer.launch` keeps it alive until shutdown.

The module top level constructs schemas, routes, and layers. `node server.js` starts the application: a module that exports a `main` without parameters ends with `if (import.meta.main) NodeRuntime.runMain(main())`, so importing it, from a test or another module, starts nothing.

## Test a route without a socket

```ts
// routes.test.effeck
import { assert, it } from "@effect/vitest"
import { HttpClientRequest, HttpRouter, HttpServerRequest } from "effect/http"
import { App } from "./routes.effeck"

function request(body: unknown) {
  const client = HttpClientRequest.bodyJson(HttpClientRequest.post("http://local/users"), body)
  return HttpServerRequest.fromClientRequest(client)
}

it.effect("validates bodies and rejects duplicate names") {
  Effect.scoped {
    const app = HttpRouter.toHttpEffect(App)
    const created = Effect.provideService(app, HttpServerRequest.HttpServerRequest, request({ name: "Ada" }))
    const conflict = Effect.provideService(app, HttpServerRequest.HttpServerRequest, request({ name: "Ada" }))
    const bad = Effect.provideService(app, HttpServerRequest.HttpServerRequest, request({ name: 123 }))
    assert.deepStrictEqual([created.status, conflict.status, bad.status], [201, 409, 400])
  }
}
```

`HttpRouter.toHttpEffect` builds a request program without starting a server. Hold that program as `app`, then provide a request for each execution. The scope shares the service layer across these requests and releases it after the test. A new test scope builds a fresh directory.

Import assertions from `@effect/vitest`; the compiler trusts its re-exports. Use the [installation guide](../getting-started/004-installation.md) to set up the compiler and test plugin. The complete project, including its Vite configuration, is in `examples/http-api`.
