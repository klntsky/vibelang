# Batch b2, pass 2 notes

Reviewed all 18 assigned pages against STYLE (including its rewrite updates), D1-D54, spec chapters 1-17 and 28, appendix C, and the earlier b2 notes. Changed 14 pages while retaining their titles, section headings, and upstream links.

## Pages changed

- `src/error-management/006-two-error-types.md`: added trusted-contract failure sources, first-failure catch behavior (D28), and the precise meaning of E for mixed causes (D46).
- `src/error-management/014-expected-errors.md`: documented trusted calls and exported `@throws` classes (D22/D23), and distinguished source catch's guard from Effect recovery and result/option conversion on mixed causes (D13/D28).
- `src/error-management/015-unexpected-errors.md`: clarified unlisted exceptions, typed alternatives and ES2004 (D37), Exit data (D24), and defect recovery's behavior on mixed causes.
- `src/error-management/016-fallback.md`: corrected equivalence claims about source catch, Effect recovery combinators, and `firstSuccessOf` on mixed causes.
- `src/error-management/017-matching.md`: corrected claims that `match` and `ignore` always preserve defects and interruption when an expected failure accompanies them.
- `src/error-management/021-error-accumulation.md`: qualified validation and partitioning's mixed-cause behavior, confirmed through their local `result` implementation.
- `src/error-management/022-error-channel-operations.md`: clarified mixed-cause behavior of mapping and `flip` instead of promising unconditional defect preservation.
- `src/error-management/023-parallel-and-sequential-errors.md`: added D28's first-failure catch rule and cause-level inspection for all failures.
- `src/error-management/024-yieldable-errors.md`: demonstrated error instances as data until called (D24), and clarified that producers convert to Streams rather than sequence generator-style Effects (D26).
- `src/requirements-management/025-services.md`: resolved the shape-extraction proposal with D45's interface/key emission; added value and Effect-valued member rules, plain member trust and name collisions (D41); corrected partial `wire` graph behavior (D27).
- `src/requirements-management/026-default-services.md`: demonstrated Config composition before execution (D24) and `static readonly default` services backed by `Context.Reference` (D40).
- `src/requirements-management/027-layers.md`: corrected partial graph and opaque-output `wire` diagnostics (D27/D53), and demonstrated `static make()` with a default static layer (D40).
- `src/resource-management/029-introduction.md`: distinguished typed `finally` cleanup from infallible `ensuring` (D46), retained cleanup failure under abandonment (D15), and documented Scope's prelude entry and acquisition masking (D25/D48).
- `src/resource-management/030-scope.md`: removed the Scope import and explained its module/type prelude names (D48), interruptible acquisition (D25), and fork-only block ownership (D47).

The retrying, timing-out, sandboxing, and layer-memoization pages needed no changes. Existing program collections agree with D49. This batch has no runner call sites or tooling names needing D30-D38 corrections.

## Markers

Remaining in the 18 pages: **0 Proposed**, **0 Not designed yet**.

Removed the sole Proposed block from `025-services.md`: D45 now specifies an implementation interface plus a key class, and local `Context.Service.Shape<T>` extracts the shape from `Context.Key`. No marker was retained. Neither owner-only open question, partial-annotation spelling or the language's name, appears as a marker in this batch.

## Spec issues and ambiguities

- Appendix C3's service-key row assigns duplicate keys ES3005. D53, chapter 4, and chapter 28 assign them ES3001; ES3005 is a generic service class. The pages follow D53.
- The mixed-exit rows in chapters 3 and 17 say the expected failures are in E without qualifying that claim. D46 and chapter 14 define E only for causes made entirely of expected failures; mixed causes can retain Fail values outside E. The error overview now states that distinction.
- Chapter 5's replacement table and appendix C2 summarize Effect recovery/conversion operators as source catch forms without carrying every mixed-cause distinction. Chapter 17 explicitly distinguishes `Effect.catch`, but local 4.0.5 source and runtime checks show the same caution applies to `result`, `option`, `match`, `ignore`, `firstSuccessOf`, `flip`, and validation/partitioning, with similar failure-selection behavior in mapping operators. The pages preserve the specified source-catch guard and describe these library calls' actual behavior. No language-design marker is needed because the spec keeps Effect's APIs unchanged and the local implementation answers the API question.

## Verification

- Checked Effect API names against local `effect@4.0.5` source: 131 qualified module exports, plus `Context.Service.Shape`, unqualified eager mapping variants and reason readers, and the `Path.join` and `Terminal.readLine` service members.
- Inspected relevant signatures and implementations for recovery, validation, Config, service keys/references, Layer factories and memo maps, Scope provision, and resource release.
- Ran inline local runtime checks for mixed-cause recovery/conversion, validation/partitioning, and retry. Recovery/conversion can consume accompanying defects; retry preserves the mixed cause.
- Parsed all 87 TypeScript-fenced snippets with the installed TypeScript parser: zero syntax diagnostics. All fences balance, all original headings and upstream links remain, and no forbidden dash characters or design markers remain.
- These checks do not constitute compiling the examples with a completed EffectScript compiler.

Only the assigned pages and this notes file were edited. No git or jj commands were run.
