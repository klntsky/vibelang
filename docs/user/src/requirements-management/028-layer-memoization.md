# Layer Memoization

Effect shares repeated uses of one layer value within a build graph, and VibeLang keeps this: reuse a layer value when consumers should share its initialization and resources.

[Upstream page](https://effect.website/docs/v4/requirements-management/layer-memoization)

## Sharing acquisition across a graph

```vibe
// run: Console.log(JSON.stringify(documentLabels()))
service Prefix {
  value: string
}
service Heading {
  value: string
}
service Footer {
  value: string
}

const PrefixLive = Layer.effect(Prefix) {
  Console.log("Prefix initialized")
  return {
    value: "Monthly"
  }
}
const HeadingLive = Layer.effect(Heading) {
  const prefix = Prefix()
  return {
    value: `${prefix.value} report`
  }
}
const FooterLive = Layer.effect(Footer) {
  const prefix = Prefix()
  return {
    value: `${prefix.value} archive`
  }
}

const Shared = Layer.merge(
  Layer.provide(HeadingLive, PrefixLive),
  Layer.provide(FooterLive, PrefixLive)
)

function documentLabels() {
  provide Shared { return [Heading().value, Footer().value] }
}
// Output: Prefix initialized
// Output: ["Monthly report","Monthly archive"]
```

Both branches reference `PrefixLive`, so its initialization logs once per `documentLabels` call. Memoization uses layer object identity, not the output service type or structural equivalence; when sharing matters, call a layer-building factory once and reuse its value.

Sharing lasts for the graph's managed lifetime, not process-wide. Scoped acquisitions stay open while the graph uses them and release when the owning lifetime ends.

## Requesting fresh acquisition

```vibe
// run: Console.log(JSON.stringify(isolatedLabels()))
const Separate = Layer.merge(
  Layer.provide(HeadingLive, Layer.fresh(PrefixLive)),
  Layer.provide(FooterLive, Layer.fresh(PrefixLive))
)

function isolatedLabels() {
  provide Separate { return [Heading().value, Footer().value] }
}
// Output: Prefix initialized
// Output: Prefix initialized
// Output: ["Monthly report","Monthly archive"]
```

`Layer.fresh` isolates acquisition from the surrounding graph's memoization: each fresh branch acquires its own prefix implementation. Use it when separate state or resource ownership is intended.

## Local provision boundaries

```vibe
// run: Console.log(JSON.stringify(twoLocalReads())); Console.log(JSON.stringify(oneSharedReadBoundary()))
function twoLocalReads() {
  const first = Effect.provide(() => Prefix().value, PrefixLive)
  const second = Effect.provide(() => Prefix().value, PrefixLive)
  return [first, second]
}

function oneSharedReadBoundary() {
  provide PrefixLive {
    const first = Prefix().value
    const second = Prefix().value
    return [first, second]
  }
}
// Output: Prefix initialized
// Output: Prefix initialized
// Output: ["Monthly","Monthly"]
// Output: Prefix initialized
// Output: ["Monthly","Monthly"]
```

Each `Effect.provide` starts its own build boundary, so `twoLocalReads` builds twice; `oneSharedReadBoundary` builds once and shares within it. Provide the application layer around the entry point; use `ManagedRuntime` when several entry points need a common runtime lifetime.

## Controlling memoization explicitly

```vibe
// run: Console.log(JSON.stringify(manualSharing()))
function manualSharing() {
  return Effect.scoped {
    const memoMap = Layer.makeMemoMap()
    const scope = Effect.scope()
    const first = Layer.buildWithMemoMap(PrefixLive, memoMap, scope)
    const second = Layer.buildWithMemoMap(PrefixLive, memoMap, scope)
    return [
      Effect.provide(() => Prefix().value, first),
      Effect.provide(() => Prefix().value, second)
    ]
  }
}
// Output: Prefix initialized
// Output: ["Monthly","Monthly"]
```

`Layer.makeMemoMap` is an Effect value; call it to get the map. Building one layer twice with the same map reuses its acquisition, and providing the resulting contexts doesn't rebuild it. The explicit scope owns resource release; both contexts are consumed before it closes.

A memo map controls sharing; a scope controls lifetime. Align both with the intended application or test boundary, and don't retain resource-owning implementations after their scopes close.
