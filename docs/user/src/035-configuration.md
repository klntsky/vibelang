# Configuration

Use `Config` to load typed settings and `ConfigProvider` to choose their source. Configuration uses Effect's validation and error types. A call that returns `Config<T>` builds a descriptor; calling that descriptor loads its value.

## Declare typed configuration

```ts
import { Config } from "effect"

const Host = Config.NonEmptyString("HOST")
const Port = Config.Port("PORT")

function serverSettings() {
  const host = Host()
  const port = Port()
  return { host, port }
}
// Effect<{ host: string; port: number }, ConfigError, never>
```

`Host()` and `Port()` load values from the active provider, which defaults to the environment. Constructing the descriptors reads nothing. Missing or invalid input fails with `ConfigError`. No direct environment access is needed in `.es` files.

| Constructor | Decoded value |
| --- | --- |
| `Config.String`, `Config.NonEmptyString` | A string, optionally required to be nonempty |
| `Config.Finite`, `Config.Int`, `Config.Port` | A finite number, integer, or port from 1 to 65535 |
| `Config.Boolean` | A boolean |
| `Config.Literal`, `Config.Literals` | A specific literal or one of several literals |
| `Config.Duration`, `Config.Date`, `Config.URL` | The corresponding value type |
| `Config.LogLevel` | A log severity |
| `Config.Redacted` | A protected string |

Prefer `Finite` to `Number` for ordinary numeric settings: `Number` also accepts non-finite values. Boolean text is case sensitive and supports `true`/`false`, `yes`/`no`, `on`/`off`, `1`/`0`, and `y`/`n`.

`Config.all`, `Config.nested`, and `Config.withDefault` compose descriptors as data in `.es`. A type that extends `Effect` with its own structure stays data when returned by a call; only a call whose result type is `Effect` itself runs that result automatically.

## Parse with schemas

```ts
import { Config } from "effect"

function teamName() {
  return Config.schema(
    Schema.String.check(Schema.isMinLength(3)),
    "TEAM_NAME"
  )()
}
// Effect<string, ConfigError, never>
```

`Config.schema` builds a descriptor that decodes the provider's representation using the schema. Calling it returns the decoded value. Put constraints in the schema so invalid input reports a configuration failure. A structured schema can describe an entire object, with field paths mapping to environment keys or object properties.

```ts
// descriptors.es
import { Config } from "effect"

export const server = Config.schema(Schema.Struct({
  host: Schema.String,
  port: Schema.Int.check(Schema.isBetween({ minimum: 1, maximum: 65535 }))
}), "server")
```

```ts
import { ConfigProvider } from "effect"
import { server } from "./descriptors.es"

function localServer() {
  return server.parse(ConfigProvider.fromUnknown({
    server: { host: "127.0.0.1", port: 9091 }
  }))
}
```

Use `descriptor.parse(provider)` when the source is explicit, especially in tests. Its result is an `Effect`, so the call runs it. Calling the descriptor as `server()` instead uses the active provider.

## Read arrays and records

```ts
import { Config } from "effect"

function routingSettings() {
  const regions = Config.Array(Schema.String, "REGIONS")()
  const labels = Config.Record(Schema.String, Schema.String, "LABELS")()
  return { regions, labels }
}
```

Arrays accept structural values or a separated string such as `north,south`. Records accept objects or text such as `team=search,tier=internal`. Both constructors take separator options; use plain schema arrays or records through `Config.schema` when only structural input should be accepted.

## Nest and combine configuration

```ts
// database-config.es
import { Config } from "effect"

export const database = Config.nested(Config.all({
  host: Config.NonEmptyString("HOST"),
  port: Config.Port("PORT")
}), "DATABASE")
```

```ts
import { ConfigProvider } from "effect"
import { database } from "./database-config.es"

function testDatabaseSettings() {
  return database.parse(ConfigProvider.fromEnv({
    env: { DATABASE_HOST: "db.internal", DATABASE_PORT: "5433" }
  }))
}
```

`Config.all` preserves the input's tuple or named-object shape. `Config.nested` prepends a path to every lookup; the environment provider joins segments with `_`. Libraries accepting nested records of descriptors can use `Config.Wrap<T>` for the input type and `Config.unwrap` to build the combined descriptor.

## Defaults and fallbacks

```ts
// optional-config.es
import { Config } from "effect"

export const port = Config.withDefault(Config.Port("PORT"), 9091)
export const apiKey = Config.option(Config.String("API_KEY"))
export const host = Config.orElse(
  Config.NonEmptyString("HOST"),
  () => Config.NonEmptyString("BACKUP_HOST")
)
```

```ts
import { ConfigProvider } from "effect"
import { port, apiKey, host } from "./optional-config.es"

function defaultsExample() {
  const provider = ConfigProvider.fromUnknown({ BACKUP_HOST: "standby.internal" })
  return {
    port: port.parse(provider),
    apiKey: apiKey.parse(provider),
    host: host.parse(provider)
  }
}
```

`withDefault` replaces missing input and preserves validation failures. `option` returns `Option.none()` for absence. `orElse` tries another descriptor after any `ConfigError`, including invalid input, so use that broader recovery deliberately.

A default on a `Config.all` group replaces the whole group when any child is absent, unless another child fails validation or its source fails. Put defaults on individual children to preserve the supplied siblings.

## Transform parsed values

```ts
// origin-config.es
import { Config } from "effect"

export const origin = Config.map(Config.all({
  host: Config.NonEmptyString("HOST"),
  port: Config.Port("PORT")
}), ({ host, port }) => `https://${host}:${port}`)
```

```ts
import { origin } from "./origin-config.es"

function reportOrigin() {
  const value = origin()
  Effect.log(value)
  return value
}
```

Use `Config.map` for a transformation that cannot fail. Express validation in a schema; use `Config.mapEffect` when transformation needs an Effect with `ConfigError`. Loading first and transforming with ordinary EffectScript statements also works when you do not need a reusable descriptor.

## Protect secrets

```ts
import { Config } from "effect"

function secretSetting() {
  const token = Config.Redacted("ACCESS_TOKEN")()
  Effect.log(token)
  return token
}
```

`Redacted` hides its contents in ordinary string representations. Import `Redacted` from `effect` and use `Redacted.value(token)` only at the point that needs the raw secret. For a decoded secret, combine `Config.schema` with `Schema.RedactedFromValue`, such as a protected number decoded with `Schema.FiniteFromString`.

## Load providers

```ts
import { Config, ConfigProvider } from "effect"

const TestProvider = ConfigProvider.fromUnknown({ PORT: 9091 })
const EnvProvider = ConfigProvider.fromEnv({ env: { PORT: "9092" } })
const DotEnvProvider = ConfigProvider.fromDotEnvContents("PORT=9093")

function testPort() {
  return Effect.provide(() => Config.Port("PORT")(), ConfigProvider.layer(TestProvider))
}

function fileProvider() {
  return ConfigProvider.fromDotEnv({ path: ".env.local" })
}
```

`fromUnknown` reads object properties and array indexes as path segments. `fromEnv()` reads the host environment; an explicit `{ env }` replaces that source for tests. Both normally treat empty strings as missing; use `preserveEmptyStrings: true` when empty text has meaning.

`fromDotEnvContents` parses text already in memory. `fromDotEnv` reads a file through `FileSystem`, and `fromDir` reads a directory tree through `FileSystem` and `Path`; provide their platform layers before running them. `ConfigProvider.make` supports a custom backing store.

## Remap provider paths

```ts
import { Config, ConfigProvider } from "effect"

const DeploymentProvider = ConfigProvider.constantCase(
  ConfigProvider.fromEnv({ env: { SERVICE_PORT: "9094" } })
)

function deploymentPort() {
  return Effect.provide(
    () => Config.Port("servicePort")(),
    ConfigProvider.layer(DeploymentProvider)
  )
}
```

`constantCase` converts requested string segments to environment-style names. `ConfigProvider.nested` adds a fixed prefix, and `ConfigProvider.mapInput` transforms a requested path. Adapt the provider when an entire source uses a naming convention; nest a descriptor when just one settings group has a prefix.

## Compose and install providers

```ts
import { Config, ConfigProvider } from "effect"

const Combined = ConfigProvider.orElse(
  ConfigProvider.fromEnv({ env: { HOST: "edge.internal" } }),
  ConfigProvider.fromUnknown({ HOST: "localhost", PORT: 9091 })
)

function configuredServer() {
  return Effect.provide(() => {
    return { host: Config.String("HOST")(), port: Config.Port("PORT")() }
  }, ConfigProvider.layer(Combined))
}
```

Provider fallback fills absent paths; it does not recover from source or validation errors. `ConfigProvider.layer` replaces the active provider for the supplied region. `layerAdd` adds a fallback to the current provider, or takes `{ asPrimary: true }` to give the added source precedence.

## Coming from Effect

Compose Config descriptors directly in `.es`, then call a descriptor to load it, or call its `parse` method with an explicit provider. Returning a descriptor does not read it, just as returning an `Exit`, a Context key, or an error instance does not run that value. `new` constructs an instance and never runs it.

[Upstream Effect page](https://effect.website/docs/v4/configuration)
