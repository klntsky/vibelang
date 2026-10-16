import { Context, Effect, Layer } from "effect"
import * as K from "./kernel/kernel.js"

export const $Context = Context

type LOut<L> = L extends Layer.Layer<infer O, any, any> ? O : never
type LErr<L> = L extends Layer.Layer<any, infer E, any> ? E : never
type LIn<L> = L extends Layer.Layer<any, any, infer I> ? I : never

export const wire = <const L extends readonly [Layer.Any, ...Array<Layer.Any>]>(
  ...layers: L
): Layer.Layer<LOut<L[number]>, LErr<L[number]>, Exclude<LIn<L[number]>, LOut<L[number]>>> => {
  let acc = layers[layers.length - 1] as Layer.Layer<any, any, any>
  for (let i = layers.length - 2; i >= 0; i--) acc = Layer.provideMerge(layers[i] as Layer.Layer<any, any, any>, acc)
  return acc as any
}

// ---------------------------------------------------------------------------
// Control operations and handlers (D2, D16-D18), on the continuation kernel
// ---------------------------------------------------------------------------

/** D2: control operations are abstract classes that `extend Control`; the compiler rewrites them. */
export abstract class Control {}

interface ControlInfo {
  readonly ops: Record<string, K.Operation<any, any, any>>
  readonly impl: Record<string, (...args: Array<any>) => Effect.Effect<any, any, any>>
}
export const controls = new WeakMap<object, ControlInfo>()

/** Registers a generated control class: one kernel operation per abstract method. */
export const $control = (cls: Context.Key<any, any>, methods: ReadonlyArray<string>) => {
  const ops: Record<string, K.Operation<any, any, any>> = {}
  const impl: Record<string, (...args: Array<any>) => Effect.Effect<any, any, any>> = {}
  for (const m of methods) {
    const op = K.operation<Array<any>, any, any>(`${cls.key}.${m}`)
    ops[m] = op
    impl[m] = (...args) => op(args)
  }
  controls.set(cls, { ops, impl })
}

/**
 * D1: the base of a generated service class. Its static side is an Effect 4
 * Context key whose identifier and shape are both the service's interface;
 * its instance side is empty, so the merged interface is the class's type.
 */
export const $service = <I>(key: string): (abstract new () => {}) & Context.Key<I, I> =>
  Context.Service<I, I>()(key) as any

/**
 * D40: the base of a generated service class that declares
 * `static readonly default`. Its static side is an Effect 4
 * `Context.Reference`, so a program that uses the service needs no provider:
 * the first lookup without one reads `defaultValue`, which returns the
 * class's `default`.
 */
export const $reference = <I>(key: string, defaultValue: () => I): (abstract new () => {}) & Context.Reference<I> =>
  Context.Reference<I>(key, { defaultValue }) as any
