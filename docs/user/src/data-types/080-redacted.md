# Redacted

`Redacted` works in Effeck exactly as in Effect. It hides a sensitive value from ordinary string, JSON, and inspection output, helping prevent accidental disclosure while allowing deliberate access.

## Wrapping sensitive values

```ts
const exampleKey = Redacted.make("example-key-not-a-real-secret")
String(exampleKey) // "<redacted>"

function reportConfiguration() {
  log({ apiKey: exampleKey })
}
```

`make` retains the original value behind a wrapper whose display is redacted. The wrapper is not encryption and does not prevent code from retrieving the secret. Wrap a value as soon as it enters the application, before diagnostic code sees it.

## Revealing values deliberately

```ts
service class CredentialsClient {
  authenticate(token: string): boolean!!
}

function authenticate(token: Redacted.Redacted<string>) {
  return CredentialsClient.authenticate(Redacted.value(token))
}
// boolean!!CredentialsClient
```

`value` returns the stored payload. Reveal it only at the operation that requires the raw value and keep it out of messages, error fields, and log annotations. Redaction cannot protect a string that has already been extracted.

## Wiping stored secrets

```ts
function useOnce(raw: string) {
  const token = Redacted.make(raw)
  try {
    return authenticate(token)
  } finally {
    Redacted.wipeUnsafe(token)
  }
}
```

`wipeUnsafe` removes the wrapper's stored value; later `Redacted.value` calls throw. Cleanup runs on success, failure, and interruption, so this releases the wrapper's reference at the operation boundary. It does not erase the original `raw` string, other aliases, or bytes already copied elsewhere in memory.

## Comparing protected values

```ts
const same = Redacted.makeEquivalence(Equivalence.String)
const first = Redacted.make("example-a")
const second = Redacted.make("example-a")
const third = Redacted.make("example-b")
same(first, second) // true
same(first, third) // false
```

`makeEquivalence` lifts a relation for payloads to a relation for wrappers. The comparison does not print the secret, but its boolean result still reveals whether they match. This helper provides no constant-time cryptographic comparison guarantee; compare before wiping, while both payloads remain available.
