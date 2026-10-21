/**
 * The source of CONTRIBUTING.md, MAINTAINERS.md, AGENTS.md and templates/app/AGENTS.md.
 * Edit this file, then run `bun run docs:generate` in .smithers/. `{{name}}` inserts a fact below.
 */
export interface Section { readonly heading: string; readonly body: string }
export interface Document { readonly title: string; readonly intro: string; readonly sections: readonly Section[] }

const repo = "https://github.com/smithersai/vibelang";

export const facts = {
  name: "VibeLang",
  issues: `${repo}/issues`,
  newIssue: `${repo}/issues/new/choose`,
  newBug: `${repo}/issues/new?template=bug.yml`,
  newIdea: `${repo}/issues/new?template=idea.yml`,
  newQuestion: `${repo}/issues/new?template=question.yml`,
  // The language rules agents read. Swap this one line for the VibeLang skill's path when it ships.
  languageRules: "https://vibelang.sh/llms.txt",
  setup: "sh scripts/setup.sh",
  e2e: "sh scripts/e2e.sh",
  merge: "A person reviews and merges every change. Agents never approve or merge.",
  generated: "CONTRIBUTING.md, MAINTAINERS.md and both AGENTS.md files come from `.smithers/flows/docs/project.ts`, except the starter's Essentials, which come from `docs/user/llms-essentials.md`. Edit the source, then run `bun run docs:generate` in `.smithers/`",
  style: "Follow the surrounding code and each package's configuration. Format Go with gofmt. Keep TypeScript and VibeLang examples consistent with nearby files, and don't reformat unrelated code.",
  secrets: "Never put credentials or private user content in fixtures, logs or issues."
} as const;

export const layout: readonly (readonly [path: string, responsibility: string])[] = [
  ["`typescript/tsc/`", "The compiler: a fork of the TypeScript 7 Go compiler with VibeLang's checker, lowering and language server"],
  ["`typescript/vibelang/cases/`", "Compiler acceptance cases, run by `typescript/vibelang/smoke.sh`"],
  ["`packages/compiler/`", "The `vibelang` npm package: launcher, editor extension and packaging"],
  ["`packages/runtime/`", "`vibelang-runtime`: the continuation kernel and generated-code helpers"],
  ["`packages/create-vibelang/`, `templates/app/`", "The starter generator and the starter app"],
  ["`packages/vite/`, `packages/unplugin/`", "Vite, webpack, Rspack and other bundler integrations"],
  ["`packages/ts-mapper/`, `packages/converter-worker/`", "TypeScript mapping utilities and the converter's compile service"],
  ["`editors/`", "VS Code and Cursor extension, Neovim and Zed setup, tree-sitter grammar"],
  ["`examples/`", "Runnable example projects"],
  ["`docs/user/`, `docs/design/`", "User guides, the specification and the decision log"],
  ["`docs/tools/`, `site/`", "Docs checks and the vibelang.sh site"],
  ["`scripts/`", "Setup, end-to-end checks and release scripts"],
  ["`experiments/`, `prototype/`", "The continuation kernel's law suite and earlier compilers"],
  ["`.smithers/`", "Generated-docs flow and the README's token comparison"]
];

export const tests: readonly (readonly [change: string, command: string])[] = [
  ["Install, compiler, runtime or starter", "`{{e2e}}`"],
  ["Compiler semantics", "`(cd typescript && sh vibelang/smoke.sh)`"],
  ["Compiler package", "`(cd packages/compiler && node --test test/*.test.mjs)`"],
  ["Doc code blocks", "`bun docs/tools/check-snippets.ts`"],
  ["Examples", "`sh scripts/check-examples.sh`"],
  ["Continuation kernel", "`(cd experiments && bun test)`"],
  ["These docs", "`(cd .smithers && bun install && bun run docs:check)`"]
];

export const labels: readonly (readonly [label: string, meaning: string])[] = [
  ["`bug`", "A bug report, from the bug form"],
  ["`enhancement`", "An idea, from the idea form"],
  ["`question`", "A question, from the question form"],
  ["`documentation`", "Guide, reference or example work"],
  ["`needs-repro`", "The report has no reproduction that shows the bug on the latest version"],
  ["`accepted`", "Accepted for discussion; its proof of concept is generated and linked"],
  ["`timeline: weeks`", "A maintainer expects it to land within weeks"],
  ["`timeline: months`", "A maintainer expects it to land within months"],
  ["`timeline: unscheduled`", "No estimate yet"],
  ["`commitment: committed`", "Maintainers plan to build it"],
  ["`commitment: open`", "Maintainers would merge a good implementation but don't plan to build it; replaces `help wanted`"],
  ["`commitment: undecided`", "Under discussion; it may be declined"],
  ["`good first issue`", "A bounded task with a clear result and test"],
  ["`in-progress`", "Someone is working on it"],
  ["`duplicate`, `invalid`, `wontfix`", "Closing reasons; say why and link what replaces it"]
];

export const releaseSteps: readonly string[] = [
  "**Align versions.** Every package version must equal the compiler's, in `typescript/tsc/internal/vibelang/identity.go`; `make-packages.mjs` refuses a mismatch. Review breaking changes.",
  "**Check.** Run setup, `{{e2e}}`, the compiler cases, affected package tests, editor tests and `bun docs/tools/check-snippets.ts`. Record commands, toolchain versions and results. A skipped platform is untested.",
  "**Build.** `sh scripts/release/build-binaries.sh` cross-compiles the six platform binaries. `node scripts/release/make-packages.mjs` assembles the eleven packages and their tarballs in `dist-npm/`. Check manifests, the bundled extension, licenses and notices.",
  "**Rehearse.** `sh scripts/release/rehearse.sh` publishes the tarballs to a throwaway registry, then scaffolds, builds, starts and tests an app with npm and with Bun. It runs on one host, so it can't prove the other platforms' binaries work.",
  "**Publish.** A person runs `sh scripts/release/publish.sh --dry-run`, then `sh scripts/release/publish.sh`. It publishes the platform packages first and skips versions already published, so a rerun resumes. Keep credentials out of recordings and logs.",
  "**Verify.** `sh scripts/release/verify-published.sh <version>` scaffolds, installs, builds, starts and tests an app from the public registry. Publish release notes with exact versions, breaking changes and known limitations."
];

export const contributing: Document = {
  title: "Contributing to {{name}}",
  intro: "Bug reports with a minimal reproduction, ideas, docs fixes and focused pull requests are welcome. {{name}} is v0, so the language still changes in breaking ways.",
  sections: [
    {
      heading: "How work moves",
      body: `1. **Open an issue.** A [bug report]({{newBug}}) needs a minimal reproduction: the smallest program and command that show the bug on the latest version. An [idea]({{newIdea}}) starts as an issue too, before any code.
2. **A maintainer accepts it for discussion.** The \`accepted\` label opens the design discussion. A \`timeline:\` label gives the maintainer's estimate of when it could land, and a \`commitment:\` label says whether maintainers plan to build it. See [the labels](MAINTAINERS.md#labels).
3. **Accepted issues get a proof of concept.** Accepting an issue generates a working prototype of the change and links it on the issue, so everyone can read it and run it before settling the design. It is a starting point, not a merge candidate.
4. **Open a pull request** that links the accepted issue. A typo fix needs no issue.`
    },
    {
      heading: "Set up",
      body: `From a checkout, with Go, Bun and Node on your \`PATH\`:

\`\`\`sh
{{setup}}
\`\`\`

The tested toolchain is Go 1.26 (setup downloads the Go 1.27 the compiler requires), Bun 1.4.1 and Node 26.10. Setup builds the native compiler into \`packages/compiler/native/vibelang\`, packs the runtime, builds the editor extension, and installs a local starter in \`~/.bun-create/vibelang\`. Apps you then create with \`bun create vibelang my-app\` depend on this checkout. After you change the compiler or runtime, rerun setup, then run \`bun update vibelang-runtime vibelang vibelang-vite\` in those apps. Remove \`~/.bun-create/vibelang\` to scaffold from npm again.

The compiler cases and the doc snippet check also need the pinned packages in \`experiments/\` and \`docs/tools/\`. Install them once:

\`\`\`sh
(cd experiments && bun install)
(cd docs/tools && bun install)
\`\`\``
    },
    { heading: "Find your way around", body: "{{layout}}" },
    {
      heading: "Test your change",
      body: `{{tests}}

The end-to-end check runs setup, then scaffolds, installs, builds, runs and tests a temporary app, and stops at the first failing step. A compiler case is a directory under \`typescript/vibelang/cases/\`; \`typescript/vibelang/harness.mjs\` describes the format. A passing check doesn't prove every language feature works, so list the checks you ran, the ones that failed and the ones you couldn't run.`
    },
    {
      heading: "Pull requests",
      body: `1. Link the accepted issue: \`Closes #123\`.
2. Change one thing. Add a test that fails without the change, and update the examples, diagnostics and docs it affects.
3. Describe the behavior before and after, any syntax or API break, and the commands you ran with their results.
4. Open a draft for early feedback. {{merge}} Maintainers squash-merge each pull request.`
    },
    {
      heading: "Style",
      body: `{{style}} Tests assert behavior, including rejected programs and their diagnostics. {{secrets}}

{{generated}}.`
    },
    {
      heading: "First contributions",
      body: `Start with [good first issues]({{issues}}?q=is%3Aopen+label%3A%22good+first+issue%22) or [documentation]({{issues}}?q=is%3Aopen+label%3Adocumentation). A reduced reproduction of a reported bug or a correction to a guide is a full contribution. Ask on the issue before you start a large change.`
    },
    {
      heading: "Questions and security",
      body: `Ask with the [question form]({{newQuestion}}). Report a vulnerability privately, never in an issue: see [SECURITY.md](.github/SECURITY.md).`
    }
  ]
};

export const maintainers: Document = {
  title: "Maintaining {{name}}",
  intro: "Maintainers triage issues, accept work for discussion, review pull requests and ship releases.",
  sections: [
    {
      heading: "Maintainers",
      body: `| Maintainer | Areas |
| --- | --- |
| Will Cory, [@roninjin10](https://github.com/roninjin10) | Everything: language, compiler, runtime, editors, docs, releases and security reports |

Add a maintainer here, with their areas and a backup, when they agree to review those areas. Commit history doesn't make someone an owner.`
    },
    {
      heading: "Triage",
      body: `Give every new issue one outcome:

- **Needs a reproduction.** If a bug report's reproduction is missing or doesn't show the bug on the latest version, add \`needs-repro\` and say what is missing.
- **Accepted.** Add \`accepted\`, one \`timeline:\` label and one \`commitment:\` label. Acceptance generates the issue's proof of concept and links it on the issue; read it before you discuss the design.
- **Closed.** Use \`duplicate\`, \`invalid\` or \`wontfix\`, with a reason and a link.

Classify a reproduced bug as a compiler crash, a wrong emitted program, a wrong diagnostic or a docs error. An install failure or a wrong emitted program blocks the next release until a maintainer assesses it.

Timeline and commitment labels state a maintainer's current belief, not a promise. When the belief changes, change the label and say why on the issue.`
    },
    { heading: "Labels", body: "{{labels}}" },
    {
      heading: "Review and merge",
      body: `One accepted issue becomes one focused pull request, with a test that fails without the change, the checks it affects, and updated docs. Syntax, emitted declarations, runtime semantics and diagnostics need a compatibility review. {{generated}}.

{{merge}} Squash-merge. Never rewrite \`main\`'s history or republish a package version.`
    },
    {
      heading: "Release process",
      body: `{{releaseSteps}}

Never call a cross-compiled binary tested on an operating system it didn't run on. If a release is broken, say so in its release notes and ship a fixed version through the same steps.`
    },
    {
      heading: "Security reports",
      body: "Reports arrive through GitHub's private vulnerability reporting; see [SECURITY.md](.github/SECURITY.md). If exploit details appear in a public issue, hide them and continue in a private advisory. A maintainer coordinates the fix, the advisory and the release."
    },
    {
      heading: "Benchmark claims",
      body: "Publish a performance or token claim only with a public, reproducible method: pinned versions, inputs, units, results and limits. The README's [token comparison](.smithers/token-compare/README.md) is the model. Keep held-out tasks, hidden tests and transcripts private; a private benchmark is not public evidence."
    }
  ]
};

/** The root AGENTS.md, for agents working on this repository. Every line costs tokens on every turn. */
export const agents: Document = {
  title: "{{name}} repository",
  intro: `Language rules: {{languageRules}}. Specification: \`docs/design/src/\`. Process: CONTRIBUTING.md.

- Work on one accepted issue. Add a test that fails without your change; compiler semantics get a case in \`typescript/vibelang/cases/\`.
- Run \`{{e2e}}\` for install, compiler, runtime or starter changes. Report exact results, including failures and checks you couldn't run.
- A file that says it is generated changes only through its source.
- Match nearby code, format Go with gofmt, and don't reformat unrelated code.
- {{secrets}}
- Agents never approve or merge.`,
  sections: []
};

/** templates/app/AGENTS.md, copied into every new app. It carries docs/user/llms-essentials.md verbatim and stays under 2 KB. */
export const starterAgents = {
  title: "{{name}} project",
  intro: "`bun run build` compiles `.vibe` files to `dist/`; `bun run test` runs `src/**/*.test.vibe`. Docs: {{languageRules}}"
} as const;
