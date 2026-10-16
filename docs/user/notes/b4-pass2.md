# Batch b4, pass 2

Read STYLE.md (including its rewrite updates), DECISIONS.md D1-D54, the spec's chapters 1-17 and 28 and all of appendix C, and b4.md. Reviewed all 17 assigned pages; changed 9. Kept the original page names, headings, and topic order. No version-control commands were run.

## Pages changed

- `src/stream/053-introduction.md`: replaced both markers with D22's trusted-library rules and D26's producer conversion, Stream iteration, demand, and cleanup rules; added a short example using a producer with a Stream operator and `for...of`.
- `src/stream/054-creating.md`: removed the obsolete proposed-trust wording from the introduction's cross-reference.
- `src/stream/057-operations.md`: corrected the claim that `for...of` cannot consume a Stream, retaining manual-pull and async-iterable guidance.
- `src/sink/059-introduction.md`: resolved trusted Sink builders under D22 and documented producer inputs to `Stream.run` under D26.
- `src/sink/060-creating.md`: removed the obsolete proposed-trust wording from its Sink introduction link.
- `src/code-style/064-guidelines.md`: stated D38's runner locations, ES1005, and the distinction between `.es` Effect tests and `.ts` Promise-style tests.
- `src/code-style/065-dual.md`: replaced the proposed-trust reference with the settled trust rule for builders and returned stages.
- `src/code-style/066-branded-types.md`: resolved Brand trust, removed unnecessary nominal-constructor and inspection wrappers, distinguished defect exceptions from expected validation errors, and documented `Effect.fromResult`; retained explicit mappings to domain errors.
- `src/code-style/067-pattern-matching.md`: resolved Match trust and called the returned matcher directly; kept synchronous-handler restrictions.

The other 8 pages needed no change. In particular, the testing page already uses `it.effect` and block-owned forks, resource pages correctly distinguish caller-owned acquisitions from `using`, and the collection examples already use the D49 program conversion. Existing examples construct error instances as data, and no example uses obsolete Config or Exit execution rules, service statics, Scope imports, or tooling names.

## Markers remaining

**0 Proposed; 0 Not designed yet.** Removed all five earlier markers: Stream, Sink, Brand, and Match trust are settled by D22 and the standard-library/feature-map guidance; producer and Stream interoperability is settled by D26 and the producers chapter. No owner-open partial-annotation spelling or language-name question occurs in these pages, so there is no marker to retain for either.

## Spec issues and clarifications

- Appendix C3's service-key row says duplicate keys are ES3005. D53, the services chapter, and chapter 28 say ES3001; ES3005 is a generic service class. No batch page repeats the incorrect code.
- Appendix C14 groups `Stream.scoped` and `Stream.ensuring` under “Programs as functions.” This is too broad: Effect 4.0.5's `Stream.scoped` takes a Stream, while `Stream.ensuring` takes a Stream plus an Effect finalizer. Kept the pages' correct Stream argument for `scoped` and program callback for `ensuring`.
- D22's wording about library exports is terse about functions returned by those exports. The standard-library HTTP example explicitly calls a returned function trusted, appendix C17 treats Brand and Match as unchanged, and their callable signatures are declared in Effect's source. Applied that treatment to returned Brand constructors, their methods, Match finalizers, and dual stages; no unresolved marker remains.
- The older opening prelude lists in STYLE.md omit Scope, but its final rewrite updates, D48, and the spec's prelude include it. Followed the final update; no batch contract needed a Scope spelling correction.

## Verification

- Checked qualified Effect names against exported declarations in `experiments/node_modules/effect/src`, including export aliases (`catch`, `let`, `await`, `null`, `undefined`), nested `Brand.Brand.FromConstructor`, and `TestClock`'s `effect/testing` export. Also checked the Stream and Sink variant names mentioned without qualification in prose and relevant callback/resource signatures. The installed package reports version 4.0.5.
- Platform runtime imports and `it.effect` are supplied by separate packages outside `effect/src`; followed the specification and recorded that local source-tree verification does not cover those packages.
- Parsed all 98 `ts` examples with the installed TypeScript parser: no syntax diagnostics. Checked fence balance, local Markdown links, and prohibited dash characters: no problems.
- Ran 11 assertions against the installed Effect runtime for direct Brand constructors, invalid-input defects, Brand `option`/`result`/`is`, typed `Effect.fromResult` validation, and direct returned Match execution: all passed. These checks validate API and runtime behavior, not compilation of the documentation as a complete `.es` program.
- Compared final page text with the captured originals to verify the 9-page count and unchanged headings. Wrote only those pages and this notes file.

Final totals: **9 pages changed; 0 markers remaining.**
