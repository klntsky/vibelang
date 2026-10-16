// TypeScript-authored Context.Service classes and Effect's own services are
// used like .es services (D41): a member that is an Effect value runs when
// called, a member the service declares wins over the key's own pipe, and a
// member that returns a plain value still looks the service up.
import { Effect, Layer } from "effect"
import { layer as PathLayer, Path } from "effect/Path"
import { UserRepo, UserRepoLive } from "./repo.ts"

export function describe(id: string) {
  const user = UserRepo.byId(id)
  const everyone = UserRepo.all()
  return `${user.name} of ${everyone.length}`
}

export function piped() {
  return UserRepo.pipe("label")
}

export function joined(a: string, b: string) {
  return Path.join(a, b)
}

// A lambda whose body only looks a service up is an Effect function too.
const underRoot = (part: string) => Path.join("root", part)

const program = Effect.provide(Effect.all([describe("2"), piped(), joined("a", "b"), underRoot("c")]), Layer.mergeAll(UserRepoLive, PathLayer))

// expect-result: ["Grace of 2","piped label","a/b","root/c"]
export function main() {
  return program()
}
