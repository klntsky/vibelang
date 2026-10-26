# Redacted

`Redacted` works in VibeLang exactly as in Effect. It hides a sensitive value from string, JSON, and inspection output, helping prevent accidental disclosure while allowing deliberate access.

## Wrapping sensitive values

```vibe
// run: Console.log(String(exampleKey)); reportConfiguration(); Console.log(JSON.stringify(exampleKey))
const exampleKey = Redacted.make("example-key-not-a-real-secret")
String(exampleKey) // "<redacted>"

function reportConfiguration() {
  Console.log(JSON.stringify({ apiKey: exampleKey }))
}
```

```output
<redacted>
{"apiKey":"<redacted>"}
"<redacted>"
```

`make` keeps the original value behind a wrapper that displays as redacted. The wrapper is not encryption; code can still retrieve the secret. Wrap a value as soon as it enters the application, before diagnostic code sees it.

## Revealing values deliberately

```vibe
// run: provide Layer.succeed(CredentialsClient, { authenticate: (token) => token === "example-token" }) { Console.log(authenticate(Redacted.make("example-token")), authenticate(Redacted.make("wrong-token"))) }
service CredentialsClient {
  authenticate(token: string): boolean!!
}

function authenticate(token: Redacted.Redacted<string>) {
  return CredentialsClient.authenticate(Redacted.value(token))
}
// boolean!!CredentialsClient
```

```output
true false
```

`value` returns the stored payload. Reveal it only at the operation that needs the raw value, and keep it out of messages, error fields, and log annotations: redaction cannot protect an extracted string.

## Wiping stored secrets

```vibe
// run: provide Layer.succeed(CredentialsClient, { authenticate: (token) => token === "example-token" }) { Console.log(useOnce("example-token")) }; const wiped = Redacted.make("example-token"); Redacted.wipeUnsafe(wiped); const failed = Effect.exit(() => Redacted.value(wiped)); Console.log(failed.isFailure(), String(wiped))
function useOnce(raw: string) {
  const token = Redacted.make(raw)
  try {
    return authenticate(token)
  } finally {
    Redacted.wipeUnsafe(token)
  }
}
```

```output
true
true <redacted>
```

`wipeUnsafe` removes the wrapper's stored value; later `Redacted.value` calls throw. `finally` runs on success, failure, and interruption, so the wrapper drops its reference at the operation boundary. Wiping does not erase the original `raw` string, other aliases, or bytes already copied elsewhere in memory.

## Comparing protected values

```vibe
// run: Console.log(same(first, second), same(first, third))
const same = Redacted.makeEquivalence(Equivalence.String)
const first = Redacted.make("example-a")
const second = Redacted.make("example-a")
const third = Redacted.make("example-b")
same(first, second) // true
same(first, third) // false
```

```output
true false
```

`makeEquivalence` lifts a relation on payloads to one on wrappers. The comparison prints no secret, but its boolean result reveals whether they match, and it gives no constant-time cryptographic guarantee. Compare before wiping, while both payloads remain.
