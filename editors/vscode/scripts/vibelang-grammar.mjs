// Adds VibeLang's declaration and statement forms to the grammar derived from
// TypeScript's. Scopes reuse TypeScript's names, so themes color an error
// declaration like a class and a handle clause like catch.
//
//   error NotFound { id: string }          schema User { name: string }
//   schema Code = "a" | "b"                (D280, read as a type alias)
//   error Missing extends NotFound { path: string; describe(): string { ... } }
//   schema error FsError { method: string }
//   service Users { find(id: string): User!NotFound }
//   control Approval { approve(request: string): boolean }
//   layer UsersLive: Users { find(id) { ... } }   (the old spelling, D221)
//   try { ... } handle Approval.approve(request) { ... }
//   getUser(id) catch (e) => match (e) { when NotFound: guest; }
//   function find(id: string): User!NotFound!Db { ... }
//   module text { ...TypeScript... }       import { clean } from text

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

export function addVibeLangSyntax(grammar) {
  const { repository } = grammar
  // schema Code = "a" | "b" (D280) is TypeScript's type alias with schema for
  // type: its = and type are the alias's, and it ends where the alias does
  // or at a line that starts another VibeLang declaration.
  const alias = JSON.parse(
    JSON.stringify(repository["type-alias-declaration"]).replaceAll("(?:abstract|", "(?:error|service|control|schema|layer|abstract|")
  )
  repository["vibelang-schema-alias"] = {
    ...alias,
    begin: `${start}\\b(schema)\\s+(${name})(?=\\s*=)\\s*`,
    beginCaptures: {
      1: { name: "keyword.control.export.ts" },
      2: { name: "storage.type.type.ts" },
      3: { name: "entity.name.type.alias.ts" }
    }
  }
  repository["vibelang-type-declaration"] = declaration("schema", "#type-object")
  // error and schema error declare classes (D126, D278, D279): abstract,
  // extends and implements, and a class body, whose fields read as properties.
  repository["vibelang-error-declaration"] = {
    name: "meta.class.ts",
    begin: `${start}(?:(abstract)\\s+)?(?:(schema)\\s+)?\\b(error)\\s+(${name})`,
    beginCaptures: {
      1: { name: "keyword.control.export.ts" },
      2: { name: "storage.modifier.ts" },
      3: { name: "storage.type.class.ts" },
      4: { name: "storage.type.class.ts" },
      5: { name: "entity.name.type.class.ts" }
    },
    end,
    patterns: [
      { include: "#comment" },
      { include: "#type-parameters" },
      { include: "#vibelang-error-heritage" },
      { include: "#class-or-interface-body" }
    ]
  }
  // An error's extends and implements end at the line's end too, since an
  // error without fields or members has no body: error Gone extends Http.
  const heritage = repository["class-or-interface-heritage"]
  repository["vibelang-error-heritage"] = {
    ...heritage,
    end: "(?=\\{)|$",
    patterns: heritage.patterns.map((pattern) =>
      pattern.include === "#class-or-interface-heritage" ? { include: "#vibelang-error-heritage" } : pattern
    )
  }
  // service and control (D128, D223); the older spelling with class
  // (D220) matches too.
  repository["vibelang-member-declaration"] = {
    name: "meta.class.ts",
    begin: `${start}\\b(service|control)\\s+(?:(class)\\s+)?(${name})`,
    beginCaptures: {
      1: { name: "keyword.control.export.ts" },
      2: { name: "storage.modifier.ts" },
      3: { name: "storage.type.class.ts" },
      4: { name: "entity.name.type.class.ts" }
    },
    end,
    patterns: [{ include: "#comment" }, { include: "#type-parameters" }, { include: "#class-or-interface-body" }]
  }
  repository["vibelang-layer-declaration"] = declaration("layer", "#class-or-interface-body", `\\s*:\\s*(${name}(?:\\.${name})*)`)
  repository["vibelang-handle-clause"] = {
    match: `\\b(handle)\\s+(?=${name}\\s*\\.\\s*${name}\\s*\\()`,
    captures: { 1: { name: "keyword.control.trycatch.ts" } }
  }
  // The fiber forms (D166, D169, D196, D225): fork operand, join operand, await.all [...] and the other await operations, for concurrent (...).
  repository["vibelang-fork"] = {
    match: "(?<![_$[:alnum:]])(fork)\\s+(?=[_$[:alpha:]{]|this\\b|new\\b|await\\b)",
    captures: { 1: { name: "keyword.control.flow.ts" } }
  }
  // join operand (D225): join a fiber, or an array or object of fibers. join( stays a call.
  repository["vibelang-join"] = {
    match: "(?<![_$[:alnum:].])(join)\\s+(?=[_$[:alpha:]{\\[]|this\\b)",
    captures: { 1: { name: "keyword.control.flow.ts" } }
  }
  repository["vibelang-await-operation"] = {
    match: "(?<![_$[:alnum:].])(await)\\s*(\\.)\\s*(all|allSettled|race|any)(?![_$[:alnum:]])",
    captures: {
      1: { name: "keyword.control.flow.ts" },
      2: { name: "punctuation.accessor.ts" },
      3: { name: "keyword.control.flow.ts" }
    }
  }
  // provide Layer { ... } (D130, D172): the keyword at the start of a statement.
  repository["vibelang-provide"] = {
    match: "^\\s*(provide)(?=\\s+[_$[:alpha:]\\[])",
    captures: { 1: { name: "keyword.control.with.ts" } }
  }
  repository["vibelang-for-concurrent"] = {
    match: "(?<![_$[:alnum:]])(for)\\s+(concurrent)(?![_$[:alnum:]])",
    captures: { 1: { name: "keyword.control.loop.ts" }, 2: { name: "keyword.control.loop.ts" } }
  }
  // Pattern matching (D206-D225): match (x) { when P: v; }, x is P, and the
  // and, or, and not combinators before a pattern.
  repository["vibelang-match"] = {
    patterns: [
      { match: "(?<![_$[:alnum:].])(match)\\s*(?=\\(.*\\)\\s*\\{)", captures: { 1: { name: "keyword.control.switch.ts" } } },
      { match: "^\\s*(when)(?=\\s)", captures: { 1: { name: "keyword.control.switch.ts" } } },
      { match: "(?<=[_$[:alnum:])\\]])\\s+(is)(?=\\s+[^\\s=])", captures: { 1: { name: "keyword.operator.expression.instanceof.ts" } } },
      { match: "(?<=[\\s(])(and|or|not)(?=\\s+(?:let|const|var|not|if|[\\[{(\"'`0-9]|[A-Z]))", captures: { 1: { name: "keyword.operator.expression.instanceof.ts" } } }
    ]
  }
  // A catch expression (D236): x catch fallback, x catch (e) => handler. The
  // keyword follows an expression on the same line; after a try block's },
  // catch keeps TypeScript's scope from the control statements.
  repository["vibelang-catch-expression"] = {
    match: "(?<=[_$[:alnum:])\\]'\"`]\\s)(catch)(?=\\s)",
    captures: { 1: { name: "keyword.control.trycatch.ts" } }
  }
  // A module declaration (D283) at a file's top level. Its body is
  // TypeScript, so VS Code's TypeScript grammar, source.ts, colors it, and
  // decides which brace ends it. declare module x, module a.b and
  // module "x" keep TypeScript's namespace-declaration, as in the parser.
  repository["vibelang-module-declaration"] = {
    name: "meta.namespace.declaration.ts",
    begin: `${start}\\b(module)\\s+(${name})(?=\\s*(?:$|\\{|//|/\\*))`,
    beginCaptures: {
      1: { name: "keyword.control.export.ts" },
      2: { name: "storage.type.namespace.ts" },
      3: { name: "entity.name.type.module.ts" }
    },
    end: repository["namespace-declaration"].end,
    patterns: [
      { include: "#comment" },
      {
        name: "meta.block.ts",
        begin: "\\{",
        beginCaptures: { 0: { name: "punctuation.definition.block.ts" } },
        end: "\\}",
        endCaptures: { 0: { name: "punctuation.definition.block.ts" } },
        patterns: [{ include: "source.ts" }]
      }
    ]
  }
  grammar.patterns = [{ include: "#vibelang-module-declaration" }, ...grammar.patterns.filter((pattern) => pattern.include !== "#vibelang-module-declaration")]
  // An identifier module specifier names a module declaration: from text,
  // and import text, which imports the module for its effects.
  const specifier = `(${name})(?=\\s*(?:$|;|//|/\\*))`
  repository["vibelang-module-specifier"] = {
    match: `(?<=\\bfrom\\s)\\s*${specifier}`,
    captures: { 1: { name: "entity.name.type.module.ts" } }
  }
  const moduleSpecifier = { include: "#vibelang-module-specifier" }
  const importExport = repository["import-export-declaration"]
  importExport.patterns = [moduleSpecifier, ...importExport.patterns.filter((pattern) => pattern.include !== moduleSpecifier.include)]
  repository["vibelang-module-import"] = {
    name: "meta.import.ts",
    match: `(?<![_$[:alnum:]])(?:(?<=\\.\\.\\.)|(?<!\\.))\\b(import)\\s+${specifier}`,
    captures: { 1: { name: "keyword.control.import.ts" }, 2: { name: "entity.name.type.module.ts" } }
  }
  const vibelangIncludes = ["#vibelang-module-import", "#vibelang-error-declaration", "#vibelang-schema-alias", "#vibelang-type-declaration", "#vibelang-member-declaration", "#vibelang-layer-declaration", "#vibelang-handle-clause", "#vibelang-provide", "#vibelang-for-concurrent", "#vibelang-fork", "#vibelang-join"]
  const statements = repository.statements.patterns.filter((pattern) => !vibelangIncludes.includes(pattern.include))
  repository.statements.patterns = [...vibelangIncludes.map((include) => ({ include })), ...statements]
  const vibelangExpressions = ["#vibelang-await-operation", "#vibelang-fork", "#vibelang-join", "#vibelang-match", "#vibelang-catch-expression"]
  const expressions = repository.expression.patterns.filter((pattern) => !vibelangExpressions.includes(pattern.include))
  repository.expression.patterns = [...vibelangExpressions.map((include) => ({ include })), ...expressions]
  // `!` separates the slots of an Effect type: Result!Error!Dependencies.
  const effectSlot = { name: "keyword.operator.type.effect.ts", match: "!" }
  repository.type.patterns = [effectSlot, ...repository.type.patterns.filter((pattern) => pattern.name !== effectSlot.name)]
  return grammar
}
