# Batching

Use requests and resolvers when independent lookups can share a backend call. A request describes one answer; a resolver collects compatible pending requests and completes each one with its own success or failure.

## Model requests and domain errors

```ts
type Part = { readonly code: string; readonly label: string }

error PartMissing {
  code: string
}

error CatalogUnavailable

interface FindPart extends Request.Request<Part, PartMissing | CatalogUnavailable> {
  readonly _tag: "FindPart"
  readonly code: string
}

const FindPart = Request.tagged<FindPart>("FindPart")
```

The request carries the input, success type, and expected errors. `Request.tagged` supplies structural equality and hashing so equal request values can be compared for cache reuse. Include every input that affects the answer, such as a tenant or locale, in a real request model.

A direct service call for each item performs repeated backend work. Batching helps only when the data source can actually answer several inputs together; it does not create a bulk endpoint for you.

## Define request resolvers

```ts
const DemoCatalog = new Map<string, Part>([
  ["ink-black", { code: "ink-black", label: "Black ink" }],
  ["ink-blue", { code: "ink-blue", label: "Blue ink" }]
])

const PartResolver = RequestResolver.make<FindPart>((entries) => {
  Effect.log(`catalog batch size=${entries.length}`)
  for (const entry of entries) {
    Request.completeEffect(entry, () => {
      const part = DemoCatalog.get(entry.request.code)
      if (part === undefined) throw new PartMissing({ code: entry.request.code })
      return part
    })
  }
})
```

This executable fixture resolves a batch from a map. A backend resolver replaces the map lookup with one bulk operation, then matches the returned rows to entries. `Request.completeEffect` runs its program and completes the entry with that result, including an expected failure.

Complete every entry, including requests for absent rows. Use `Request.succeed` and `Request.fail` when you already have a value or error. `RequestResolver.fromEffect` is the simpler alternative when each entry needs an independent operation rather than a bulk call.

## Build queries from requests

```ts
function findPart(code: string) {
  return Effect.request(FindPart({ code }), PartResolver)
}
// Part!PartMissing | CatalogUnavailable!

function partLabel(code: string) {
  return findPart(code).label
}
```

`Effect.request` submits the request to its resolver and waits for that entry's answer. The returned success value is available directly in Effeck. The resolver's pending work, rather than the query function's syntax, determines which calls share a backend operation.

## Combine compatible requests into batches

```ts
function labels(codes: ReadonlyArray<string>) {
  return Effect.forEach(codes, (code) => partLabel(code), {
    concurrency: "unbounded"
  })
}
```

Requests using the same resolver and batch key can join the same pending batch. The default resolver delay yields before running the collected work, allowing requests started together to accumulate. Dependencies introduce later waves: a query that needs an earlier answer starts after that answer arrives.

`Effect.forEach` and `Effect.all` are sequential by default, so enable concurrency to collect independent requests. A bounded limit controls how many requests can be outstanding, but also limits batch size. Batching alone does not promise that repeated equal requests are cached; add a resolver cache when you need shared results.

## Control batch execution

```ts
const TunedResolver = pipe(
  PartResolver,
  RequestResolver.setDelay("8 millis"),
  RequestResolver.batchN(64)
)

function tunedLookup(code: string) {
  return Effect.request(FindPart({ code }), TunedResolver)
}
```

`setDelay` trades latency for a larger collection window, and `batchN` caps each backend batch. `setDelayEffect` takes a program for a custom delay, while `grouped` partitions requests by a key. Reuse the same configured resolver for callers that should share pending batches.

For full control, `RequestResolver.makeWith` accepts `batchKey`, `delay`, `collectWhile`, and `runAll`. Group by backend compatibility, such as destination or tenant. A batch size log like the one above helps verify that concurrent callers are actually sharing work.

## Supply resolver dependencies

```ts
abstract class CatalogApi {
  abstract fetchMany(codes: ReadonlyArray<string>): ReadonlyMap<string, Part>!CatalogUnavailable!
}

abstract class Parts {
  abstract find(code: string): Part!PartMissing | CatalogUnavailable!

  static readonly layer = Layer.effect(Parts, () => {
    const api = CatalogApi()
    const resolver = RequestResolver.make<FindPart>((entries) => {
      try {
        const codes = entries.map((entry) => entry.request.code)
        const rows = api.fetchMany(codes)
        for (const entry of entries) {
          Request.completeEffect(entry, () => {
            const part = rows.get(entry.request.code)
            if (part === undefined) throw new PartMissing({ code: entry.request.code })
            return part
          })
        }
      } catch (error) {
        for (const entry of entries) Request.fail(entry, error)
      }
    })
    return {
      find: (code) => Effect.request(FindPart({ code }), resolver)
    }
  })
}
// Parts.layer: Layer<Parts, never, CatalogApi>
```

Resolve `CatalogApi()` while building the layer, then close over its implementation. Resolver execution has no remaining service requirements, while building `Parts.layer` requires `CatalogApi`. All calls to the provided `Parts` implementation share the resolver built for that layer.

Match results by identifier rather than assuming the backend preserves request order. A backend failure completes every affected entry with `CatalogUnavailable`; a missing row completes only its entry with `PartMissing`. Supply a `CatalogApi` layer alongside this layer when wiring the application.

## Share and tune request caches

```ts
function cachedQueries() {
  const resolver = RequestResolver.withCache(PartResolver, {
    capacity: 128,
    strategy: "lru"
  })
  return {
    find: (code: string) => Effect.request(FindPart({ code }), resolver)
  }
}

function cachedExample() {
  const parts = cachedQueries()
  const first = parts.find("ink-blue")
  const again = parts.find("ink-blue")
  return [first, again]
}
```

`withCache` runs to allocate a cache and returns a wrapped resolver. Build it once in a service layer or another shared owner, then reuse it for every query that should share answers. Allocating it inside each lookup creates a fresh cache each time.

The cache is bounded by `capacity`, with `"lru"` eviction by default or `"fifo"` for insertion order. Completed expected failures are cached too; interrupted results are not. Entries have no time-based expiry, and cache hits depend on request equality.

```ts
function expiringLookup() {
  const cache = RequestResolver.asCache(PartResolver, {
    capacity: 128,
    timeToLive: () => "20 seconds"
  })
  const request = FindPart({ code: "ink-black" })
  const first = Cache.get(cache, request)
  Cache.invalidate(cache, request)
  return first
}
```

Use `asCache` when you need a first-class cache with expiry, refresh, and invalidation. Its `timeToLive` function receives the request's `Exit` and request value, so success and failure can have different lifetimes. Cache read requests whose answers can be reused; keep commands that must execute every time out of that cache.

## Coming from Effect

Requests and resolvers keep Effect 4's APIs. Resolver callbacks are ordinary Effeck functions, completion programs use `() => ...`, and cache allocation runs when `withCache` or `asCache` is called inside a function.

[Upstream Effect page](https://effect.website/docs/v4/batching)
