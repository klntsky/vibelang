# Batch b6, pass 2: Schema consistency

Reviewed all 20 pages in `src/schema/` against the full `STYLE.md` (including the rewrite updates), decisions D1-D54, design chapters 1-17 and 28, appendix C, and `notes/b6.md`. Kept the page headings, topic order, and upstream links. Repository edits are limited to this batch and this notes file; no Git or Jujutsu commands were run.

## Pages changed: 20

- `src/schema/085-introduction.md`: replaced the proposed trust convention with D22/D23/D37 rules and documented the v0 compiler package, `esc`, TypeScript 7.1 checker, and exact Effect version (D30).
- `src/schema/086-getting-started.md`: used trusted Result decoding, resolved guard/assertion narrowing with direct calls, explained synchronous defects and ES2004, and clarified Promise boundaries and Exit data (D22-D24, D37).
- `src/schema/087-basic-usage.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/088-filters.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/089-advanced-usage.md`: inferred declaration predicates from `instanceof`, used full contracts for recursive callbacks, kept codec service types separate from callback requirements, and updated the trust link (D9/D10, D22, D53).
- `src/schema/090-projections.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/091-transformations.md`: replaced a bare conversion return annotation with its full contract to avoid ES2003 and updated the trust link (D22, D53).
- `src/schema/092-annotations.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/093-canonical-codecs.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/094-error-messages.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/095-error-formatters.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/096-classes.md`: used a full recursive callback contract, clarified that error instances and `new` do not run programs, and updated the trust link (D9/D10, D22, D24, D53).
- `src/schema/097-default-constructors.md`: clarified that constructor exceptions are defects unless explicitly bounded and that `new` never runs a program; updated the trust link (D22, D24, D37).
- `src/schema/098-effect-data-types.md`: distinguished Config construction from reads and Exit construction/parsing from running the stored outcome; updated the trust link (D22, D24).
- `src/schema/099-standard-schema.md`: resolved the mixed synchronous/Promise validation proposal using the existing `Effect.tryPromise` boundary and `Promise.resolve`; updated the trust link (D22/D23).
- `src/schema/100-arbitrary.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/101-json-schema.md`: used a full recursive callback contract and updated the trust link (D22, D53).
- `src/schema/102-equivalence.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/103-formatter.md`: replaced the link to a proposed builder convention with the settled trusted-call rules (D22).
- `src/schema/104-compilers.md`: named and exported the source schema used by the AOT loader, changed the EffectScript consumer to import its `.es` source, and updated the trust link (D22, D31).

## Markers remaining: 0

No markers were kept. The three earlier markers are resolved:

- `085-introduction.md`, Proposed: D22 trusts Effect's own synchronous APIs; D23 supplies the explicit boundary and trusted-declaration rules for other TypeScript code.
- `086-getting-started.md`, Not designed yet: D22/D37 permit direct guards and assertions with normal TypeScript narrowing. A required boundary no longer separates the check from the narrowed variable. Assertion exceptions remain defects.
- `099-standard-schema.md`, Proposed: D23 permits plain calls inside `Effect.tryPromise`; `Promise.resolve` supplies the Promise shape that boundary expects for either Standard Schema result form.

The owner's open partial-annotation spelling and language-name questions do not occur as markers in this batch. Examples use full contracts where annotated and retain the current EffectScript name.

## Spec issue

- Appendix C3's service-key row still says duplicate keys are ES3005. D53 and the service and tooling chapters define ES3001 for duplicate keys and ES3005 for generic service classes. No schema page repeats the incorrect code.

## Validation

- Checked 185 qualified API references against the exports of local `effect@4.0.5` source modules, including the Schema compiler submodules; all names exist. Checked relevant signatures and behavior in `Schema`, `SchemaParser`, `SchemaGetter`, `SchemaIssue`, `Config`, and the Standard Schema and AOT implementations. The retained platform-node build helper was not executed.
- Type-checked Schema declarations from 19 pages against local TypeScript 5.9.3 with `strict` and `exactOptionalPropertyTypes`, adapting EffectScript callbacks to the Effect/plain forms expected by TypeScript. Separately checked direct guard/assertion narrowing and inferred declaration predicates. These checks validate API shapes, not end-to-end EffectScript compilation.
- Ran six passing local runtime probes covering direct guards/assertions, synchronous parser defects versus explicit expected-error boundaries, Config and parsed Exit data, error-instance construction, synchronous/asynchronous Standard Schema results, and recursive codec callbacks.
- Reviewed the changed text against the call, catch, service, runner, and callback rules. Retained valid explicit `Effect.try` examples: they deliberately give synchronous exceptions an expected error type, even though trusted calls do not require a boundary. No producer, stream, program-collection, or Scope example in this batch needed a rule change.
- Temporary snapshots, API checks, and probes are under `/tmp/effectscript-b6-pass2-original` and `/tmp/effectscript-b6-pass2-check`.
