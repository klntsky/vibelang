# Yieldable Errors

Effect's custom error values can be yielded inside TypeScript generators. In Effeck, signal those same domain failures with `throw`; `function*` produces values rather than sequencing an Effect program.

[Upstream page](https://effect.website/docs/v4/error-management/yieldable-errors)

## Defining an error without a tag

```ts
class EmptyDocument extends Data.Error<{
  readonly documentId: string
}> {}

function requireText(documentId: string, text: string) {
  if (text.trim() === "") throw new EmptyDocument({ documentId })
  return text
}
// Effect<string, EmptyDocument, never>
```

`Data.Error` creates an error class with typed fields and Effect's yieldable behavior. Effeck does not require yielding it: an explicit throw puts `EmptyDocument` in `E`. Creating or returning an error object by itself does not fail the computation.

```ts
function emptyDocumentError(documentId: string) {
  return new EmptyDocument({ documentId })
}

function rejectDocument(documentId: string) {
  const error = emptyDocumentError(documentId) // data, not a failure
  return error()                             // calling the error fails with it
}
// Effect<never, EmptyDocument, never>
```

`new` only constructs, and a call returning an error instance returns data. Calling the held error runs its failure behavior, just as `throw error` does.

## Creating tagged error classes

```ts
error EmptyTitle
error TitleTooLong {
  length: number
}

function title(text: string) {
  if (text.length === 0) throw new EmptyTitle()
  if (text.length > 80) throw new TitleTooLong({ length: text.length })
  return text
}
// Effect<string, EmptyTitle | TitleTooLong, never>
```

An `error` declaration is a class that extends the global `TaggedError`, Effect's `Data.TaggedError`, which needs no import. Every instance has a readonly `_tag` equal to the class name, and the class names the instance type as well as constructing values. Use `Schema.TaggedError` when an error also needs schema-based serialization.

Choose distinct tags for distinct error cases. Tag-based matching compares strings, so two classes with the same tag cannot be distinguished by `catchTag` or `_tag` narrowing; do not assume a duplicate tag is diagnosed.

## Recovering failures by tag

```ts
function displayTitle(text: string) {
  return Effect.catchTag(title(text), "EmptyTitle", () => "Untitled")
}
// Effect<string, TitleTooLong, never>

function editableTitle(text: string) {
  try {
    return title(text)
  } catch (error) {
    if (error._tag === "EmptyTitle") return "Untitled"
    throw error
  }
}
// Effect<string, TitleTooLong, never>
```

Both versions remove `EmptyTitle` from `E` while preserving `TitleTooLong`. `Effect.catchTags` supports one handler per tag when several cases need different responses. These remain typed failures even though the error classes also support generators in TypeScript.

## Coming from Effect

Existing `.ts` functions that yield `Data.Error` or `Data.TaggedError` stay in TypeScript and are called normally from `.effeck` files. Replace `return yield* new ErrorCase(...)` with `throw new ErrorCase(...)` when writing the Effeck implementation.

An Effeck `function*` is a producer that yields elements. It converts to an Effect `Stream` wherever one is expected; it is not a place to paste `Effect.gen` or `yield*` error sequencing.
