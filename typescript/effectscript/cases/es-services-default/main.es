// A service with `static readonly default` is a Context.Reference (D40):
// code that uses it needs no provider, so it never joins R, and
// Effect.provideService overrides it for a region. Static members such as
// make and layer stay on the class.
import { Effect, Layer } from "effect"

interface Session {
  readonly user: string
}

abstract class Sessions {
  abstract get(token: string): Effect.Effect<Session | undefined, never, never>
  abstract set(token: string, session: Session): Effect.Effect<void, never, never>

  static make() {
    const entries = new Map<string, Session>()
    const impl: Sessions = {
      get: (token: string) => Effect.succeed(entries.get(token)),
      set: (token: string, session: Session) => Effect.sync(() => { entries.set(token, session) })
    }
    return impl
  }

  static readonly layer = Layer.sync(Sessions, () => Sessions.make())
}

abstract class FeatureFlags {
  abstract readonly newCheckout: boolean
  static readonly default: FeatureFlags = { newCheckout: false }
}

export function checkoutPage(cartId: string) {
  return FeatureFlags().newCheckout ? `new ${cartId}` : `classic ${cartId}`
}

export function login(user: string) {
  Sessions.set("t1", { user })
  const session = Sessions.get("t1")
  return session?.user ?? "nobody"
}

const classic = checkoutPage("c1")
const fresh = Effect.provideService(checkoutPage("c2"), FeatureFlags, { newCheckout: true })
const session = Effect.provide(login("ada"), Sessions.layer)

// expect-result: {"classic":"classic c1","fresh":"new c2","session":"ada"}
export function main() {
  return { classic: classic(), fresh: fresh(), session: session() }
}
