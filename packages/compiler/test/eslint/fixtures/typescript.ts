/**
 * Every TypeScript form the differential test holds vibelang --estree to,
 * against @typescript-eslint/typescript-estree.
 *
 * @since 0.0.2
 */

"use strict"
import def, * as everything from "./a.js"
import other, { named, type Kind, default as renamed, "string name" as stringName } from "./b.js"
import type { OnlyType } from "./c.js"
import type * as Types from "./d.js"
import json from "./data.json" with { type: "json" }
import "./side-effect.js"
import Legacy = require("legacy")
import Alias = everything.member
export import Reexported = everything.inner
export { named, renamed as again }
export type { Kind }
export * from "./e.js"
export * as namespaced from "./f.js"
export { default } from "./g.js"
export { type OnlyType as Exported } from "./c.js"

// Declarations
export const answer: number = 42, other2 = "text"
let mutable = 1
var legacy
const { a, b: [c, , d = 2, ...rest], ...remaining } = everything as any
const [first, second = first, ...tail] = [1, 2, 3]
declare const declared: string
declare let declaredLet: number
using resource = open()
export declare function overloaded(x: string): string
export declare function overloaded(x: number): number
export function overloaded(x: unknown): unknown {
  return x
}
function* generator<T,>(this: Window, ...items: T[]): Generator<T, void, unknown> {
  yield* items
  yield
}
async function asynchronous(): Promise<void> {
  await Promise.resolve()
  for await (const item of source()) void item
  await using handle = open()
}
export default function () {}
export abstract class Base<in out T extends object = {}> extends Parent<T> implements One, Two<T> {
  static #count = 0
  #secret?: string
  declare readonly field: T
  public static override readonly x = 1
  protected y!: number
  private z?: string = "z"
  accessor auto = 1
  abstract accessor abstractAuto: number
  abstract method(): void
  abstract property: string;
  [key: string]: unknown
  static {
    Base.#count++
  }
  constructor(private readonly injected: string, public other?: number, ...args: unknown[]) {
    super()
  }
  get value(): T {
    return this.field
  }
  set value(next: T) {}
  static async *stream(): AsyncGenerator<number> {}
  "quoted"() {}
  42() {}
  [Symbol.iterator]() {}
  optional?(): void
  @decorated() decoratedMethod(@param() p: string) {}
}
@sealed
class Decorated {}
interface Shape<T = unknown> extends Base<T>, Other {
  readonly id: string
  name?: string
  method<U>(u: U): T
  optionalMethod?(): void
  (call: number): string
  new (construct: string): Shape
  [index: number]: string
  get getter(): string
  set setter(value: string)
}
type Union = "a" | "b" | 1 | -1 | 1n | true | null | undefined | void
type Intersection = A & B & { c: string }
type Conditional<T> = T extends string ? "s" : T extends infer U extends number ? U : never
type Mapped<T> = { readonly [K in keyof T as `get${Capitalize<K & string>}`]-?: () => T[K] }
type Mapped2<T> = { -readonly [K in keyof T]+?: T[K] }
type Template = `prefix-${string}-${number}`
type Tuple = [a: string, b?: number, ...rest: boolean[]]
type Tuple2 = readonly [string, number?, ...boolean[]]
type Fn = (this: void, a: string, b?: number) => void
type Ctor = abstract new (...args: any[]) => object
type Query = typeof answer
type QueryThis = typeof this.x
type ImportType = import("./x.js").Name<string>
type ImportTypeOf = typeof import("./x.js")
type ImportWith = import("./x.js", { with: { "resolution-mode": "import" } }).T
type Predicate = (x: unknown) => x is string
type Asserts = (x: unknown) => asserts x is string
type AssertsThis = (x: unknown) => asserts this
type Keys = keyof typeof everything
type Unique = unique symbol
type Indexed = Shape["id"]
type Arrays = Array<string>[] | readonly string[] | ReadonlyArray<number>
type Literal = { a: 1; b: "two"; "c-d": `three` }
type Intrinsics = string | number | bigint | boolean | symbol | object | any | unknown | never
enum Color { Red, Green = "green", Blue = 1 << 2 }
const enum Fixed { A = 1 }
declare enum Ambient { X }
namespace Space {
  export const inside = 1
}
namespace Outer.Inner.Deep {
  export type T = string
}
declare module "external" {
  export const x: number
}
declare global {
  interface Window {
    extra: string
  }
}
declare module "shorthand"

// Expressions and statements
label: for (let i = 0, j = 10; i < j; i++, j--) {
  if (i === 2) continue label
  else if (i > 5) break label
  else {
  }
}
for (const key in everything) {}
for (const value of [1, 2]) {}
for (;;) break
while (mutable < 10) mutable += 2
do mutable--
while (mutable > 0)
switch (mutable) {
  case 1:
  case 2: {
    break
  }
  default:
    mutable = 0
}
try {
  throw new Error("x")
} catch {
} finally {
}
try {
} catch ({ message }) {
  void message
}
try {
} catch (error: unknown) {
}
with (Math) void max
debugger
;
const object = {
  a,
  b: 2,
  [c]: 3,
  "d": 4,
  5: 5,
  method() {},
  get g() { return 1 },
  set g(v) {},
  async *gen() {},
  ...rest,
  shorthand = 1,
}
const values = [1, , 3, ...tail]
const arrow = async <T,>(x: T, { y }: { y: number } = { y: 1 }): Promise<T> => x
const arrowExpression = () => ({ object })
const fn = function named<T>(this: unknown) {}
const klass = class Named<T> extends Base<T> {}
const anonymousClass = class {}
const template = `a${1}b${`nested ${2}`}c`
const tagged = tag`hello ${world}!`
const taggedGeneric = tag<string>`x`
const raw = String.raw`\unicode`
const escapes = String.raw`\u0065 \x61 \u{62}` + tag`\xZ` + `plain \u0041`
const regex = /a[b-c]+\/(?<name>d)/giu
const division = 4 / 2 / 1
const numbers = [0, 1.5, .5, 1e10, 0x1f, 0o17, 0b101, 1_000_000, 10n, 0x10n, 1e400]
const strings = ["double", 'single', "escape\n\"", "\u{1F600}", "é"]
const unary = [-a, +b, !c, ~d, typeof a, void 0, delete object.a, ++mutable, mutable--]
const binary = [a + b, a - b, a * b, a / b, a % b, a ** b, a << b, a >> b, a >>> b, a & b, a | b, a ^ b]
const comparison = [a == b, a != b, a === b, a !== b, a < b, a > b, a <= b, a >= b, a in b, a instanceof B]
const logical = [a && b, a || b, a ?? b]
mutable ||= 1
mutable &&= 2
mutable ??= 3
mutable **= 2
mutable >>>= 1
mutable >>= 1
mutable <<= 1
const conditional = a ? b : c ? d : 0
const sequence = (a, b, c)
const optional = object?.a?.b?.[c]?.(d)!.e
const nonNull = object!.a!
const chainParen = (object?.a).b
const calls = [fn(), fn<string>(), new Base(), new Base<string>, new (klass)(), fn(...values)]
const meta = [import.meta.url, new.target]
const dynamic = import("./dynamic.js")
const dynamicWith = import("./dynamic.js", { with: { type: "json" } })
const assertions = [a as string, <number>b, c satisfies unknown, d as const, a!]
const instantiation = fn<string>
const privateIn = #secret in object
const thisValue = this
const superCall = class extends Base<string> { m() { return super.method() } }
async function* yields() {
  const sent = yield 1
  yield* [sent]
  await sent
}
function defaults(a = 1, { b, c: [d] } = {}, [e] = [], ...f: number[]) {
  return arguments.length
}
export = answer
