# Pattern Matching

VibeLang matches values with `match` and tests them with `is`, from the TC39 pattern matching proposal. Both compile to plain conditionals: TypeScript narrows the value in each arm, and an arm can call Effect functions. [Pattern matching with match and is](../vibelang-features/116-match-and-is.md) lists every pattern. Effect's `Match` module still builds reusable matcher functions.

## Matching values

```vibe
function describeInput(input: number | string) {
  return match (input) {
    when Number: `count ${input}`;
    when String: `label ${input}`;
  }
}

function describeJob(input: { state: string; attempts: number }) {
  return match (input) {
    when { state: "queued" }: `waiting: ${input.attempts}`;
    default: `state: ${input.state}`;
  }
}
```

Arms are tried in order, and the first arm whose pattern matches gives the value. `Number` and `String` test `typeof`. An object pattern checks only the properties it names.

## Shapes, guards, and tags

```vibe
function classifyRetry(input: { status: string; attempts: number }) {
  return match (input) {
    when { status: "failed", attempts: let n } and if (n >= 3): "escalate";
    when { status: "failed" }: "retry";
    default: "leave unchanged";
  }
}

type Job =
  | { _tag: "Queued" }
  | { _tag: "Running"; worker: string }
  | { _tag: "Done"; output: string }

function jobLabel(job: Job) {
  return match (job) {
    when { _tag: "Queued" }: "waiting";
    when { _tag: "Running", let worker }: `worker ${worker}`;
    when { _tag: "Done", let output }: output;
  }
}
```

`let n` binds a property, and a guard, `if (condition)`, tests the bindings before it. Matching `_tag` narrows a tagged union. An `error` class is itself a pattern: `when NotFound:` matches its instances.

## Negating and combining patterns

```vibe
function availability(status: "active" | "paused" | "closed") {
  return match (status) {
    when not "closed": "can reopen";
    default: "archived";
  }
}

function weekend(day: string) {
  return match (day) {
    when "Saturday" or "Sunday": true;
    default: false;
  }
}
```

`not` matches what its pattern does not; `or` matches either pattern. Order still matters: a broad arm can consume inputs that a later arm was meant to handle.

## Fixing the return type

```vibe
function display(input: number | boolean): string {
  return match (input) {
    when Number: input.toFixed(1);
    when Boolean: input ? "enabled" : "disabled";
  }
}
```

A match's type is the union of its arms' types. Annotate the function's return type to require one type from every arm, which catches an arm that returns a number from a text formatter.

## Exhaustive handling

```vibe
type Phase = "draft" | "published"

function phaseLabel(phase: Phase) {
  return match (phase) {
    when "draft": "Draft";
    when "published": "Published";
  }
}
```

A match without a `default` arm over a finite union, such as literals, `error` classes, `Option`, `Exit`, or `Result`, must handle every case. Adding a member to `Phase` makes this match EK4701 until it gets an arm. A guard does not prove that every value of a type is covered. Over other types, a match that no arm fits throws a `TypeError` when it runs.

## Defaults, optional outputs, and result outputs

```vibe
type Command = "start" | "stop" | "help"

function withDefault(command: Command) {
  return match (command) {
    when "start": "starting";
    default: `unhandled: ${command}`;
  }
}

function optionalCommand(command: Command) {
  return match (command) {
    when "start": Option.some("starting");
    default: Option.none();
  }
}

function commandResult(command: Command) {
  return match (command) {
    when "start": Result.succeed("starting");
    default: Result.fail(command);
  }
}
```

A `default` arm comes last and receives every unmatched input, narrowed to the cases no arm took. Return an `Option` or a `Result` from the arms when the caller needs to know whether a case matched. That failed Result is a returned value, not a typed Effect failure.

## Reusable matchers with the Match module

```vibe
const describe = pipe(
  Match.type<number | string>(),
  Match.when(Match.number, (n) => `count ${n}`),
  Match.when(Match.string, (s) => `label ${s}`),
  Match.exhaustive
)

function describeAll(inputs: ReadonlyArray<number | string>) {
  return inputs.map(describe)
}
```

`Match.type<T>()` builds a matcher without choosing an input, and its finalizer produces a plain function to pass as a callback. Match callbacks are synchronous value computations, and Match functions are trusted Effect library functions: their exceptions are defects that an ordinary `catch` does not recover. Use `match` when an arm performs Effect work.
