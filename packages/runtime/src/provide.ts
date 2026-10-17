import { Context, Layer } from "effect"

/**
 * A layer that provides one service, for a provider written `Key = value` in a
 * provide block (D173):
 *
 *     provide Clock = testClock, Users = fakeUsers { app() }
 *
 * compiles to `Effect.provide(..., [$provideService(Clock, testClock, "Clock"),
 * $provideService(Users, fakeUsers, "Users")])`. The key is a service key, such
 * as a `service` declaration, or a module whose export of the same name is
 * one, so the effect module `Clock` stands for `Clock.Clock`.
 */
export function $provideService<const N extends string, M extends { readonly [K in N]: Context.Key<any, any> }>(
  module: M,
  value: NoInfer<Context.Service.Shape<M[N]>>,
  name: N
): Layer.Layer<Context.Service.Identifier<M[N]>>
// Last, so a type error names the key's shape rather than the module form.
export function $provideService<I, S>(key: Context.Key<I, S>, value: NoInfer<S>, name?: string): Layer.Layer<I>
export function $provideService(key: any, value: unknown, name?: string): Layer.Layer<any> {
  const service = Context.isKey(key) ? key : key[name!]
  if (!Context.isKey(service)) {
    throw new TypeError(`provide ${name} = ...: ${name} is not a service key, and it has no service key named ${name}`)
  }
  return Layer.succeed(service as Context.Key<any, any>)(value)
}
