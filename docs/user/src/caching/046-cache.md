# Cache

A `Cache` stores lookup outcomes by key and shares in-progress lookups across callers. Use it when many requests need the same data, with capacity and expiration controlling how long that sharing lasts.

## Building a lookup cache

```effeck
error MissingProduct {
  sku: string
}

service Products {
  price(sku: string): number!MissingProduct
}

function makePrices() {
  return Cache.make({
    capacity: 200,
    timeToLive: "2 minutes",
    lookup: (sku: string) => Products.price(sku)
  })
}
// Cache.Cache<string, number, MissingProduct>!!Products
```

The default cache captures lookup services when it is created. `Cache.get(cache, sku)` then either reads an entry or calls the captured lookup. Creating a cache per request prevents sharing across requests; allocate it in a service layer when the cache belongs to the application.

## Sharing concurrent requests

```effeck
function concurrentLookup() {
  let calls = 0
  const cache = Cache.make({
    capacity: 10,
    lookup: (key: string) => {
      calls++
      sleep("30 millis")
      return key.toUpperCase()
    }
  })
  const values = all([
    Cache.get(cache, "north"),
    Cache.get(cache, "north"),
    Cache.get(cache, "north")
  ], { concurrency: 3 })
  return { values, calls }
}
// Returns { values: ["NORTH", "NORTH", "NORTH"], calls: 1 }.
```

Requests for the same missing key wait on one computation without blocking the JavaScript thread. A failed lookup shares its failure and caches it too. An interrupted lookup is removed so a later call can try again.

## Setting capacity and expiration

```effeck
function makePermanentNames() {
  return Cache.make({
    capacity: 50,
    timeToLive: Duration.infinity,
    lookup: (id: string) => `item-${id}`
  })
}
```

Capacity is a bound on entries, not bytes, and recently used entries take priority over older ones when eviction is needed. A finite TTL prevents expired outcomes from being reused and causes the next lookup to reload them. Infinite TTL disables time expiration while leaving capacity eviction active.

## Inspecting cache state

```effeck
function inspect() {
  const cache = Cache.make({ capacity: 5, lookup: (key: string) => key.length })
  Cache.get(cache, "paper")
  return {
    present: Cache.has(cache, "paper"),
    size: Cache.size(cache),
    keys: Array.from(Cache.keys(cache)),
    entries: Array.from(Cache.entries(cache))
  }
}
```

`has`, `size`, `keys`, and `entries` inspect state without loading missing keys. `entries` includes successful resolved values, excluding failures and unfinished lookups. These observations can change immediately under concurrent access, so do not use `has` followed by `get` as a synchronization protocol.

## Invalidating and refreshing entries

```effeck
function refreshExample() {
  let version = 0
  const cache = Cache.make({
    capacity: 5,
    timeToLive: "1 minute",
    lookup: (key: string) => `${key}:${++version}`
  })
  const first = Cache.get(cache, "profile")
  const refreshed = Cache.refresh(cache, "profile")
  Cache.invalidate(cache, "profile")
  const reloaded = Cache.get(cache, "profile")
  Cache.invalidateAll(cache)
  return { first, refreshed, reloaded }
}
```

`refresh` forces a new lookup and returns the refreshed value. An existing valid value remains available to readers while its replacement loads. `invalidate` removes one entry, while `invalidateAll` removes the cache's entries; the next `get` loads again. Use refresh when the caller needs recomputation now and invalidation when a later reader should trigger it.
