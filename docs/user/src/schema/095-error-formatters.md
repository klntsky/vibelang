# Error Formatters

Schema issue formatters work as in Effect. Use readable strings for diagnostics and structured issue paths for forms or other validation clients. These formatters describe failures; [value formatters](103-formatter.md) describe successful values. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Rendering schema issues as readable strings

```vibe
const Dispatch = Schema.Struct({
  destination: Schema.String,
  packages: Schema.Int
})

function readableFailure(input: unknown) {
  try {
    Schema.decodeUnknownEffect(Dispatch)(input)
    return "Accepted"
  } catch (error) {
    return try((error) => SchemaIssue.makeFormatterDefault()(error.issue), error)
  }
}
```

`makeFormatterDefault()` creates a function from an issue to a multiline string. Paths identify nested keys and indices. `SchemaError.message` already uses this format, so reading it directly is enough when no custom formatting is required.

## Reporting several issues

```vibe
function allFailures(input: unknown) {
  try {
    Schema.decodeUnknownEffect(Dispatch)(input, { errors: "all" })
    return "Accepted"
  } catch (error) {
    return error.message
  }
}
// {} reports missing destination and packages fields.
```

Formatting cannot recover issues that parsing did not collect. Set `errors: "all"` on the parser to collect multiple independent failures. Nested prerequisites still apply: a struct-level check cannot run until its input has been validated.

## Formatting Standard Schema validation errors

```vibe
function formFailures(input: unknown) {
  try {
    Schema.decodeUnknownEffect(Dispatch)(input, { errors: "all" })
    return []
  } catch (error) {
    return try((error) => SchemaIssue.makeFormatterStandardSchemaV1()(error.issue).issues, error)
  }
}
// An issue has a message and a path such as ["destination"].
```

The Standard Schema formatter produces a failure object with an `issues` array. Leaf issues carry a full path, making it possible to associate each message with a form field. Keep paths structured when field names contain dots or when array indices matter; joining path segments into a string can lose that distinction.

## Customizing leaf messages

```vibe
const formFormatter = SchemaIssue.makeFormatterStandardSchemaV1({
  leafHook: (issue) => issue._tag === "MissingKey"
    ? "Complete this field"
    : SchemaIssue.defaultLeafHook(issue)
})

function localizedFailures(input: unknown) {
  try {
    Schema.decodeUnknownEffect(Dispatch)(input, { errors: "all" })
    return []
  } catch (error) {
    return try((formFormatter, error) => formFormatter(error.issue).issues, formFormatter, error)
  }
}
```

`leafHook` replaces selected terminal messages. Delegate unhandled cases to `defaultLeafHook` so built-in messages continue to work. Keep the hook synchronous and free of service calls; load translated strings before creating it.

## Connecting validation to form libraries

```vibe
const DispatchForm = Schema.toStandardSchemaV1(Dispatch, {
  parseOptions: { errors: "all" },
  leafHook: (issue) => issue._tag === "MissingKey"
    ? "Complete this field"
    : SchemaIssue.defaultLeafHook(issue)
})

function formSchema() {
  return DispatchForm
}
```

Pass the adapter to a form library that accepts Standard Schema V1, keeping the UI integration in its normal TypeScript module. The adapter exposes structured issues and also retains the original Schema operations. Its decoding services must be `never`.

React Hook Form has an `effectTsResolver` in `@hookform/resolvers`; verify that the installed resolver supports Effect 4 before connecting it. A resolver tied to an earlier Effect API is not interchangeable with the Standard Schema adapter. The schema and formatters above remain useful when writing a small form-library adapter yourself.

[Upstream page](https://effect.website/docs/v4/schema/error-formatters)
