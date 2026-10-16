# TC39 proposals that could give syntax to Effect features: status

Fetched 2026-10-16 (UTC).

Only one proposal in scope is finished: Explicit Resource Management (`using`, `DisposableStack`) met its Stage 4 conditions on 2026-05-19 and is listed for ES2027. Import defer and await dictionary (`Promise.allKeyed`) are at Stage 3. The Hack-style pipeline operator has been at Stage 2 since 2021-08-31, its topic token is still undecided, and the January 2026 committee review called it "somewhat stuck". Throw expressions, AsyncContext, extractors, async iterator helpers, discard bindings, the error `code` property, and Composites (since 2026-09-30) are at Stage 2. Pattern matching, partial application, `do` expressions, call-this, extensions, concurrency control, the Cancellation API, and Abort Controller (since 2026-09-30) are at Stage 1. Function.pipe/flow, generator arrow functions, and Records & Tuples are inactive. The "try operator" (formerly "safe assignment operator") is not a TC39 proposal. Its own README calls it "Stage 0" and says it has no champion.

**Method.** Every stage below was read from a source fetched on 2026-10-16. The sources are the tc39/proposals lists at commit `07002d4` (2026-10-14), the tc39/proposals commit log, TC39 meeting notes (`tc39/notes`, newest published meeting 2026-07), the TC39 agendas (`tc39/agendas`, 2026/09.md), each proposal's repository, the Babel docs (`babel/website` at `eb2e026`, 2026-10-05), and the TypeScript issue tracker. Each claim is labeled as follows:

- **Unlabeled:** read in a cited source.
- **UNVERIFIED:** not confirmed in any fetched source.
- **SUPPLEMENTARY:** taken from an engine repository's commit log, which is outside the requested source list. These claims are used only for engine shipping status.

**The 2026-09 meeting.** The 116th meeting (Tokyo, 2026-09-29 to 2026-10-01) has no published notes. Its outcomes below come from tc39/proposals commit messages of the form "per 2026.09.30 TC39" and from the agenda. What was said at that meeting is UNVERIFIED.

**Stale READMEs.** Some proposal READMEs lag behind tc39/proposals. Where they disagree, this document uses tc39/proposals and the notes, and says so.

---

## Summary

```
proposal                          area          stage      last stage change
--------------------------------  ------------  ---------  ------------------------------------------
Explicit Resource Management      resources     4          2026-05-19 (conditional Stage 4 2025-05-28)
Import defer                      modules       3          2025-02-18
Await dictionary                  concurrency   3          2026-07-20
Pipeline operator (Hack)          pipelines     2          2021-08-31
Throw expressions                 errors        2          2017-09-27
AsyncContext                      context       2          2023-03-23
Extractors                        matching      2          2024-10-09
Async iterator helpers            iteration     2          2023-01-31 (split out, demoted 3 -> 2)
Composites                        data          2          2026-09-30 (notes unpublished)
Discard (void) bindings           resources     2          2024-06-13
Error code property               errors        2          2026-07-21
Iterator.range                    iteration     2          2023-03-22
Pattern matching                  matching      1          2018-05-24
Partial application               pipelines     1          2017-09-28
do expressions                    expressions   1          2017-01-26
async do expressions              concurrency   1          2021-01-27
Call-this (~>)                    pipelines     1          2021-10-27
Extensions (::)                   pipelines     1          2020-11-19
Concurrency control               concurrency   1          2024-07-29
Unordered async iterator helpers  iteration     1          2024-07-30
await operations (await.all)      concurrency   1          2020-07-22
Cancellation API                  cancellation  1          2017-07-25
Abort Controller                  cancellation  1          2026-09-30 (notes unpublished)
Disposable AsyncContext.Variable  context       1          2025-04-17
Strict enforcement of `using`     resources     1          2024-04-11
Function bind syntax (old ::)     pipelines     0          listed at Stage 0 (presented 2015-03)
F#-style pipes                    pipelines     none       failed Stage 2 2017-11-29 and 2018-03-22
Function.pipe / Function.flow     pipelines     withdrawn  rejected for Stage 1 2022-07-21
Generator arrow functions         iteration     inactive   withdrawn 2024-02-07
Records & Tuples                  data          withdrawn  2025-04-14 (subsumed by Composites)
JSON.tryParse                     errors        rejected   2023-11-29
Try operator (ex "safe assign.")  errors        none       not in tc39/proposals; README says "Stage 0"
```

---

## 1. Pipeline operator (Hack style, `|>` with a topic reference)

- **Stage:** 2. Listed in the Stage 2 table of the tc39/proposals README. The proposal README also says "Stage: 2".
- **Last stage change:** Stage 2 at the 2021-08-31 plenary ("Consensus for Stage 2").
- **Champions:** J. S. Choi, Ron Buckton, and Tab Atkins-Bittner (tc39/proposals). The proposal README also lists James DiGioia and marks the list "[list incomplete]".
- **Stage 2.7 reviewers:** Ross Kirsling and Christian Ulbrich, per the tc39/proposals README. They were added by commit `07002d4` on 2026-10-14. The 2026-09 agenda listed `pipeline-operator` under "Stage 2 proposals missing Stage 2.7 reviewers". The plenary decision behind the assignment is UNVERIFIED because the notes are unpublished.
- **Syntax:** `value |> one(%) |> two(%, 1) |> await %` (the README's placeholder token).

### Topic token: not final

- The README uses `%` and says this "will ***almost certainly not be the final choice***". The spec text, last changed 2022-04-01, "presumptively uses `%`".
- The repository wiki page "Bikeshedding the Hack topic token" lists these current possibilities: `#`, `##`, `^^`, `<^`, `%%`, `@@`, and `#_`.
- The same page lists these tokens as disqualified:
  - `%`, `^`, and `*`: implementers objected to the parsing complexity (2021-11 incubator call).
  - `@`: complex interactions with decorators.
  - Among others: `~`, `?`, `::`, `..`, `~~`, `var`, `$`, `_`, `$_`, `it`, `@it`, and non-ASCII symbols, each with its own stated reason.
- History of the bikeshed:
  - 2022-03-29 plenary: the notes' conclusion reads "`@` was popular"; HISTORY.md says the committee "mildly preferred `@`".
  - 2022-04: `@` was excluded after a delegate raised concerns (official changes thread #232).
- Latest champion position: on 2026-07-27, J. S. Choi called `<^` (proposed that day by Nicolò Ribaudo) "tied for my other frontrunners `^^` and `##`" (#91).
- The `#` candidate depends on Composites not taking `#[…]`/`#{…}` literal syntax (wiki; champion update 2025-04-07). The Composites README now defers syntax to a separate follow-on proposal and sketches `#{ x: 1 }` (see §13).

### Plenary and champion record, 2024-2026

- **2024-07-30, Stage 2 scrub.** No champion was present.
  - MM supported "this proposal as proposed not the f-sharp one".
  - DLM (Mozilla) reported "a general negative feeling about the pipeline operator".
  - SYG (Google) said the demand is recognized and wants a way to support it without engine support.
  - EAO called the syntax "very noisy" (an aesthetic objection).
- **2024-10-08, JSSugar/JS0 (SYG).** Pipeline was cited as an example of syntax people want to type that cannot be expressed as an API. The conclusion was "No consensus on JSSugar/JS0".
- **2025-04-07, champion update on #232 (not a plenary).** J. S. Choi wrote:
  - Engine implementers lead the opposition to tacit programming.
  - Hack pipes "remain the only viable path forward".
  - F#/Elixir pipes "still do not have a chance to succeed", and PFA "remains in stasis".
  - The token bikeshed is tied to Composites' use of `#`.
  - The proposal is "definitely not going to reach Stage 4 this year".
- **2026-01-21, Stage 2 proposal review.**
  - RBN (co-champion) calls himself "a conscientious objector" who still prefers F# pipes. He said the topic token is unresolved and the champion group has "not had a meeting in quite a while".
  - JHD said some delegates doubt the use case is worth syntax, and that delegates disagree about whether the topic should be nameable.
  - DLM (Mozilla) said it is "unlikely to be something we would support". He cited skepticism of syntax that adds no new capability.
  - The summary lists the pipeline under "Proposals possibly in need of champions": "(proposal is somewhat stuck, additional champions may help move the proposal forward)".
- **2026-09-29 to 10-01.** The agenda included the reviewer item above. No pipeline presentation was scheduled.

### Babel

- Plugin: `@babel/plugin-proposal-pipeline-operator`. The proposal README says Hack pipes were implemented in Babel 7.15.
- The current docs cover Babel 8. Babel v8.0.0 was released 2026-06-16, and the docs dropped their Babel 7 conditionals on 2026-07-17.
- The `proposal` option takes `"hack"` (the default) or `"fsharp"`.
- `"hack"` requires a `topicToken`: one of `"%"`, `"^^"`, `"@@"`, `"^"`, or `"#"`.
- Babel 8 removed the `"smart"` and `"minimal"` modes (v8 migration guide; babel/babel#17732).

### TypeScript

- **#17718** ("Support proposed ES Next "|>" pipeline operator"):
  - Labels: Suggestion, ES Next, Waiting for TC39. Locked 2019-03-15.
  - Closed 2023-06-23 by Ryan Cavanaugh: "Marking this as closed since there's nothing for us to do until something happens at TC39. … please contribute feedback there until that proposal (or its kin) reaches stage 3."
  - Earlier, Ryan Cavanaugh replied about community PR #22816: "We're *definitely* not merging this."
- **#63323** ("Support pipe `|>` operator"): opened and closed 2026-03-30 as not planned, labeled "Out of Scope". Ryan Cavanaugh noted that the "This isn't a runtime feature" checklist item is mandatory. The feature-request template's viability checklist includes "This isn't a runtime feature (e.g. library functionality, non-ECMAScript syntax with JavaScript output, new syntax sugar for JS, etc.)".

### Fate of F#-style pipes

- F# pipes were never a separate entry in tc39/proposals.
- HISTORY.md records this sequence:
  - 2017-09-26: "F#-or-Elixir pipes" reached Stage 1 inside this proposal.
  - 2017-11-29 and 2018-03-22: Stage 2 was attempted without success.
  - 2021-08-31: Hack pipes were chosen and advanced.
- The README says "TC39 has rejected F# pipes multiple times".
- Babel still supports `proposal: "fsharp"`.
- Repository issue #202 ("Separate (complementary) F# pipeline proposal?") is open. Its latest comments (2026-10-08) are from non-champions.

### Notable open issues

- #91, the token bikeshed: open, 823 comments, last activity 2026-08-24.
- #202, the F# question above.
- #309, #311, #314, and #315: motivation and real-world examples.

**Sources:**
- https://github.com/tc39/proposal-pipeline-operator
- https://github.com/tc39/proposal-pipeline-operator/blob/main/HISTORY.md
- https://github.com/tc39/proposal-pipeline-operator/blob/main/spec.html
- https://github.com/tc39/proposal-pipeline-operator/wiki/Bikeshedding-the-Hack-topic-token
- https://github.com/tc39/proposal-pipeline-operator/issues/91#issuecomment-5097905537
- https://github.com/tc39/proposal-pipeline-operator/issues/232#issuecomment-2784879225
- https://github.com/tc39/notes/blob/main/meetings/2021-08/aug-31.md#pipeline-operator-for-stage-2
- https://github.com/tc39/notes/blob/main/meetings/2022-03/mar-29.md#bikeshedding-the-pipe-operator-topic-token
- https://github.com/tc39/notes/blob/main/meetings/2024-07/july-30.md#scrub-of-stage-2-proposals
- https://github.com/tc39/notes/blob/main/meetings/2024-10/october-08.md#jssugarjs0
- https://github.com/tc39/notes/blob/main/meetings/2026-01/january-21.md#pipeline-operator-pt-1 (and `#pipeline-operator-pt-2`)
- https://github.com/tc39/agendas/blob/main/2026/09.md
- https://github.com/tc39/proposals/commit/07002d4d999558b9c7326f98e5134e0222b13808
- https://github.com/babel/website/blob/main/docs/plugin-proposal-pipeline-operator.md
- https://github.com/babel/website/blob/main/docs/v8-migration.md
- https://github.com/microsoft/TypeScript/issues/17718#issuecomment-1604896793
- https://github.com/microsoft/TypeScript/issues/63323
- https://github.com/microsoft/TypeScript/blob/main/.github/ISSUE_TEMPLATE/feature_request.yml

---

## 2. Function.pipe and Function.flow (J. S. Choi)

- **Status:** withdrawn. The tc39/proposals inactive list says "Withdrawn: rejected for stage 1".
- **Last change:** presented for Stage 1 at the 2022-07-21 plenary. The conclusion was "No advancement". The README says the champion "subsequently withdrew it from consideration" and calls it a "Stage-0 Proposal (**withdrawn**)".
- **Champion:** J. S. Choi.
- **API:** `Function.pipe(x, f, g, h)`, `Function.flow(f, g, h)`, `Function.pipeAsync`, `Function.flowAsync`.
- **Reasons:**
  - README summary: the committee "generally found its use cases either easily solved by **userland functions**" or "also solved by the pipe operator".
  - At the 2022-07-21 plenary, DE and MM asked the committee to choose between a pipe function and pipe syntax.
  - MM: "I think it's unacceptable to have both in the language".
- **Origin:** split out of `proposal-function-helpers`, which was rejected before Stage 1 in October 2021 (inactive list).

**Sources:**
- https://github.com/tc39/proposal-function-pipe-flow
- https://github.com/tc39/proposals/blob/main/inactive-proposals.md
- https://github.com/tc39/notes/blob/main/meetings/2022-07/jul-21.md#functionpipe--flow-for-stage-1

---

## 3. Partial application (`f~(x, ?)`)

- **Stage:** 1.
- **Last stage change:**
  - Stage 1 on 2017-09-28. MF said at the time that he did not believe "we'll be able to come up with a syntax that makes this proposal viable".
  - Stage 2 was requested 2018-07-25 and 2021-10-25. The 2021-10-25 conclusion was "Does not advance at this time".
- **Champion:** Ron Buckton.
- **Syntax:** `const addOne = add~(1, ?);`
  - Ordinal placeholder: `identity~(?1)`.
  - Rest placeholder: `console.log~("[service]", ...)`.
  - Constructors: `new C~(?)`.
- **Recorded obstacles:**
  - HISTORY.md says the 2021-10 rejection cited "insufficiently specific and compelling use cases … with high syntax cost and novelty in comparison to arrow functions". It also records V8's "strong reservations" about closure allocation (2018).
  - The champion update of 2025-04-07 says PFA "remains in stasis".
  - The README has not been committed to since 2021-10-19.
- **Babel:** `@babel/plugin-proposal-partial-application` requires `version: "2018-07"`, which is the only supported version (the option was added in Babel 8). That version is the older syntax without `~`: `add(1, ?)`. Babel does not implement the current `~()` form.
- **Open issues:** #39 ("Argument against this proposal"), #45 (userland viability), #53, #57, and #58 (2026-02).

**Sources:**
- https://github.com/tc39/proposal-partial-application
- https://github.com/tc39/proposals/blob/main/stage-1-proposals.md
- https://github.com/tc39/notes/blob/main/meetings/2017-09/sept-28.md
- https://github.com/tc39/notes/blob/main/meetings/2021-10/oct-25.md#partial-function-application-for-stage-2
- https://github.com/tc39/proposal-pipeline-operator/blob/main/HISTORY.md
- https://github.com/babel/website/blob/main/docs/plugin-proposal-partial-application.md

---

## 4. Pattern matching (`match`)

- **Stage:** 1.
- **Last stage change:**
  - Stage 1 on 2018-05-24.
  - Stage 2 was requested 2022-03-28 ("Not advancing at this time").
  - The last plenary presentation was an update on 2023-09-26. Its summary: "there are still some things to be solved in the group of champions". tc39/proposals lists no later presentation.
- **Champions:** Daniel Rosenwasser, Jack Works, Jordan Harband, Mark Cohen, Ross Kirsling, and Tab Atkins-Bittner.
- **Syntax:** `match (res) { when { status: 200, let body }: handle(body); default: fallback(); }`.
  - The boolean form is `subject is <pattern>`.
  - Bindings use `let`/`const`. Combinators are `and`, `or`, and `not`. Guards use `and if (...)`.
  - Custom matchers use `Symbol.customMatcher`.
  - With no `default` arm, a failed match throws `TypeError`.
- **Possible future enhancements in the README (not proposed):**
  - `async match`.
  - void patterns.
  - "Integration with `catch`": `catch match (e) { when RangeError: …; }` or `catch (e is RangeError)`.
- **Open issues and blockers:**
  - Arms contain only expressions. The README expects `do` expressions to allow statements in arms (§6).
  - 2026-01-21: RBN said the champion group needs to reschedule its meetings.
  - 2026-07-21: JHD, speaking for RBN, asked that "Declarations in Conditionals" (Stage 1) wait on cross-cutting concerns with pattern matching and extractors. He said a direction delegates pushed earlier is something "we cannot go in".
  - Recent repository commits: 2025-10-03 (spec formatting), 2025-07-07 ("Mention the new Flow syntax"), and 2024-09-24 ("Remove regex patterns from README").
- **Babel:** no plugin in the Babel docs.

**Sources:**
- https://github.com/tc39/proposal-pattern-matching
- https://github.com/tc39/notes/blob/main/meetings/2018-05/may-24.md
- https://github.com/tc39/notes/blob/main/meetings/2022-03/mar-28.md#pattern-matching-for-stage-2
- https://github.com/tc39/notes/blob/main/meetings/2023-09/september-26.md#pattern-matching-updates
- https://github.com/tc39/notes/blob/main/meetings/2026-01/january-21.md#pipeline-operator-pt-2
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-21.md#declarations-in-conditionals-for-stage-2-or-27

---

## 5. Throw expressions

- **Stage:** 2.
- **Last stage change:** Stage 2 on 2017-09-27 ("Stage2 acceptance"). Stage 3 was requested twice and not granted:
  - 2018-01-24: "no promotion".
  - 2023-09-26 and 2023-09-28: "remains at Stage 2 with no conclusion on syntax questions".
- **Champion:** Ron Buckton. Stage 2.7 reviewer listed: Nicolò Ribaudo.
- **Syntax:** `function save(name = throw new TypeError("required")) {}` and `const x = a ? throw b : c;`.
  - Binary operators and `?` are forbidden to the right of a `throw` expression unless parenthesized.
- **Open issues:**
  - Grammar and precedence. In 2023-09, KG and WH had "strong, incompatible preferences". Issues #21, #22, #23, and #18 cover the competing options: `throw` as Expression, as ParenthesizedExpression, mandatory parentheses, and early errors for ASI.
  - 2024-02-08: MM said "we would not be willing to see this advanced as a syntax proposal" and supported exploring a helper function instead. RBN raised TypeScript control-flow and `never` limits as reasons for syntax.
  - 2026-01-21 review: RBN said "The last time it left off was a discussion about whether it could be an `error.throw` method, and I have concerns". The summary lists it as "backlogged but active".
  - 2023-07-13: RBN said the proposal was waiting on what happens with `do` expressions.
- **Babel:** `@babel/plugin-proposal-throw-expressions`.

**Sources:**
- https://github.com/tc39/proposal-throw-expressions
- https://github.com/tc39/notes/blob/main/meetings/2017-09/sept-27.md
- https://github.com/tc39/notes/blob/main/meetings/2018-01/jan-24.md
- https://github.com/tc39/notes/blob/main/meetings/2023-09/september-28.md#throw-expression-continued
- https://github.com/tc39/notes/blob/main/meetings/2024-02/feb-8.md#throw-expressions-update-or-stage-27
- https://github.com/tc39/notes/blob/main/meetings/2026-01/january-21.md#discard-void-binding--extractors--structs--throw-expressions
- https://github.com/babel/website/blob/main/docs/plugin-proposal-throw-expressions.md

---

## 6. `do` expressions (and `async do`)

### `do` expressions

- **Stage:** 1.
- **Last stage change:** Stage 1 on 2017-01-26 (champion at the time: DH). Stage 2 was requested twice:
  - 2020-06-01: "Not advancing yet".
  - 2021-01-26: "Proposal does not advance. Reason: Conflicting requirements about return and loops need to be resolved."
- **Champion:** Kevin Gibbons. On 2023-11-29 he said: "I am not planning to work on it in the immediate future".
- **Syntax:** `let x = do { let t = f(); t * t + 1 };`. Ending the block with a declaration, a loop, or an `if` without `else` is an early error.
- **Open issues:** #80 ("Simplify proposal to exclusively syntactic sugar?", active 2026-06) and #72 ("Implicit return is bad", active 2026-06).
- **Babel:** `@babel/plugin-proposal-do-expressions`.

### `async do`

- **Stage:** 1. Reached Stage 1 on 2021-01-27. The last presentation was 2021-03-09 ("update towards stage 2").
- **Syntax:** `async do { await f(); }` evaluates to a Promise.
- **Babel:** `@babel/plugin-proposal-async-do-expressions`.

**Sources:**
- https://github.com/tc39/proposal-do-expressions
- https://github.com/tc39/proposal-async-do-expressions
- https://github.com/tc39/notes/blob/main/meetings/2017-01/jan-26.md
- https://github.com/tc39/notes/blob/main/meetings/2021-01/jan-26.md#do-expressions
- https://github.com/tc39/notes/blob/main/meetings/2021-01/jan-27.md
- https://github.com/tc39/notes/blob/main/meetings/2023-11/november-29.md#jsontryparse
- https://github.com/babel/website/blob/main/docs/plugin-proposal-do-expressions.md

---

## 7. Explicit Resource Management (`using`, `DisposableStack`)

- **Stage:** 4. The proposal is in finished-proposals.md with "Expected Publication Year" 2027.
- **Last stage change:**
  - 2025-05-28: conditional Stage 4, pending final Test262 review and editor sign-off.
  - 2026-03-10: status update; still waiting on Test262 and editorial review.
  - 2026-05-19: "All conditions met for Stage 4", plus one needs-consensus change: `%AsyncIteratorPrototype%[Symbol.asyncDispose]` no longer passes `undefined` to `return`.
  - tc39/proposals commit `5adf344`: "Explicit Resource Management to stage 4, per 2026.05.19 TC39".
  - Earlier stages: Stage 2 on 2019-07-25; Stage 3 on 2022-12-01 for sync and 2023-03 for async.
- **The proposal README is stale.** It still says "Stage: 3", and its last commit is 2024-06-15.
- **Champion:** Ron Buckton.
- **Syntax:** `using f = open(); await using c = await connect();`
  - Built-ins: `DisposableStack`/`AsyncDisposableStack` (`use`, `adopt`, `defer`, `move`), `SuppressedError`, `Symbol.dispose`, and `Symbol.asyncDispose`.
  - `using` is not allowed directly in `case`/`default` clauses (consensus 2025-04-15) or at the top level of a Script.
  - Spec PR: tc39/ecma262#3000.

### Engines

From the TC39 notes:

- **2025-05-28:** "shipping in Chrome 134 and shipping in Firefox behind a flag". Moddable XS has an implementation. Speaker's summary: "Implemented in Chrome, Firefox, and XS".
- **2025-05-28:** LCA said Deno shipped it using V8's implementation.
- **2025-04-15:** Mozilla reported a full SpiderMonkey implementation in Nightly, disabled behind a pref (the transcript reads "prop").

SUPPLEMENTARY (engine commit logs):

- **SpiderMonkey:** "Bug 1967744 - Ship Explicit Resource Management" (2025-06-06). The pref and compile flag were removed 2026-07-28 (Bug 1969212).
- **V8:** `--js-explicit-resource-management` was removed 2026-08-19.
- **JavaScriptCore:** "[JSC] Enable Explicit Resource Management by default" (2026-07-24, 317894@main).

UNVERIFIED: which Safari release first ships it, and which Firefox release numbers ship it.

### Tooling

- **Babel:** `@babel/plugin-transform-explicit-resource-management` is included in `@babel/preset-env`. The Babel docs label it "ES2026"; tc39/proposals lists 2027.
- **TypeScript:** `using`/`await using` support landed in PR #54505, merged 2023-06-22.

### Follow-on work

- Strict Enforcement of `using` (Stage 1, §18).
- Discard bindings (Stage 2, §18).
- Disposable AsyncContext.Variable (Stage 1, §8).
- 2026-03-10: MM asked for a follow-on that adds an options bag to `SuppressedError`. JSL said the error `code` property proposal would add one.

**Sources:**
- https://github.com/tc39/proposals/blob/main/finished-proposals.md
- https://github.com/tc39/proposals/commit/5adf344f5091c8726363594b44d54f14c97a52f6
- https://github.com/tc39/proposal-explicit-resource-management
- https://github.com/tc39/notes/blob/main/meetings/2025-04/april-15.md#explicit-resource-management
- https://github.com/tc39/notes/blob/main/meetings/2025-05/may-28.md#explicit-resource-management-for-stage-4 (and `#explicit-resource-management-continuation`)
- https://github.com/tc39/notes/blob/main/meetings/2026-03/march-10.md#explicit-resource-management-conditional-stage-4-status-update
- https://github.com/tc39/notes/blob/main/meetings/2026-05/may-19.md#explicit-resource-management-stage-4-progress-update
- https://github.com/babel/website/blob/main/docs/plugin-transform-explicit-resource-management.md
- https://github.com/microsoft/TypeScript/pull/54505
- SUPPLEMENTARY:
  - https://github.com/mozilla-firefox/firefox/commit/0739d2a8b127b8ddace100a886e56a30b09007e0
  - https://github.com/mozilla-firefox/firefox/commit/16413c68614135282257676d9d0b9d5094a62a7b
  - https://github.com/v8/v8/commit/b5a03cd89325ee7d958f1aee62400dec94c075ca
  - https://github.com/WebKit/WebKit/commit/9845affea3ca058cd0a3ca659eabf792cdc326b8

---

## 8. AsyncContext

- **Stage:** 2.
- **Last stage change:** Stage 1 on 2023-02-01; Stage 2 on 2023-03-23.
- **Champions:** Andreu Botella, Chengzhong Wu, and Justin Ridgewell.
- **Stage 2.7 reviewers:** James M Snell and Mark S. Miller (assigned 2024-12-02).
- **Syntax (API only):**
  ```js
  const v = new AsyncContext.Variable();
  v.run(value, fn);
  v.get();
  new AsyncContext.Snapshot().run(fn);
  AsyncContext.Snapshot.wrap(fn);
  ```

### Semantics at `await` and generator resumption (spec.html at HEAD; last spec commit 2026-03-17)

- **`await`: yes, the context is snapshotted at the await point.**
  - `PerformPromiseThen` stores `AsyncContextSnapshot()` in each PromiseReaction (`[[PromiseAsyncContextMapping]]`).
  - `NewPromiseReactionJob` swaps that mapping in before running the handler.
  - `await` registers its reactions at the await point, so code after `await` resumes in the mapping that was active at the `await`.
  - README: "AsyncContext.Variable is propagated on async/await."
- **Generators: they snapshot at creation, not at each `.next()`.**
  - `GeneratorStart` takes `AsyncContextSnapshot()` when the generator object is created. The body runs in that mapping.
  - Each `yield` snapshots before suspending and swaps the snapshot back in after resuming.
  - `GeneratorResume` saves and restores the caller's mapping.
  - The result: a generator body never sees the context of the `.next()` caller. NRO, 2025-09-23: "once you are in a given code, the context is constant".
- **`yield*`: an agreed change is not yet in the main repository.**
  - The 2025-09-23 plenary agreed ("There were no concerns") to move the `AsyncContextSwap` out of the `yield*` loop. With that change, a delegated iterator's `next` runs in the outer `.next()` caller's context.
  - spec.html at HEAD still swaps inside the loop. The change was presented from a commit on NRO's fork (`32856cb`).
  - UNVERIFIED: whether a merge into tc39/proposal-async-context is pending.
  - Issue #18 ("Specifying (async) generator behavior") is open.

### Notable open issues

- **Web integration.**
  - 2026-05-20: the registration context for events was dropped and five rules for dispatch versus empty context were defined. WHATWG PRs are open; tracking issue #152.
- **Implementer trade-off.** 2025-04-14 conclusion: "Different browsers have opposite opinions about this tradeoff", referring to use cases versus implementation cost.

### Related proposals

- **Disposable AsyncContext.Variable:** Stage 1 on 2025-04-17. Champions: Chengzhong Wu, Luca Casonato, and Gus Caplan. Syntax: `using _ = v.withValue(x);`.
- **Propagate active ScriptOrModule with JobCallback Record:** Stage 2. The 2026-01-21 summary notes it "will likely advance alongside" AsyncContext.

**Sources:**
- https://github.com/tc39/proposal-async-context (README, spec.html)
- https://github.com/tc39/proposal-async-context/issues/18
- https://github.com/tc39/proposal-async-context-disposable
- https://github.com/tc39/notes/blob/main/meetings/2023-01/feb-01.md
- https://github.com/tc39/notes/blob/main/meetings/2023-03/mar-23.md
- https://github.com/tc39/notes/blob/main/meetings/2024-12/december-02.md#asynccontext-request-for-stage-27-reviewers
- https://github.com/tc39/notes/blob/main/meetings/2025-04/april-14.md#asynccontext-stage-2-update
- https://github.com/tc39/notes/blob/main/meetings/2025-04/april-17.md#disposable-asynccontext-for-stage-1
- https://github.com/tc39/notes/blob/main/meetings/2025-09/september-23.md#asynccontext-yield
- https://github.com/tc39/notes/blob/main/meetings/2026-05/may-20.md#asynccontext-stage-2-web-integration-status-update

---

## 9. Extractors

- **Stage:** 2.
- **Last stage change:**
  - Stage 2 on 2024-10-09.
  - An earlier Stage 2 request (2024-04-10 and 2024-04-11) did not advance because of grammar issues.
- **Champion:** Ron Buckton. Stage 3 reviewers: Jordan Harband and Justin Ridgewell.
- **Syntax:** `const Point(x, y) = p;` and `const Foo({ y }) = x;`. With pattern matching: `when Option.Some(y): …`. Extractors use `Symbol.customMatcher` from the pattern matching proposal.
- **Open issues:**
  - 2026-01-21: listed as "backlogged but active (RBN needs to confer with pattern matching champions)".
  - #35, "Handling of extraction failures": a 2026-07-14 dispute between RBN and KG that ties into Declarations in Conditionals.
  - #32 and #33: `customMatcher` static in classes.
  - #34: overlap with "Runtime Errors for Function Call Assignment Targets".
  - Last README commit: 2024-09-27.
- **Babel:** no plugin in the Babel docs.

**Sources:**
- https://github.com/tc39/proposal-extractors
- https://github.com/tc39/proposal-extractors/issues/35
- https://github.com/tc39/notes/blob/main/meetings/2024-04/april-11.md#continuation-extractors
- https://github.com/tc39/notes/blob/main/meetings/2024-10/october-09.md#extractors-for-stage-2

---

## 10. Call-this (`~>`), Extensions (`::`), and the old bind operator

### Call-this

- **Stage:** 1. Reached Stage 1 on 2021-10-27 as "bind-this". The last plenary item was "Bikeshedding call-this syntax" on 2022-03-29.
- **Champion:** J. S. Choi.
- **Tentative syntax:** `receiver~>fn(arg0, arg1)`. The syntax is still being bikeshedded in #10 (latest comments 2026-03-26).
- **Committee reception:** HISTORY.md says it "continues to polarize the Committee due to ecosystem-schism concerns". In the 2025-04-07 update, J. S. Choi says he plans to propose a pared-down `Function.prototype.demethodize` method "that does the job of some parts of call-this".
- **Babel:** the README says "Babel plugin: Not yet".

### Extensions

- **Stage:** 1. Reached Stage 1 on 2020-11-19, with "stage 1 concerns about pipeline operator and namespace".
- **Champion:** HE Shi-Jun.
- **Syntax:**
  - Declaring an extension: `const ::toArray = function () { return [...this] };`
  - Calling it: `x::toArray()`
  - Using a namespace member: `x::Array:flatMap(f)`
- **Activity:** last README commit 2023-01-18. Open issues #17 ("Built-in extension functions"), #18 ("Reshaping the Extensions proposal"), and #19 were active from 2025-09 to 2026-03.

### Function bind syntax (old `::`)

- Listed in stage-0-proposals.md. Champion: Brian Terlson. Presented 2015-03.
- Babel: `@babel/plugin-proposal-function-bind` (`obj::func`, `::obj.func`).

**Sources:**
- https://github.com/tc39/proposal-call-this
- https://github.com/tc39/proposal-call-this/issues/10
- https://github.com/tc39/proposal-extensions
- https://github.com/tc39/proposal-bind-operator
- https://github.com/tc39/proposals/blob/main/stage-0-proposals.md
- https://github.com/tc39/notes/blob/main/meetings/2021-10/oct-27.md#bind-this-operator-for-stage-1
- https://github.com/tc39/notes/blob/main/meetings/2022-03/mar-29.md#bikeshedding-call-this-syntax
- https://github.com/tc39/notes/blob/main/meetings/2020-11/nov-19.md
- https://github.com/babel/website/blob/main/docs/plugin-proposal-function-bind.md

---

## 11. Generator arrow functions (`=>*`)

- **Status:** inactive. The inactive list says "Withdrawn: no interested champions in February 2024".
- **Last change:**
  - 2024-02-07, Stage 1 scrub: DE asked for interest and found none.
  - tc39/proposals commit `e55a54a`: "Generator Arrow functions to inactive, per 2024.02.07 TC39".
  - Last presentation: 2016-09-27.
- **Champions (inactive list):** Sergey Rubanov and Brendan Eich.
- **Syntax candidates (README, which still says Stage 1):** `() =*> …`, `() =>* …`, `*() => …`, and a `generator` keyword (`generator function () {}`). No form was chosen.

**Sources:**
- https://github.com/tc39/proposal-generator-arrow-functions
- https://github.com/tc39/proposals/blob/main/inactive-proposals.md
- https://github.com/tc39/proposals/commit/e55a54ac88574ff9b2bae653532aa69dd1232a93
- https://github.com/tc39/notes/blob/main/meetings/2024-02/feb-7.md#scrub-stage-1-proposals

---

## 12. Import defer (and `export defer`)

### Import defer

- **Stage:** 3. Reached Stage 3 on 2025-02-18 ("Consensus for Stage 3").
- **Champion:** Nicolò Ribaudo. Authors: Yulia Startsev, Nicolò Ribaudo, and Guy Bedford.
- **Syntax:** `import defer * as ns from "mod";` (namespace form).
- **Status update, 2026-07-20:**
  - "All major browsers started their implementation, with JSC and V8 being available behind a flag."
  - Consensus on a spec fix for issue #85.
  - The spec stays unchanged for issues #82 and #83.
  - SUPPLEMENTARY: V8's `src/flags/feature-flags.h` at HEAD lists `js_defer_import_eval` among staged features.
- **Babel:** `@babel/plugin-proposal-import-defer`. It works only when also compiling modules to CommonJS.
- **TypeScript:** support landed in PR #60757 (merged 2025-06-06). PR #64640 (merged 2026-10-07) rejects deferred imports without namespace bindings.

### `export defer` (deferred re-exports)

- **Stage:** 2.7, per tc39/proposals commit `cf3caef` ("per 2026.09.30 TC39"). Notes unpublished.

**Sources:**
- https://github.com/tc39/proposal-defer-import-eval
- https://github.com/tc39/notes/blob/main/meetings/2025-02/february-18.md#import-defer-for-stage-3
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-20.md#import-defer-stage-3-updates
- https://github.com/tc39/proposals/commit/cf3caefc90053a478a4eaf073fdcaa8fb0e9c888
- https://github.com/babel/website/blob/main/docs/plugin-proposal-import-defer.md
- https://github.com/microsoft/TypeScript/pull/60757
- https://github.com/microsoft/TypeScript/pull/64640
- SUPPLEMENTARY: https://github.com/v8/v8/blob/main/src/flags/feature-flags.h

---

## 13. Composites (successor to Records & Tuples)

### Composites

- **Stage:** 2.
- **Last stage change:**
  - Stage 1 on 2025-04-14.
  - Stage 2 per tc39/proposals commit `dbed920` ("Composites to stage 2, per 2026.09.30 TC39"). The proposal README commit `03b7388` (2026-10-10) says "Proposal is now stage 2". The 2026-09 notes are unpublished.
- **Champion:** Ashley Claymore. Stage 2.7 reviewers: Jordan Harband, Michael Ficarra, and Ross Kirsling.
- **API (no syntax):**
  - `Composite({ x: 1, y: 4 }) === Composite({ x: 1, y: 4 }) // true`. Composites are interned per realm.
  - `Composite.isComposite(v)`.
  - Option `{ preserveNegativeZero: true }`.
  - Composites cannot be held weakly.
- **Syntax:** the README says syntax "would be a separate follow-on proposal" and sketches `#{ x: 1 }`.
- **Open issues:** 2026-07-21 conclusion: "pivot to an interning based approach … Will continue discussing handling 'undefined' values".

### Records & Tuples

- **Status:** withdrawn 2025-04-14. Inactive list: "Subsumed by Composites".
- Babel 8 removed `@babel/plugin-proposal-record-and-tuple` (v8 migration guide).

**Sources:**
- https://github.com/tc39/proposal-composites
- https://github.com/tc39/proposals/commit/dbed92055dede345c89df93f59fb6020a21cd2d7
- https://github.com/tc39/notes/blob/main/meetings/2025-04/april-14.md#composite-keys-for-stage-1 (and `#withdrawing-records--tuples`)
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-21.md#composites-update
- https://github.com/tc39/agendas/blob/main/2026/09.md
- https://github.com/babel/website/blob/main/docs/v8-migration.md

---

## 14. Async iterator helpers (and unordered async iterator helpers)

### Async iterator helpers

- **Stage:** 2.
- **Last stage change:** 2023-01-31. The committee split the async helpers out of iterator helpers and "demoted to stage 2". The sync helpers reached Stage 4 in 2024-10.
- **Champions:** Michael Ficarra, Jonathan Keslin, and Kevin Gibbons (tc39/proposals). The README lists Michael Ficarra and Kevin Gibbons.
- **API:**
  - `AsyncIterator.from`
  - `Iterator.prototype.toAsync`
  - `.map`, `.filter`, `.take`, `.drop`, `.flatMap`, `.reduce`, `.toArray`, `.forEach`, `.some`, `.every`, `.find`
- **Status:** the README says "This proposal is in the process of being revised" to support concurrent `.next()` calls.
- **Plenary record:**
  - 2026-07-21: the committee supports skipping `await` for primitive mapper results; "other questions remain open".
  - 2026-07-22: when closing `flatMap`, the committee prefers closing the inner iterator first.
  - The 2026-09 agenda had a 120-minute slot ("the whole proposal + open questions"). The outcome is UNVERIFIED; tc39/proposals still lists Stage 2.
- **Open issues:** #22, #30, and #31.

### Unordered async iterator helpers

- **Stage:** 1. Reached Stage 1 on 2024-07-30.
- **Champion:** Michael Ficarra.
- **API:** `.unordered()` lifts an async iterator to an `UnorderedAsyncIterator`.

**Sources:**
- https://github.com/tc39/proposal-async-iterator-helpers
- https://github.com/tc39/proposal-unordered-async-iterator-helpers
- https://github.com/tc39/notes/blob/main/meetings/2023-01/jan-31.md#parallel-async-iterators-via-a-tweak-to-iterator-helpers
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-21.md#async-iterator-helpers-open-questions
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-22.md#continuation-async-iterator-helpers-open-questions
- https://github.com/tc39/notes/blob/main/meetings/2024-07/july-30.md#unordered-async-iterator-helpers-for-stage-1

---

## 15. Concurrency control

- **Stage:** 1.
- **Last stage change:** Stage 1 on 2024-07-29. A Stage 1 update on 2025-11-19 narrowed the scope to exclude concurrent task queues.
- **Champions:** Michael Ficarra and Luca Casonato.
- **API:**
  - `Governor` and `AbortableGovernor` interfaces.
  - `CountingGovernor(capacity)`.
  - `acquire(): Promise<GovernorToken>`. The token is disposable: `using void = await governor.acquire();`.
  - `governor.wrap(fn)` and `wrapIterator(it)`.
  - A concurrency parameter on async-iterator consuming methods, and a `limit(governor)` method.
- **Open issues:**
  - 2024-07-29: "much concern about the naming of Semaphore" and "some skepticism about the motivation for the full Governor abstraction from Igalia and Mozilla".
  - #14 (use `AbortSignal`) and #15 to #19 (2025-11).

**Sources:**
- https://github.com/tc39/proposal-concurrency-control
- https://github.com/tc39/notes/blob/main/meetings/2024-07/july-29.md#concurrency-control-presenter-mf-and-lca
- https://github.com/tc39/notes/blob/main/meetings/2025-11/november-19.md#concurrency-control-stage-1-update

---

## 16. Cancellation: Cancellation API and Abort Controller (structured concurrency)

### Cancellation API

- **Stage:** 1. Reached Stage 1 on 2017-07-25. Last presented 2018-07-25.
- **Champions:** Ron Buckton and James M. Snell. James M. Snell was added per tc39/proposals commit `7921e40` ("per 2026.03.10 TC39").
- **2026-03-10, "Abort Protocol Discussion" (JSL):**
  - "Yes, there's general agreement it should be part of the language".
  - Open points: synchronous callbacks, combinators, and alignment with `AbortSignal`.
  - "JSL will join the champions of the original effort".

### Abort Controller

- **Stage:** 1, per tc39/proposals commit `a6230e0` ("Abort Controller to stage 1, per 2026.09.30 TC39"). Agenda item: "Pulling in AbortController for Stage 1 or something".
- **Champion:** Kevin Gibbons.
- **Repository:** `bakkot/structured-concurrency-for-js`.
- **2026-03-12, "Towards Structured Concurrency" (KG):** "General support from the committee for this direction". KG described a path "as a series of independently-motivated extensions to AbortController and AbortSignal, making heavy use of the new `using` syntax". Also recorded: "Safari, at least, is committed to only having AbortController-based cancellation".
- **README ideas (no API decided):**
  - `signal.addAbortCallback(cb)`, returning a disposable.
  - `using d = new AbortController.Disposable()`.
  - `await using c = new AbortController.AsyncDisposable()`, whose `abort()` returns a Promise that waits for async cleanup.
  - Alternatives: `controller.wrap(promise)` and `controller.all(promises)`.

### Inactive

- Cancelable Promises (Domenic Denicola), withdrawn in 2016.

**Sources:**
- https://github.com/tc39/proposal-cancellation
- https://github.com/bakkot/structured-concurrency-for-js
- https://github.com/tc39/proposals/commit/7921e4088405a62010fd14f2f62db27f4cd6a922
- https://github.com/tc39/proposals/commit/a6230e03891ff324e143d915cab1bd875dcdd897
- https://github.com/tc39/notes/blob/main/meetings/2017-07/jul-25.md
- https://github.com/tc39/notes/blob/main/meetings/2026-03/march-10.md#abort-protocol-discussion
- https://github.com/tc39/notes/blob/main/meetings/2026-03/march-12.md#towards-structured-concurrency
- https://github.com/tc39/agendas/blob/main/2026/09.md

---

## 17. Try operator ("safe assignment operator") and JSON.tryParse

### Try operator

- **Status:** not a TC39 proposal.
  - It does not appear in any tc39/proposals list (Stage 0, Stage 1, active, finished, or inactive).
  - The README at `arthurfiorette/proposal-try-operator` says "**Stage:** 0" and "**Champion:** _Actively looking for one_". Its first concept was the "Safe Assignment Operator"; the old repository name now redirects here.
  - No plenary presentation appears in the notes from 2023-07 to 2026-07 or in the 2026 agendas (searched for "try operator", "safe assignment", and "?=").
- **Syntax (README):**
  ```js
  const result = try JSON.parse(input);
  const [ok, err, value] = try await fetch(url);
  ```
  The expression yields a `Result`.
- **Only related plenary remark:** on 2023-11-29, MM said "'try expressions' are in the middle. And altogether, I think I would still object to them, but more weakly than I object to special casing `JSON.parse`."

### JSON.tryParse

- **Status:** rejected 2023-11-29. Inactive list: "some deemed this too specific a solution for a generalized language-wide problem".

**Sources:**
- https://github.com/arthurfiorette/proposal-try-operator
- https://github.com/tc39/proposals (all lists)
- https://github.com/tc39/notes/blob/main/meetings/2023-11/november-29.md#jsontryparse

---

## 18. Other active proposals in scope

```
proposal                          stage  last stage change  champion(s)                          syntax / API
--------------------------------  -----  -----------------  -----------------------------------  ---------------------------------------
Discard (void) bindings           2      2024-06-13         Ron Buckton                          using void = getResource(); const [void, x] = a;
Strict enforcement of `using`     1      2024-04-11         Ron Buckton                          Symbol.enter (opt-in "must be using'd")
Await dictionary                  3      2026-07-20         A. Claymore, J. Harband, C. de       await Promise.allKeyed({ a: pA, b: pB })
                                                            Almeida                              (+ Promise.allSettledKeyed)
await operations                  1      2020-07-22         Jack Works, Jordan Harband           await.all xs  (also .race/.any/.allSettled)
Error code property               2      2026-07-21         James M Snell                        new Error(msg, { code: "ERR_X" })
Declarations in Conditionals      1      UNVERIFIED; Stage  Devin Rousso                         if (let x = f(); x) { … }  (let, const,
                                         2 asked 2025-11                                         using, await using; if/while only)
                                         and 2026-07
Iterator.range                    2      2023-03-22         Jack Works                           Iterator.range(0, 10)
function.sent                     2      UNVERIFIED         HE Shi-Jun (README); listed as       function.sent (first .next() argument)
                                                            missing a champion, 2026-09 agenda
Observable (TC39)                 1      UNVERIFIED         Jafar Husain, Mark Miller            (WHATWG Observable presented 2025-04-17)
```

### Notes

- **Discard bindings:**
  - At Stage 2 advancement, "Many delegates expressed a preference for underscore. Some delegates maintain strong preference for 'void'."
  - Babel: `@babel/plugin-proposal-discard-binding` (`syntaxType: "void"`).
  - 2026-01-21: listed as "backlogged but active".
- **Error code property:** the 2026-07-21 conclusion was "Further advancement blocked on reconciling with DOMException".
- **Declarations in Conditionals:** 2026-07-21 conclusion: "Need to meet with champion of Pattern Matching proposal."
- **function.sent:** 2026-01-20: "Conditionally withdrawn, pending JHX's approval … Later update: not withdrawn yet".

**Sources:**
- https://github.com/tc39/proposal-discard-binding
- https://github.com/tc39/proposal-using-enforcement
- https://github.com/tc39/proposal-await-dictionary
- https://github.com/tc39/proposal-await.ops
- https://github.com/tc39/proposal-error-code-property
- https://github.com/tc39/proposal-Declarations-in-Conditionals
- https://github.com/tc39/proposal-iterator.range
- https://github.com/tc39/proposal-function.sent
- https://github.com/tc39/proposal-observable
- https://github.com/tc39/notes/blob/main/meetings/2024-06/june-13.md#discard-bindings-update-or-stage-2
- https://github.com/tc39/notes/blob/main/meetings/2024-04/april-11.md#strict-enforcement-of-using
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-20.md#await-dictionary-for-stage-3
- https://github.com/tc39/notes/blob/main/meetings/2020-07/july-22.md
- https://github.com/tc39/notes/blob/main/meetings/2026-07/july-21.md#error-code-property-for-stage-2-or-27
- https://github.com/tc39/notes/blob/main/meetings/2023-03/mar-22.md#iteratorrange-for-stage-2
- https://github.com/tc39/notes/blob/main/meetings/2026-01/january-20.md#withdraw-functionsent
- https://github.com/tc39/notes/blob/main/meetings/2025-04/april-17.md#whatwg-observables
- https://github.com/babel/website/blob/main/docs/plugin-proposal-discard-binding.md

---

## 19. Finished proposals in the same areas

```
proposal                       stage 4 reached                     expected ES year  area
-----------------------------  ----------------------------------  ----------------  -----------
Promise.try                    2024-10-09                          2025              errors
Sync iterator helpers          2024-10-08                          2025              iteration
Error.isError                  2025-05-28                          2026              errors
Array.fromAsync                2025-05-28 (cond. on editor review) 2026              iteration
Iterator sequencing            2025-11-18                          2026              iteration
Joint iteration (Iterator.zip) 2026-05-19                          2027              iteration
Explicit Resource Management   2026-05-19                          2027              resources
Iterator chunking              2026-09-29 (commit ee92ecf)         2027              iteration
Iterator.prototype.includes    2026-09-29 (commit 3c1e9ee)         2027              iteration
Iterator.prototype.join        2026-09-29 (commit 3c956df)         2027              iteration
```

The years come from the "Expected Publication Year" column of finished-proposals.md. The 2026-09-29 dates come from tc39/proposals commit messages; those notes are unpublished.

**Sources:**
- https://github.com/tc39/proposals/blob/main/finished-proposals.md
- https://github.com/tc39/notes/blob/main/meetings/2024-10/october-09.md#promisetry-for-stage-4
- https://github.com/tc39/notes/blob/main/meetings/2025-05/may-28.md
- https://github.com/tc39/notes/blob/main/meetings/2025-11/november-18.md
- https://github.com/tc39/notes/blob/main/meetings/2026-05/may-19.md#joint-iteration-for-stage-4

---

## Unverified items

1. Everything said at the 2026-09-29 to 10-01 plenary, including the reasons behind the Composites and Abort Controller advancements, the pipeline Stage 2.7 reviewer assignment, and the outcome of the 120-minute async iterator helpers session. The notes were not published as of fetch.
2. Which Safari and Firefox release versions first ship `using`. The engine-trunk facts above are SUPPLEMENTARY and come from outside the requested source list.
3. Whether the 2025-09 AsyncContext `yield*` change has a pending PR in tc39/proposal-async-context.
4. The Stage 2 date for `function.sent`, and the Stage 1 dates for Observable and Declarations in Conditionals.
5. The TG5 study of the pipeline operator mentioned in the 2025-04-07 champion update. The TG5 workshop page (tc39/tg5 `workshops/2025/106.md`) does not mention the pipeline.
