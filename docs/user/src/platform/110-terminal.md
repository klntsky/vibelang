# Terminal

`Terminal` exposes Effect's interactive input and output service in Effeck. Use it for prompts and line input, provide a runtime terminal layer, and handle a user's quit request as a typed error where appropriate.

## Requesting terminal services

```ts
function dimensions() {
  const terminal = Effect.service(Terminal.Terminal)
  return { columns: terminal.columns(), rows: terminal.rows() }
}
// Effect<{ columns: number; rows: number }, never, Terminal.Terminal>
```

`Terminal.Terminal` is the service key; `Effect.service` retrieves its implementation. `columns`, `rows`, and `readLine` are Effect values, so call them to run them. `display` is an Effect-returning method and runs on a normal method call.

## Writing standard output

```ts
// main.effeck
import { NodeRuntime, NodeTerminal } from "@effect/platform-node"
function main() {
  const terminal = Effect.service(Terminal.Terminal)
  terminal.display("Package prepared.\n")
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeTerminal.layer))
```

`display` writes the string you provide; add a newline when the output should occupy a complete line. Output can fail with `PlatformError`. The Node terminal layer supplies the service without requiring your application to call native standard-output APIs.

## Reading standard input

```ts
function askName() {
  const terminal = Effect.service(Terminal.Terminal)
  terminal.display("Your name: ")
  const name = terminal.readLine()
  terminal.display(`Hello, ${name.trim() || "visitor"}.\n`)
}
// Effect<void, Terminal.QuitError | PlatformError.PlatformError, Terminal.Terminal>
```

`readLine()` waits for a line and returns its text. Reading and display happen in order, so the prompt is issued before input is requested. Low-level `readInput` exposes scoped input events when line-oriented interaction is insufficient.

## Handling terminal quit errors

```ts
function askNameOrQuit() {
  try {
    askName()
  } catch (error) {
    if (Terminal.isQuitError(error)) return
    throw error
  }
}
```

`QuitError` represents a user-requested exit from terminal input, commonly Ctrl+C at a prompt. Handle that error when quitting should count as normal application completion, and rethrow unrelated failures. Host signal interruption is a separate termination path; it still bypasses ordinary `catch` and runs cleanup.

## Building an interactive guessing program

```ts
// guessing.effeck
import { NodeRuntime, NodeTerminal } from "@effect/platform-node"
function parseGuess(input: string) {
  const text = input.trim()
  const number = Number(text)
  return /^\d+$/.test(text) && Number.isInteger(number) &&
    number >= 1 && number <= 20
    ? Option.some(number)
    : Option.none<number>()
}

function game() {
  const terminal = Effect.service(Terminal.Terminal)
  const secret = Random.nextIntBetween(1, 20)
  terminal.display("Guess an integer from 1 to 20. You have five guesses.\n")

  let attempts = 0
  while (attempts < 5) {
    terminal.display(`Guess ${attempts + 1}: `)
    const guess = parseGuess(terminal.readLine())
    if (Option.isNone(guess)) {
      terminal.display("Enter a whole number from 1 to 20.\n")
      continue
    }
    attempts++
    if (guess.value === secret) {
      terminal.display("Correct!\n")
      return
    }
    terminal.display(guess.value < secret ? "Try higher.\n" : "Try lower.\n")
  }
  terminal.display(`The answer was ${secret}.\n`)
}
// Effect<void, Terminal.QuitError | PlatformError.PlatformError, Terminal.Terminal>

function main() {
  try {
    game()
  } catch (error) {
    if (Terminal.isQuitError(error)) return
    throw error
  }
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeTerminal.layer))
```

The random service selects the secret once per game, with both endpoints included. Invalid input repeats the prompt without consuming a guess; valid input increments the attempt count. The loop stops on success, after five valid guesses, or on a quit request, and the runner still reports unexpected output failures.

## Coming from Effect

A held Effect such as `terminal.readLine` is executed with `terminal.readLine()`. Ordinary functions and loops replace the generator and recursive Effect construction used for the same interaction in TypeScript.
