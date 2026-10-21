# Custom Error Classes

Effect's custom error values can be yielded inside TypeScript generators. In VibeLang, signal those same domain failures with `throw`; `function*` produces values rather than sequencing an Effect program.

[Upstream page](https://effect.website/docs/v4/error-management/yieldable-errors)

## Defining an error without a tag

```vibe
// run: Console.log(requireText("doc-1", "Hello")); Console.log(Effect.flip(() => requireText("doc-1", " ")).documentId)
class EmptyDocument extends Data.Error<{
  readonly documentId: string
}> {}

function requireText(documentId: string, text: string) {
  if (text.trim() === "") throw new EmptyDocument({ documentId })
  return text
}
// string!EmptyDocument!
```

```output
Hello
doc-1
```

`Data.Error` creates an error class with typed fields and Effect's yieldable behavior. VibeLang does not require yielding it: an explicit throw puts `EmptyDocument` in `E`. Creating or returning an error object by itself does not fail the computation.

```vibe
// run: Console.log(emptyDocumentError("doc-1").documentId); Console.log(Effect.flip(() => rejectDocument("doc-1")).documentId)
function emptyDocumentError(documentId: string) {
  return new EmptyDocument({ documentId })
}

function rejectDocument(documentId: string) {
  const error = emptyDocumentError(documentId) // data, not a failure
  return error()                             // calling the error fails with it
}
// never!EmptyDocument!
```

```output
doc-1
doc-1
```

`new` only constructs, and a call returning an error instance returns data. Calling the held error runs its failure behavior, just as `throw error` does.

## Creating tagged error classes

```vibe
// run: Console.log(title("Draft")); Console.log(Effect.flip(() => title(""))._tag); const error = Effect.flip(() => title("x".repeat(81))); Console.log(error is TitleTooLong ? error.length : 0)
error EmptyTitle
error TitleTooLong {
  length: number
}

function title(text: string) {
  if (text.length === 0) throw new EmptyTitle()
  if (text.length > 80) throw new TitleTooLong({ length: text.length })
  return text
}
// string!EmptyTitle | TitleTooLong!
```

```output
Draft
EmptyTitle
81
```

An `error` declaration is a class that extends the global `TaggedError`, Effect's `Data.TaggedError`, which needs no import. Every instance has a readonly `_tag` equal to the class name, and the class names the instance type as well as constructing values. Use `Schema.TaggedError` when an error also needs schema-based serialization.

Choose distinct tags for distinct error cases. Tag-based matching compares strings, so two classes with the same tag cannot be distinguished by `catchTag` or `_tag` narrowing; do not assume a duplicate tag is diagnosed. `is` and `match` test the class with `instanceof`, so they do tell such classes apart.

## Recovering failures by tag

```vibe
// run: Console.log(displayTitle("")); Console.log(editableTitle("")); Console.log(displayTitle("Draft")); Console.log(editableTitle("Draft")); Console.log(Effect.flip(() => displayTitle("x".repeat(81)))._tag); Console.log(Effect.flip(() => editableTitle("x".repeat(81)))._tag)
function displayTitle(text: string) {
  return Effect.catchTag(title(text), "EmptyTitle", () => "Untitled")
}
// string!TitleTooLong!

function editableTitle(text: string) {
  try {
    return title(text)
  } catch (error) {
    if (error is EmptyTitle) return "Untitled"
    throw error
  }
}
// string!TitleTooLong!
```

```output
Untitled
Untitled
Draft
Draft
TitleTooLong
TitleTooLong
```

Both versions remove `EmptyTitle` from `E` while preserving `TitleTooLong`. `Effect.catchTags` supports one handler per tag when several cases need different responses. These remain typed failures even though the error classes also support generators in TypeScript.

## Coming from Effect

Existing `.ts` functions that yield `Data.Error` or `Data.TaggedError` stay in TypeScript and are called normally from `.vibe` files. Replace `return yield* new ErrorCase(...)` with `throw new ErrorCase(...)` when writing the VibeLang implementation.

In a `.vibe` file, `class NotFound extends Data.TaggedError("NotFound")<{ … }> {}` draws EK4830, whose quick fix writes `error NotFound { … }`; a `Schema.TaggedError` class draws EK4837, without a fix, because an error declaration is not a Schema ([TypeScript-style Effect code](../getting-started/012-devtools.md#typescript-style-effect-code)).

A VibeLang `function*` is a producer that yields elements. It converts to an Effect `Stream` wherever one is expected; it is not a place to paste `Effect.gen` or `yield*` error sequencing.
