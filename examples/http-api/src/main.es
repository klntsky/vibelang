import { HttpRouter, HttpServerRequest, HttpServerResponse } from "effect/http"
import { ServerLive } from "./server.ts"

export const CreateUser = Schema.Struct({ name: Schema.String })
export class NameTaken extends TaggedError("NameTaken")<{ readonly name: string }> {}

export abstract class Users {
  abstract create(name: string): Effect<{ readonly id: string; readonly name: string }, NameTaken, never>
}

// The layer owns its state; each test builds an isolated directory.
export const UsersLive = Layer.effect(Users, () => {
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
})

export function createUser() {
  try {
    const input = HttpServerRequest.schemaBodyJson(CreateUser)
    const user = Users.create(input.name)
    return HttpServerResponse.json(user, { status: 201 })
  } catch (error) {
    if (error instanceof NameTaken) {
      return HttpServerResponse.text(`Name already registered: ${error.name}`, { status: 409 })
    }
    if (error._tag === "SchemaError") {
      return HttpServerResponse.text("Expected a JSON object with a string name", { status: 400 })
    }
    throw error
  }
}

export const Routes = HttpRouter.add("POST", "/users", () => createUser())
export const App = HttpRouter.provideRequest(UsersLive)(Routes)

const Live = Layer.provide(HttpRouter.serve(App), ServerLive)

export function main() {
  Layer.launch(Live)
}
