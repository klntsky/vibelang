import { Context, Effect, Layer } from "effect"

class Stock extends Context.Service<Stock, {
  count(sku: string): Effect.Effect<number>
}>()("warehouse/Stock") {}

const StockLive = Layer.succeed(Stock, { count: (sku) => Effect.succeed(sku.length * 6) })

export const available = (sku: string): Effect.Effect<boolean, never, Stock> => Effect.gen(function* () {
  const stock = yield* Stock
  return (yield* stock.count(sku)) >= 12
})

export const stocked = available("TEA").pipe(Effect.provide(StockLive))
