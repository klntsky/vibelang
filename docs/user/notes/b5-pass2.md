# Batch b5, pass 2 notes

Reviewed all 21 pages against STYLE.md (including its final update section), DECISIONS.md D1-D54, specification chapters 1-17 and 28, appendix C, and the earlier b5 notes. Changed **8 pages**. **2 Proposed markers remain; 0 Not designed yet markers remain.** No version-control commands were run. Only the assigned pages and this notes file were edited.

## Pages changed

- `src/data-types/072-chunk.md`: corrected the claim that `fromIterable` copies arrays; local 4.0.5 reuses them, so the ownership guidance now calls for an explicit copy.
- `src/data-types/076-result.md`: resolved the generator proposal using D26/ES1002 and documented D23's trusted-helper alternative to an explicit boundary.
- `src/data-types/077-exit.md`: corrected constructors, examples, the inferred contract, and migration advice so Exit values remain data until called (D24).
- `src/data-types/079-option.md`: resolved the generator proposal using D26/ES1002, documented the trusted-helper alternative (D23), and clarified getOrThrow defects and dead catch warnings (D37).
- `src/behaviour/083-equivalence.md`: corrected the strict-equality claim; local `Equivalence.Number` equates NaN with itself and `Equivalence.Date` delegates to that numeric relation.
- `src/platform/105-introduction.md`: named and installed `@effectscript/runtime` and `@effectscript/compiler`, and replaced unspecified build guidance with `esc` and `esc --noEmit` (D30).
- `src/platform/107-path.md`: removed unnecessary boundaries around trusted Path methods and explained service-member requirements and implementation value reads (D22, D41).
- `src/platform/109-runtime.md`: stated permitted runner locations, ES1005 inside fiber-run code, and the `.es` testing rule (D38).

The other 13 pages needed no edits. Existing `Effect.service(key)` calls remain valid; D24/D41 also permit calling the key or its members directly. There are no collection-of-programs or producer/Stream interoperability claims in these pages that contradict D49 or D26. No page imports Scope or uses obsolete tooling names.

## Markers kept

- `src/trait/081-equal.md`, "Implementing the equality protocol": kept the Proposed marker because chapters 15-16 define plain function variants and passing function values into synchronous callbacks, but do not specify how an EffectScript method implementing a TypeScript interface is exposed for synchronous protocol dispatch. Appendix C19 says Equal/Hash are unchanged without settling that method emission rule.
- `src/trait/082-hash.md`, "Implementing the hashing protocol": kept the Proposed marker for the same unresolved protocol-method emission rule; Hash calls `[Hash.symbol]()` synchronously, so an Effect-returning method would not satisfy it.

The Option.gen and Result.gen markers were removed: D26 and the TypeScript-interop chapter explicitly settle both. D57 and D58 close the former partial-annotation and language-name questions; no marker is needed for either.

## Specification issues and unclear points

- Protocol methods remain underspecified as described above. Please state whether eligible methods implementing plain TypeScript interfaces retain synchronous method slots, and how their Effect form is exposed to other callers. The retained proposals require no Effect operations and E = never.
- Appendix C3's "Service keys" row calls duplicate service keys ES3005. D53 and chapters 4/28 define ES3001 for duplicate keys; ES3005 is a generic service class.
- The mixed-cause rows in chapters 3 and 17 say their expected failures are in E. D46 and chapter 14 define E only for causes consisting entirely of expected failures and explicitly allow mixed causes to contain Fail reasons outside E. The tables need that qualification. The batch's Cause page already preserves mixed causes and does not make the conflicting E claim.
- The older b5 notes say Exit constructors run and that Path needs boundaries. Those historical conclusions are superseded by D24 and D22/D41; the earlier notes were preserved as requested by the edit scope.

## API and example checks

- Confirmed the installed core package is Effect 4.0.5. Audited 271 distinct qualified core references against exports in `experiments/node_modules/effect/src`, plus bare API names in prose, nested Data helpers, and FileSystem/Path/Terminal members against their source declarations. No missing core API names remained.
- Inspected signatures and implementations for the changed ownership/equality guidance, Exit constructors and replay, Path string and URL methods, FileSystem test overrides and file stat, terminal Effect-valued members, Logger layers/file batching, Random's inclusive range, and Runtime teardown behavior.
- Parsed all 117 TypeScript code fences without syntax diagnostics. Executed 62 isolated data examples without errors, plus focused assertions for array ownership, NaN equivalence, Exit construction/replay, Option extraction, decimal arithmetic, zoned formatting, signed durations, structural hashes/sets, ordering, and the provided POSIX Path service.
- These checks validate local core APIs and example syntax, not end-to-end EffectScript compilation or the proposed protocol adaptation. Platform adapter packages and `@effect/vitest` are not installed in the experiments dependency tree and are not part of `effect/src`; their existing names were retained from the earlier batch's upstream verification. No upstream prose or code was copied.
