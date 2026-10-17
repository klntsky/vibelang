// Adds Effeck's declaration and statement forms to the grammar derived from
// TypeScript's. Scopes reuse TypeScript's names, so themes color an error
// declaration like a class and a handle clause like catch.
//
//   error NotFound { id: string }          schema User { name: string }
//   service Users { find(id: string): User!NotFound }
//   control Approval { approve(request: string): boolean }
//   layer UsersLive: Users { find(id) { ... } }
//   try { ... } handle Approval.approve(request) { ... }
//   function find(id: string): User!NotFound!Db { ... }

const name = "[_$[:alpha:]][_$[:alnum:]]*"
const start = "(?<![_$[:alnum:]])(?:(?<=\\.\\.\\.)|(?<!\\.))(?:(\\bexport)\\s+)?"
// The declaration ends after its body, or at the end of a line that has no body.
const end = "(?<=\\})|(?<=[_$[:alnum:]>])(?=\\s*(?:$|;|//))"

function declaration(words, body, header = "") {
  return {
    name: "meta.class.ts",
    begin: `${start}\\b(${words})\\s+(${name})${header}`,
    beginCaptures: {
      1: { name: "keyword.control.export.ts" },
      2: { name: "storage.type.class.ts" },
      3: { name: "entity.name.type.class.ts" },
      4: { name: "entity.name.type.ts" }
    },
    end,
    patterns: [{ include: "#comment" }, { include: "#type-parameters" }, { include: body }]
  }
}

export function addEffeckSyntax(grammar) {
  const { repository } = grammar
  repository["effeck-type-declaration"] = declaration("error|schema", "#type-object")
  repository["effeck-member-declaration"] = declaration("service|control", "#class-or-interface-body")
  repository["effeck-layer-declaration"] = declaration("layer", "#class-or-interface-body", `\\s*:\\s*(${name}(?:\\.${name})*)`)
  repository["effeck-handle-clause"] = {
    match: `\\b(handle)\\s+(?=${name}\\s*\\.\\s*${name}\\s*\\()`,
    captures: { 1: { name: "keyword.control.trycatch.ts" } }
  }
  // The fiber forms (D166, D169, D196, D225): fork operand, join operand, await.all [...] and the other await operations, for concurrent (...).
  repository["effeck-fork"] = {
    match: "(?<![_$[:alnum:]])(fork)\\s+(?=[_$[:alpha:]{]|this\\b|new\\b|await\\b)",
    captures: { 1: { name: "keyword.control.flow.ts" } }
  }
  // join operand (D225): join a fiber, or an array or object of fibers. join( stays a call.
  repository["effeck-join"] = {
    match: "(?<![_$[:alnum:].])(join)\\s+(?=[_$[:alpha:]{\\[]|this\\b)",
    captures: { 1: { name: "keyword.control.flow.ts" } }
  }
  repository["effeck-await-operation"] = {
    match: "(?<![_$[:alnum:].])(await)\\s*(\\.)\\s*(all|allSettled|race|any)(?![_$[:alnum:]])",
    captures: {
      1: { name: "keyword.control.flow.ts" },
      2: { name: "punctuation.accessor.ts" },
      3: { name: "keyword.control.flow.ts" }
    }
  }
  // provide Layer { ... } (D130, D172): the keyword at the start of a statement.
  repository["effeck-provide"] = {
    match: "^\\s*(provide)(?=\\s+[_$[:alpha:]\\[])",
    captures: { 1: { name: "keyword.control.with.ts" } }
  }
  repository["effeck-for-concurrent"] = {
    match: "(?<![_$[:alnum:]])(for)\\s+(concurrent)(?![_$[:alnum:]])",
    captures: { 1: { name: "keyword.control.loop.ts" }, 2: { name: "keyword.control.loop.ts" } }
  }
  // Pattern matching (D206-D225): match (x) { when P: v; }, x is P, and the
  // and, or, and not combinators before a pattern.
  repository["effeck-match"] = {
    patterns: [
      { match: "(?<![_$[:alnum:].])(match)\\s*(?=\\(.*\\)\\s*\\{)", captures: { 1: { name: "keyword.control.switch.ts" } } },
      { match: "^\\s*(when)(?=\\s)", captures: { 1: { name: "keyword.control.switch.ts" } } },
      { match: "(?<=[_$[:alnum:])\\]])\\s+(is)(?=\\s+[^\\s=])", captures: { 1: { name: "keyword.operator.expression.instanceof.ts" } } },
      { match: "(?<=[\\s(])(and|or|not)(?=\\s+(?:let|const|var|not|if|[\\[{(\"'`0-9]|[A-Z]))", captures: { 1: { name: "keyword.operator.expression.instanceof.ts" } } }
    ]
  }
  const effeckIncludes = ["#effeck-type-declaration", "#effeck-member-declaration", "#effeck-layer-declaration", "#effeck-handle-clause", "#effeck-provide", "#effeck-for-concurrent", "#effeck-fork", "#effeck-join"]
  const statements = repository.statements.patterns.filter((pattern) => !effeckIncludes.includes(pattern.include))
  repository.statements.patterns = [...effeckIncludes.map((include) => ({ include })), ...statements]
  const effeckExpressions = ["#effeck-await-operation", "#effeck-fork", "#effeck-join", "#effeck-match"]
  const expressions = repository.expression.patterns.filter((pattern) => !effeckExpressions.includes(pattern.include))
  repository.expression.patterns = [...effeckExpressions.map((include) => ({ include })), ...expressions]
  // `!` separates the slots of an Effect type: Result!Error!Dependencies.
  const effectSlot = { name: "keyword.operator.type.effect.ts", match: "!" }
  repository.type.patterns = [effectSlot, ...repository.type.patterns.filter((pattern) => pattern.name !== effectSlot.name)]
  return grammar
}
