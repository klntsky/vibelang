# Class APIs

Schema classes work in VibeLang as in Effect. A class can validate its fields, act as a codec, and expose domain behavior on its instances. Use one when methods or class identity help the model; use `Struct` for plain records. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Declaring validated classes

```vibe
// run: const value = readPackage({ reference: "pkg-94", pieces: "2" }); Console.log(value instanceof Package, value.pieces)
import { Console } from "effect"
class Package extends Schema.Class<Package>("Package")({
  reference: Schema.NonEmptyString,
  pieces: Schema.FiniteFromString
}) {}

function readPackage(input: unknown) {
  return Schema.decodeUnknownEffect(Package)(input)
}
// Package!Schema.SchemaError!
```

```output
true 2
```

Supply the class itself as the `Self` type, a stable identifier, and its fields. The class extends a generated base and is an ordinary data class, not a VibeLang service. The identifier is available as `Package.identifier` and helps Effect recognize class instances across module reloads.

## Constructing and checking instances

```vibe
// run: Console.log(constructPackage("pkg-94", 2).pieces, constructWithNew("pkg-94").pieces); try { constructPackage("", 2) } catch (issue) { Console.log(issue._tag) }
function constructPackage(reference: string, pieces: number) {
  return Package.makeEffect({ reference, pieces })
}

function constructWithNew(reference: string) {
  return try((reference) => new Package({ reference, pieces: 1 }), reference)
}
```

```output
2 1
Composite
```

Construction takes decoded fields, so `pieces` is a number even though decoding takes a string. `new Package(...)` and `Package.make(...)` apply defaults and type-side checks and throw for invalid construction; an explicit boundary accounts for those exceptions. `makeEffect` reports raw `SchemaIssue.Issue` failures in local Effect 4.0.5. An empty-field class can be constructed without an argument.

Pass a checked `Schema.Struct` instead of a fields object when the class needs a cross-field invariant. Its checks apply during construction and codec use.

## Bypassing checks deliberately

```vibe
// run: Console.log(fromVerifiedFields("", 2).reference === "")
function fromVerifiedFields(reference: string, pieces: number) {
  return try((reference, pieces) => new Package({ reference, pieces }, { disableChecks: true }), reference, pieces)
}
```

```output
true
```

`disableChecks` skips validation. It does not decode fields or prove an untrusted value valid. Use it only when the input already satisfies the decoded model; constructors still have their normal default-handling behavior.

## Defining structural equality

```vibe
// run: Console.log(sameContainer())
class Container extends Schema.Class<Container>("Container")({
  labels: Schema.Array(Schema.String)
}) {}

function sameContainer() {
  const first = Container.makeEffect({ labels: ["cold", "sealed"] })
  const second = Container.makeEffect({ labels: ["cold", "sealed"] })
  return Equal.equals(first, second) // true
}
```

```output
true
```

Schema-backed class instances support Effect's structural equality, including nested arrays and objects. JavaScript `===` still compares object identity. Keep fields stable when values participate in hash collections.

## Adding getters and methods

```vibe
// run: Console.log(announcePackage())
class LabeledPackage extends Schema.Class<LabeledPackage>("LabeledPackage")({
  reference: Schema.NonEmptyString
}) {
  get displayReference() {
    return this.reference.toUpperCase()
  }

  describe() {
    return `Package ${this.reference}`
  }

  announce() {
    Console.log(this.describe())
  }
}

function announcePackage() {
  const item = LabeledPackage.makeEffect({ reference: "pkg-94" })
  item.announce()
  return item.displayReference
}
```

```output
Package pkg-94
PKG-94
```

Getters derive synchronous values from fields. Methods follow ordinary VibeLang call rules and can run effects; calling `announce` logs before returning. Keep getters pure so formatting, checks, and compiled validators can read properties safely.

## Using classes as codecs

```vibe
// run: Console.log(JSON.stringify(packageWireRoundTrip([{ reference: "pkg-94", pieces: "2" }])))
const Packages = Schema.Array(Package)

function packageWireRoundTrip(input: unknown) {
  const packages = Schema.decodeUnknownEffect(Packages)(input)
  return Schema.encodeEffect(Packages)(packages)
}
```

```output
[{"reference":"pkg-94","pieces":"2"}]
```

A class schema is a transformation: decoding a plain object creates an instance, and encoding an instance produces a plain object with encoded fields. Classes fit anywhere a schema is expected, including arrays and nested structs. `Package.fields` exposes the original field descriptions for reuse.

## Annotating classes

```vibe
// run: Console.log(Dock.identifier); Console.log(Schema.resolveAnnotations(Dock)?.title); Console.log(Dock.makeEffect({ number: 2 }).number)
class Dock extends Schema.Class<Dock>("Dock")(
  { number: Schema.Int },
  { title: "Loading dock", description: "A numbered dispatch location" }
) {}
```

```output
Dock
Loading dock
2
```

Pass annotations after the fields or struct. The class identifier also becomes its default identifier annotation. Metadata documents the model without adding checks or changing construction defaults.

## Modeling recursive classes

```vibe
// run: const value = readCrate({ count: "2", children: [{ count: "1", children: [] }] }); Console.log(value instanceof Crate, value.children[0] instanceof Crate); Console.log(JSON.stringify(Schema.encodeEffect(Crate)(value)))
interface CrateWire {
  count: string
  children: ReadonlyArray<CrateWire>
}

class Crate extends Schema.Class<Crate>("Crate")({
  count: Schema.FiniteFromString,
  children: Schema.Array(
    Schema.suspend((): Schema.Codec<Crate, CrateWire>!! => Crate)
  )
}) {}

function readCrate(input: unknown) {
  return Schema.decodeUnknownEffect(Crate)(input)
}
```

```output
true true
{"count":"2","children":[{"count":"1","children":[]}]}
```

Suspend the recursive edge and write the callback's full contract to break the inference cycle. It returns the codec as data and compiles to a plain JavaScript callback. Specify a separate encoded interface when fields transform. For mutual recursion, suspend the reference to the class declared later and state the appropriate decoded and encoded types on each recursive edge.

## Creating tagged variants and extending classes

```vibe
// run: Console.log(Packed.makeEffect({ reference: "pkg-94" })._tag); Console.log(InsuredPackage.makeEffect({ reference: "pkg-94", pieces: 2, insuredValue: 100 }).summary()); try { reportMissing("pkg-94") } catch (error) { Console.log(error._tag) }
class Packed extends Schema.TaggedClass<Packed>()("Packed", {
  reference: Schema.String
}) {}

class MissingPackage extends Schema.TaggedError<MissingPackage>()("MissingPackage", {
  reference: Schema.String
}) {}

class InsuredPackage extends Package.extend<InsuredPackage>("InsuredPackage")({
  insuredValue: Schema.Finite
}) {
  summary() {
    return `${this.reference}: ${this.insuredValue}`
  }
}

function reportMissing(reference: string) {
  throw MissingPackage.makeEffect({ reference })
}
// void!MissingPackage | SchemaIssue.Issue!
```

```output
Packed
pkg-94: 100
MissingPackage
```

`TaggedClass` adds `_tag` with a constructor default. `Schema.TaggedError` also supplies Error behavior and a serializable schema; the constructed instance is data until thrown or called. `new` constructs it without running it; throw it for an expected failure. The example also retains the constructor's issue channel. For errors that do not need serialization, use the global `TaggedError` instead.

`extend` adds fields while preserving inherited behavior. Give each extension its own identifier. Schema data-class inheritance is separate from service declarations, whose implementations use object literals or `implements`.

[Upstream page](https://effect.website/docs/v4/schema/classes)
