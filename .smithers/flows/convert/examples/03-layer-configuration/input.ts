import { Config, Context, Effect, Layer } from "effect"

class Banner extends Context.Service<Banner, {
  text(topic: string): Effect.Effect<string>
}>()("news/Banner") {}

export const BannerLive = Layer.effect(Banner, Effect.gen(function* () {
  const edition = yield* Config.String("EDITION")
  yield* Effect.log(`Preparing ${edition}`)
  return Banner.of({ text: (topic) => Effect.succeed(`${edition}: ${topic}`) })
}))
