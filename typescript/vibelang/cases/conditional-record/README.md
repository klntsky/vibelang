# E-15: conditional records and compiler options

The reduction needs only one conditional property and a local variable:
`const cache = present ? { URL: "cache" } : {}; return cache`.
Its written return type is `Readonly<Record<string, string>>`.

The paired `a.vibe` and `reference.ts` sources use identical options. With
`strict` and `exactOptionalPropertyTypes` enabled, both compile. The runtime
checks both branches, and the declaration goldens keep the plain signature.
`conditional-record-environment` repeats the original Smithers helper shape,
checks credential removal and environment merging, and compares declarations.
`conditional-record-inexact.fail` disables only `exactOptionalPropertyTypes`;
both source languages must then report TS2322.

The relevant checker steps are `getWidenedTypeOfObjectLiteral` and
`getUndefinedProperty` in `internal/checker/checker.go`, followed by
`membersRelatedToIndexInfo` in `internal/checker/relater.go`. Widening gives
the empty branch an optional property whose type is `undefinedOrMissingType`.
The exact-optional option distinguishes an absent property from an explicitly
undefined one when checking the string index signature. No call, program
conversion, or Effect contract is involved in the reduction.

Smithers' `.smithers/flows/tsconfig.json` and `coding/flow-typecheck.ts` enable
`exactOptionalPropertyTypes`. At the finding's revision, the twin harness
inherits a config only when one exists directly in the package directory;
`coding` has none, so its generated config drops this parent option. A
compiler semantics change would hide that configuration mismatch.
