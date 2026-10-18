import { Context, Layer } from "effect"

/**
 * A layer that provides one service, for a provider written `Key = value` in a
 * provide block (D173):
 *
 *     provide Clock = testClock, Users = fakeUsers { app() }
 *
 * compiles to `Effect.provide(..., [$provideService(Clock, "Clock", testClock),
 * $provideService(Users, "Users", fakeUsers)])`. The key is a service key, such
 * as a `service` declaration, or a module whose export of the same name is
 * one, so the effect module `Clock` stands for `Clock.Clock`.
 *
 * One signature, with the name before the value, so TypeScript knows the key
 * and the name when it first checks the value. With an overload per form, it
 * checked the value against the first overload's shape, and a method in the
 * value took the form that shape gave it (D92) for every later overload.
 */
export function $provideService<const N extends string, K extends Context.Key<any, any> | { readonly [P in N]: Context.Key<any, any> }>(
  key: K,
  name: N,
  value: NoInfer<Context.Service.Shape<ProvidedKey<K, N>>>
): Layer.Layer<Context.Service.Identifier<ProvidedKey<K, N>>>
export function $provideService(key: any, name: string, value: unknown): Layer.Layer<any> {
  const service = Context.isKey(key) ? key : key[name]
  if (!Context.isKey(service)) {
    throw new TypeError(`provide ${name} = ...: ${name} is not a service key, and it has no service key named ${name}`)
  }
  return Layer.succeed(service as Context.Key<any, any>)(value)
}

/** The service key that `key` names: the key itself, or its export named `name`. */
type ProvidedKey<K, N extends string> = K extends Context.Key<any, any> ? K
  : K extends { readonly [P in N]: infer Key } ? Key
  : never
