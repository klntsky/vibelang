# Batch b1, pass 2

Read STYLE.md in full, DECISIONS.md D1-D54, design chapters 1-17 and 28, all of appendix C, and the earlier b1.md notes. Reviewed all five root pages and all nine getting-started pages. Changed 12 pages and resolved 10 of the earlier 11 markers while preserving the pages' headings and topic order.

## Pages changed

- `src/001-onboarding.md`: removed the outdated suggestion that compiler setup remains unresolved.
- `src/013-getting-started.md`: replaced the reference to tooling still being designed with the settled tooling topics.
- `src/035-configuration.md`: applied D24 to every configuration read, moved descriptor composition examples into `.es`, removed the old integration-gap marker, and corrected whole-group defaults against Effect 4.0.5.
- `src/036-runtime.md`: stated the D38 runner boundary and allowed plain framework callbacks registered from `.es` top level; retained the valid static service layer and managed-runtime examples.
- `src/getting-started/003-the-effect-type.md`: clarified the all-three-or-none annotation rule and ES2003, retained the owner's partial-annotation question, and explained structured Effect subtypes as data.
- `src/getting-started/004-installation.md`: replaced all three obsolete markers with the compiler/runtime packages, supported versions, `esc` commands, Vite plugin, source imports, maps, and hot reload.
- `src/getting-started/005-creating-effects.md`: added Effect-package trust and JSDoc contracts, expected-only catch and ES2004, the top-level foreign-call exception, and D43 callback boundaries with Exit data and resource-race handling.
- `src/getting-started/008-importing-effect.md`: corrected compiler distribution guidance, added Scope's module/type prelude meanings, and explained trusted Effect data calls.
- `src/getting-started/009-running-effects.md`: made D38's allowed runner locations, ES1005, and `.es` test boundaries explicit.
- `src/getting-started/010-using-generators.md`: called the Config descriptor to read it, added first-failure catch behavior, and explained producer/Stream conversion and direct Stream iteration.
- `src/getting-started/011-building-pipelines.md`: explained conversion of computed collections of programs without running their bodies in synchronous array callbacks.
- `src/getting-started/012-devtools.md`: resolved all six markers with the language server, editor features, diagnostics configuration, compiler/lint ownership, Vite Plus integration, and source-map/debugger behavior.

`src/044-batching.md` and `src/getting-started/002-why-effect.md` needed no changes. Their service layers, program callbacks, requests, resolver completion, concurrency, and cache ownership remain consistent with the decisions and installed APIs. No page in this batch incorrectly used service statics, default references, or value-member access.

## Markers kept

- `src/getting-started/003-the-effect-type.md`: kept **Not designed yet.** for partial-annotation spelling because O7 is still the owner's call. The surrounding prose states the settled interim rule: all three channels or no annotation.

No Proposed markers remain. No language-name marker existed in this batch; O9 remains open and tooling package names follow the current spec.

## Spec issues and ambiguities

- Appendix C3 says duplicate service keys are ES3005. D53, chapter 4, and chapter 28 assign duplicates ES3001 and generic service classes ES3005. These pages use the decision log's codes.
- Chapters 3 and 17 have incomplete mixed-cause table cells: “The failure's type is” and “Its failures are”. D46 and chapter 14 state the precise rule: E describes failures in causes consisting only of expected failures; mixed causes can retain Fail reasons outside E. The pages explain that the whole mixed cause bypasses catch without repeating the incomplete cells.
- D43 and chapters 6/12 describe Effect.callback's resume argument as an Exit, but Effect.ts:1853-1860 declares it as Effect<A, E, R>. Exit is a valid subtype and the correct data spelling in `.es`; the retained `.ts` adapter's Effect.succeed/Effect.fail arguments are also valid upstream. No upstream signature was narrowed in these pages.
- Chapter 15's explanation that v0 does no exception bridging across JavaScript frames needs qualification beside D54 and the chapter's readPort example, which explicitly convert listed exceptions inside a JavaScript catch. The outward E = never restriction and selective local catch behavior are clear; the blanket explanation is broader than that behavior.

## Validation

- Checked 123 distinct qualified Effect API names against exports parsed from `experiments/node_modules/effect/src`, including newly mentioned Exit, Option, Schema, Stream, and catch APIs. Also read the relevant Config/ConfigProvider, Request/RequestResolver, ManagedRuntime, callback, Data.TaggedError, and DevTools declarations and implementations, including unqualified API mentions and instance methods.
- Confirmed the installed package is Effect 4.0.5. Eleven runtime assertions passed for Config composition and reads, missing versus invalid defaults, group defaults, option/fallback, arrays/records, schema validation, redacted values, and callback completion with Exit.
- Parsed all 80 TypeScript/EffectScript code fences with the installed TypeScript parser, selecting TSX for Counter.tsx; no syntax errors. All page fences are balanced, local Markdown links resolve, and no forbidden dash punctuation remains.
- These are syntax and upstream API/behavior checks, not full EffectScript compiler or editor/build integration tests. Tooling names and promised behavior follow D30-D36 and chapter 28; no packages were installed and no external documentation prose or code was copied.

Final totals: **12 pages changed; 1 marker remaining (0 Proposed, 1 Not designed yet).**
