# Effect Data Types

Schemas for Effect data types work as in Effect. Runtime collection schemas expect the actual runtime container on both sides and transform its contents; derive a canonical JSON codec when the external representation is plain JSON. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Building configuration-compatible codecs

```vibe
// run: Console.log(JSON.stringify(configurationExample()))
import { Console } from "effect"
const DispatchConfig = Config.schema(Schema.Struct({
  depot: Schema.String,
  capacity: Schema.Finite
}), "dispatch")

function configurationExample() {
  const provider = ConfigProvider.fromUnknown({
    dispatch: { depot: "north", capacity: 24 }
  })
  return DispatchConfig.parse(provider)
}
```

```output
{"depot":"north","capacity":24}
```

`Config.schema` builds a Config value without reading it. `DispatchConfig()` reads the current provider; `DispatchConfig.parse(provider)` reads the given provider and decodes its representation to the schema's type. The example's structured provider has a numeric capacity; string-based providers need a representation that accepts strings, such as a string-tree codec or explicit `FiniteFromString` fields. Configuration errors use Config's error model, not the raw Schema parser's.

## Encoding options, results, and exits

```vibe
// run: const values = decodeContainers(); Console.log(values.option.isSome() ? values.option.value : "none"); Console.log(values.result.isFailure() ? values.result.failure : "success"); Console.log(values.exit._tag); Console.log(values.exit())
const OptionalCount = Schema.Option(Schema.FiniteFromString)
const CountResult = Schema.Result(Schema.FiniteFromString, Schema.Trim)
const CountExit = Schema.Exit(Schema.FiniteFromString, Schema.String, Schema.Defect())

function decodeContainers() {
  const option = Schema.decodeUnknownEffect(OptionalCount)(Option.some("9"))
  const result = Schema.decodeUnknownEffect(CountResult)(Result.fail(" delayed "))
  const exit = Schema.decodeUnknownEffect(CountExit)(Exit.succeed("9"))
  return { option, result, exit }
}
```

```output
9
delayed
Success
9
```

`Option` applies its value schema to `Some` and keeps `None`. `Result(success, failure)` transforms each channel independently. `Exit(success, failure, defect)` also describes failures and defects inside the cause. `Exit.succeed` and `Exit.fail` construct data, and an Exit the parser returns stays data: a failure stored in it does not fail the surrounding parser. Calling the Exit runs it, succeeding or failing with its stored outcome.

```vibe
// run: Console.log(JSON.stringify(encodeResult()))
function encodeResult() {
  const codec = Schema.toCodecJson(CountResult)
  return Schema.encodeEffect(codec)(Result.succeed(9))
}
// { _tag: "Success", success: "9" }
```

```output
{"_tag":"Success","success":"9"}
```

Canonical JSON uses `{ _tag: "Some", value }` and `{ _tag: "None" }` for options, `Success/success` or `Failure/failure` for results, and `Success/value` or `Failure/cause` for exits; causes keep their failure reasons. `Schema.Defect()` reconstructs supported error-shaped defect data, including name, message, and optional stack.

| Option conversion | Inputs decoded as `None` | Default encoding of `None` |
| --- | --- | --- |
| `OptionFromUndefinedOr` | undefined | undefined |
| `OptionFromNullOr` | null | null |
| `OptionFromNullishOr` | null or undefined | undefined |
| `OptionFromOptionalKey` | missing key | omitted key |
| `OptionFromOptional` | missing key or undefined | omitted key |
| `OptionFromOptionalNullOr` | missing key, null, or undefined | omitted key |

`OptionFromNullishOr` accepts `onNoneEncoding: null` to choose null instead. `OptionFromOptionalNullOr` accepts `"omit"`, null, or undefined for the encoding policy.

## Supporting native and Effect collections

```vibe
// run: const values = collectionsFromJson(); Console.log(JSON.stringify(Array.from(values.set))); Console.log(JSON.stringify(Array.from(values.map))); Console.log(JSON.stringify(Array.from(Schema.decodeUnknownEffect(NativeSet)(new Set(["4", "7"]))))); Console.log(JSON.stringify(Array.from(Schema.decodeUnknownEffect(NativeMap)(new Map([["cold", "4"]])))))
const NativeSet = Schema.ReadonlySet(Schema.FiniteFromString)
const NativeMap = Schema.ReadonlyMap(Schema.String, Schema.FiniteFromString)
const PersistentSet = Schema.HashSet(Schema.FiniteFromString)
const PersistentMap = Schema.HashMap(Schema.String, Schema.FiniteFromString)

function collectionsFromJson() {
  const setCodec = Schema.toCodecJson(PersistentSet)
  const mapCodec = Schema.toCodecJson(PersistentMap)
  const set = Schema.decodeUnknownEffect(setCodec)(["4", "7"])
  const map = Schema.decodeUnknownEffect(mapCodec)([["cold", "4"]])
  return { set, map }
}
```

```output
[4,7]
[["cold",4]]
[4,7]
[["cold",4]]
```

`ReadonlySet` and `ReadonlyMap` describe native Set and Map containers; `HashSet` and `HashMap` describe Effect's persistent collections. Runtime schemas transform each element or entry and keep the container kind. Canonical JSON codecs encode sets as arrays of values and maps as arrays of key-value pairs, keeping the inner codec's explicit string encoding.

## Converting durations and protected values

```vibe
// run: const value = durations(); Console.log(value.milliseconds, Duration.toMillis(value.nanoseconds)); Console.log(JSON.stringify(value.json))
function durations() {
  const hold = Schema.decodeEffect(Schema.DurationFromString)("3 seconds")
  const milliseconds = Schema.encodeEffect(Schema.DurationFromMillis)(hold)
  const nanoseconds = Schema.decodeEffect(Schema.DurationFromNanos)(3_000_000_000n)
  const jsonCodec = Schema.toCodecJson(Schema.Duration)
  const json = Schema.encodeEffect(jsonCodec)(Duration.seconds(3))
  return { milliseconds, nanoseconds, json }
}
// json: { _tag: "Millis", value: 3000 }
```

```output
3000 3000
{"_tag":"Millis","value":3000}
```

`Schema.Duration` validates an existing Duration. `DurationFromString`, `DurationFromMillis`, and `DurationFromNanos` select text, numeric milliseconds, or bigint nanoseconds. The canonical Duration representation is tagged and preserves the supported precision and infinity cases.

```vibe
// run: const value = readToken("example-token"); Console.log(String(value)); const codec = Schema.toCodecJson(NonSerializableToken); try { Schema.encodeEffect(codec)(value) } catch (error) { Console.log(error._tag) }
const AccessToken = Schema.RedactedFromValue(Schema.NonEmptyString)
const NonSerializableToken = Schema.Redacted(Schema.String, {
  label: "DispatchToken",
  disallowJsonEncode: true
})

function readToken(input: unknown) {
  return Schema.decodeUnknownEffect(AccessToken)(input)
}
```

```output
<redacted>
SchemaError
```

`RedactedFromValue` validates a raw value and wraps it; `Redacted` expects a Redacted container and validates its hidden contents. Redaction protects display, not every serialization path: encoding a raw-value codec or a default canonical JSON codec can expose the contents. Set `disallowJsonEncode: true` on the Redacted schema to make its canonical JSON codec refuse encoding, and don't log decoded secrets.

[Upstream page](https://effect.website/docs/v4/schema/effect-data-types)
