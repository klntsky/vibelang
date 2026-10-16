# Getting Started

These guides teach EffectScript one topic at a time and also serve as a reference while you build. Start with [Welcome](001-onboarding.md), or jump to the subject you need.

## Guides and reference material

```ts
function main() {
  const label = "first program"
  Effect.log(label)
  return label.length
}
// Effect<number, never, never>
```

Read [The Effect Type](getting-started/003-the-effect-type.md) to understand this function's contract, [Creating Effects](getting-started/005-creating-effects.md) to add failures, and [Running Effects](getting-started/009-running-effects.md) to execute it at an application boundary. Each guide explains a use case. The [Effect 4 API reference](https://effect.website/docs/v4/api) lists library signatures, whose program arguments become zero-argument functions in EffectScript.

## Documentation versions

These pages target Effect **4.0.5** and the language decisions recorded with this repository. When opening upstream documentation, select version 4. Effect 3 examples use a different library version and do not establish which APIs are available here.

The upstream site also documents newer changes. Check a signature against your installed package when it differs from an example on these pages.

## Finding a subject

The directories group guides by task:

- [Error management](error-management/006-two-error-types.md): failures, recovery, retry, and timeouts.
- [Concurrency](concurrency/007-basic-concurrency.md): bounded parallel work and fiber lifetimes.
- [Requirements](requirements-management/025-services.md): services and layers.
- [Resources](resource-management/029-introduction.md): acquisition and cleanup.
- [Schema](schema/085-introduction.md): decoding and validating external data.
- [Observability](observability/031-logging.md): logs, metrics, spans, and fibers.
- [Streams](stream/053-introduction.md): incremental processing with backpressure.

Use each page's headings to find a specific operation. In a Markdown reader, follow the outline or search for an API name. The numbered file names preserve the learning sequence without requiring you to read every page in order.

## Using documentation with language models

Provide the relevant EffectScript guide alongside the Effect API signature. State that every function is an Effect, calls run, and programs passed to operators are functions such as `() => main()`. Include your error and service declarations so generated code has enough information to preserve the contract.

Use compiler feedback and focused tests to check the result. Effect's TypeScript tooling applies to `.ts` integration code; the [Devtools](getting-started/012-devtools.md) page explains EffectScript's compiler, language server, linting, and debugging setup.

## Community entry points

Use [Effect Discord](https://discord.gg/effect-ts) for ecosystem help and [Effect on GitHub](https://github.com/Effect-TS) for library sources and issues. Say whether a reproduction is `.es` or `.ts`, and include the installed version. For language-specific behavior, include the smallest EffectScript function that exhibits it.

[Upstream Effect page](https://effect.website/docs/v4/getting-started)
