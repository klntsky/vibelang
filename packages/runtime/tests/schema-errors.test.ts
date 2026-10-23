import { expect, test } from "bun:test"
import { Option, Schema } from "effect"
import { $schemaErrorFamily, $schemaErrorMember } from "vibelang-runtime"

// D304: the classes a schema error family's declarations lower to.
class StoreError extends Schema.TaggedError<StoreError>()("StoreError" as string, { key: Schema.String }) {
  static {
    $schemaErrorFamily(this)
  }
}
class NotStored extends StoreError.extend<NotStored>("NotStored")({ _tag: Schema.tag("NotStored"), attempts: Schema.Number }) {
  static {
    $schemaErrorMember(this)
  }
  describe(): string {
    return `not stored after ${this.attempts}`
  }
}
class Expired extends StoreError.extend<Expired>("Expired")({ _tag: Schema.tag("Expired" as string), at: Schema.Number }) {
  static {
    $schemaErrorFamily(this)
  }
}
class Evicted extends Expired.extend<Evicted>("Evicted")({ _tag: Schema.tag("Evicted"), by: Schema.String }) {
  static {
    $schemaErrorMember(this)
  }
}

const notStored = new NotStored({ key: "a", attempts: 2 })

test("schema error family: the base's schema encodes a subclass's instance with the subclass's schema", () => {
  expect(Schema.encodeSync(StoreError)(notStored)).toEqual({ _tag: "NotStored", key: "a", attempts: 2 })
  expect(Schema.encodeSync(StoreError)(new Evicted({ key: "b", at: 3, by: "lru" }))).toEqual({ _tag: "Evicted", key: "b", at: 3, by: "lru" })
  expect(Schema.encodeSync(Expired)(new Evicted({ key: "b", at: 3, by: "lru" }))).toEqual({ _tag: "Evicted", key: "b", at: 3, by: "lru" })
  expect(Schema.encodeSync(Schema.toCodecJson(StoreError))(notStored)).toEqual({ _tag: "NotStored", key: "a", attempts: 2 })
})

test("schema error family: the base's schema decodes a subclass's record by tag", () => {
  const decoded = Schema.decodeUnknownSync(StoreError)({ _tag: "NotStored", key: "a", attempts: 2 })
  expect(decoded).toBeInstanceOf(NotStored)
  expect((decoded as NotStored).describe()).toBe("not stored after 2")
  expect(Schema.decodeUnknownSync(StoreError)({ _tag: "Evicted", key: "c", at: 4, by: "ttl" })).toBeInstanceOf(Evicted)
  expect(Schema.decodeUnknownSync(StoreError)({ _tag: "Expired", key: "c", at: 4 })).toBeInstanceOf(Expired)
  expect(Option.isNone(Schema.decodeUnknownOption(StoreError)({ _tag: "Unknown", key: "x" }))).toBe(true)
  expect(Option.isNone(Schema.decodeUnknownOption(StoreError)({ _tag: "NotStored", key: "x" }))).toBe(true)
})

test("schema error family: base instances and a subclass's own schema are unchanged", () => {
  const base = new StoreError({ key: "s" })
  expect(Schema.encodeSync(StoreError)(base)).toEqual({ _tag: "StoreError", key: "s" })
  const decoded = Schema.decodeUnknownSync(StoreError)({ _tag: "StoreError", key: "s" })
  expect(decoded.constructor).toBe(StoreError)
  expect(Schema.is(NotStored)(base)).toBe(false)
  expect(Option.isNone(Schema.encodeOption(NotStored)(base as NotStored))).toBe(true)
  expect(Schema.encodeSync(NotStored)(notStored)).toEqual({ _tag: "NotStored", key: "a", attempts: 2 })
  expect(StoreError.make({ key: "m" })).toBeInstanceOf(StoreError)
})

test("schema error family: checks and annotations still apply to the base's schema", () => {
  const long = StoreError.check(Schema.makeFilter((error: StoreError) => error.key.length > 3))
  expect(Option.isNone(Schema.encodeOption(long)(notStored))).toBe(true)
  expect(Schema.encodeSync(StoreError.annotate({ title: "store" }))(notStored)).toEqual({ _tag: "NotStored", key: "a", attempts: 2 })
})

test("schema error family: a schema built from the base before a subclass is defined lists it", () => {
  const Envelope = Schema.Struct({ id: Schema.String, error: StoreError })
  class Late extends StoreError.extend<Late>("Late")({ _tag: Schema.tag("Late") }) {
    static {
      $schemaErrorMember(this)
    }
  }
  expect(Schema.encodeSync(Envelope)({ id: "e", error: new Late({ key: "l" }) })).toEqual({ id: "e", error: { _tag: "Late", key: "l" } })
})

test("schema error family: a read after a later subclass is defined lists it, and a class defined again wins its tag", () => {
  expect(Schema.encodeSync(StoreError)(notStored)).toEqual({ _tag: "NotStored", key: "a", attempts: 2 })
  class Later extends StoreError.extend<Later>("Later")({ _tag: Schema.tag("Later") }) {
    static {
      $schemaErrorMember(this)
    }
  }
  expect(Schema.decodeUnknownSync(StoreError)({ _tag: "Later", key: "z" })).toBeInstanceOf(Later)
  class Reloaded extends StoreError.extend<Reloaded>("Later")({ _tag: Schema.tag("Later") }) {
    static {
      $schemaErrorMember(this)
    }
  }
  expect(Schema.decodeUnknownSync(StoreError)({ _tag: "Later", key: "z" })).toBeInstanceOf(Reloaded)
})
