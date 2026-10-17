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
  const effeckIncludes = ["#effeck-type-declaration", "#effeck-member-declaration", "#effeck-layer-declaration", "#effeck-handle-clause"]
  const statements = repository.statements.patterns.filter((pattern) => !effeckIncludes.includes(pattern.include))
  repository.statements.patterns = [...effeckIncludes.map((include) => ({ include })), ...statements]
  // `!` separates the slots of an Effect type: Result!Error!Dependencies.
  const effectSlot = { name: "keyword.operator.type.effect.ts", match: "!" }
  repository.type.patterns = [effectSlot, ...repository.type.patterns.filter((pattern) => pattern.name !== effectSlot.name)]
  return grammar
}
