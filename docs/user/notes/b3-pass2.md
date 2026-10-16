# Batch b3, pass 2 notes

Reviewed all 20 pages in observability, scheduling, state-management, caching, and concurrency against STYLE.md, D1-D54, spec chapters 1-17 and 28, appendix C, and the earlier b3 notes. Changed 3 pages, preserving their titles, headings, and existing example structure.

## Pages changed

- `src/observability/031-logging.md`: Build the log-level Config as data and call it inside the layer builder to read and validate the setting (D24).
- `src/observability/033-tracing.md`: Call the endpoint Config inside the layer builder (D24); replace the SDK marker with the settled layer, trusted-call, and synchronous-callback rules (D9, D22, D23, D39).
- `src/state-management/043-subscriptionref.md`: Correct the claim that an Effect Stream cannot be consumed with `for...of`; document loop-exit closure of the stream and subscription (D26).

## Markers kept

None. Remaining markers: 0 Proposed, 0 Not designed yet. The SDK marker's language-policy questions are answered by the current spec: a bridge supplies a Layer, foreign calls need a declared boundary or trust, and synchronous callbacks cannot perform Effect operations. The page keeps its locally verified OTLP example rather than adding an unverified bridge API. No page in this batch raises the owner's partial-annotation or language-name questions.

## Spec issues and ambiguities

- Appendix C5 and chapter 13 mention `NodeSdk.layer`, but the checked `effect/src` tree has no `NodeSdk` module. The spec does not identify a versioned SDK bridge or an API for attaching an existing tracer provider. The tracing page documents the settled integration rules and retains `effect/observability/OtlpTracer` for its executable example; the exact external bridge remains an API/dependency documentation gap.
- Appendix C3 says duplicate service keys are ES3005. D53 and chapters 4 and 28 assign duplicate keys ES3001; ES3005 is a generic service class. No batch page cites the incorrect code.
- Appendix C1's prelude list omits `Scope`, which D48 and chapter 1 include. This batch has no explicit `Scope` import or obsolete `Scope.Scope` contract to repair.
- The failure tables in chapters 3 and 17 describe the expected failures of a mixed cause as being in E. D46 and chapter 14 define E through causes made only of expected failures and explicitly allow mixed causes to contain failures outside E. The tables need that qualification; the batch's concurrency prose correctly says a mixed cause propagates intact through ordinary `catch`.

## Verification

- Checked referenced module members and the additional API names mentioned without qualification against exports in `experiments/node_modules/effect/src`, including reserved-name export aliases such as `await` and `while`; the installed package declares version 4.0.5.
- Checked the changed examples against the local Config, Layer, Stream, and SubscriptionRef signatures and implementation, and reviewed the batch's existing program arguments, service contracts, resource ownership, and expected-failure handling against the new decisions.
- All 106 TypeScript-fenced examples across the 20 pages parse with the local TypeScript parser; code fences balance, changed-page headings are preserved, and no proposal or undesigned markers remain. Parsing checks syntax only, not EffectScript semantics; the examples were not compiled or executed as EffectScript.
- No version-control commands were run. Edits are limited to the three pages above and this notes file.
