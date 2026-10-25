# Terminal

`Terminal` is Effect's interactive input and output service. Use it for prompts and line input, provide a runtime terminal layer, and handle a user's quit request as a typed error where it fits.

## Requesting terminal services

```vibe
// run: provide Terminal.Terminal = Terminal.make({ columns: () => 80, rows: () => 24, readInput: () => Queue.unbounded<Terminal.UserInput>(), readLine: () => " Ada ", display: (text) => Console.log(text.trimEnd()) }) { Console.log(JSON.stringify(dimensions())) }
function dimensions() {
  const terminal = Effect.service(Terminal.Terminal)
  return { columns: terminal.columns(), rows: terminal.rows() }
}
// { columns: number; rows: number }!!Terminal.Terminal
// Output: {"columns":80,"rows":24}
```

`Terminal.Terminal` is the service key; `Effect.service` retrieves its implementation. `columns`, `rows`, and `readLine` are Effect values: call them to run them. `display` is an Effect-returning method and runs on an ordinary call.

## Writing standard output

```vibe
// main.vibe
import { NodeRuntime, NodeTerminal } from "@effect/platform-node"
function main() {
  const terminal = Effect.service(Terminal.Terminal)
  terminal.display("Package prepared.\n")
}

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeTerminal.layer))
// Output: Package prepared.
```

`display` writes the string you pass; add a newline to complete a line. Output can fail with `PlatformError`. The Node terminal layer supplies the service, so the application needn't call native standard-output APIs.

## Reading standard input

```vibe
// run: provide Terminal.Terminal = Terminal.make({ columns: () => 80, rows: () => 24, readInput: () => Queue.unbounded<Terminal.UserInput>(), readLine: () => " Ada ", display: (text) => Console.log(text.trimEnd()) }) { askName() }
function askName() {
  const terminal = Effect.service(Terminal.Terminal)
  terminal.display("Your name: ")
  const name = terminal.readLine()
  terminal.display(`Hello, ${name.trim() || "visitor"}.\n`)
}
// void!Terminal.QuitError | PlatformError.PlatformError!Terminal.Terminal
// Output: Your name:
// Output: Hello, Ada.
```

`readLine()` waits for a line and returns its text. Reads and displays run in order, so the prompt appears before input is requested. When line input isn't enough, low-level `readInput` exposes scoped input events.

## Handling terminal quit errors

```vibe
// run: provide Terminal.Terminal = Terminal.make({ columns: () => 80, rows: () => 24, readInput: () => Queue.unbounded<Terminal.UserInput>(), readLine: () => throw new Terminal.QuitError(), display: (text) => Console.log(text.trimEnd()) }) { askNameOrQuit(); Console.log("quit handled") }
function askNameOrQuit() {
  try {
    askName()
  } catch (error) {
    if (Terminal.isQuitError(error)) return
    throw error
  }
}
// Output: Your name:
// Output: quit handled
```

`QuitError` signals that terminal input ended. Handle it when end-of-input counts as normal completion, and rethrow unrelated failures. In Effect 4.0.6, the Node adapter's `readLine` raises it on end-of-input, including Ctrl+D at a terminal. `readLine` leaves the terminal out of raw mode, so Ctrl+C interrupts the program. The quit keys configured by `NodeTerminal.make` govern `readInput`.

A host SIGINT or SIGTERM is a separate termination path: the runner interrupts the program, bypassing ordinary `catch`, and runs cleanup. While the Node line reader holds the terminal in raw mode, Ctrl+C raises no host signal.

## Building an interactive guessing program

```vibe
// guessing.vibe
// run: const inputs = ["no", "2", "19", "11"]; provide Random = { nextIntUnsafe: () => 0, nextDoubleUnsafe: () => 0.5 }, Terminal.Terminal = Terminal.make({ columns: () => 80, rows: () => 24, readInput: () => Queue.unbounded<Terminal.UserInput>(), readLine: () => inputs.shift() ?? throw new Terminal.QuitError(), display: (text) => Console.log(text.trimEnd()) }) { main() }
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
    if (guess.isNone()) {
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
// void!Terminal.QuitError | PlatformError.PlatformError!Terminal.Terminal

export function main() {
  try {
    game()
  } catch (error) {
    if (Terminal.isQuitError(error)) return
    throw error
  }
}


// Output: Guess an integer from 1 to 20. You have five guesses.
// Output: Guess 1:
// Output: Enter a whole number from 1 to 20.
// Output: Guess 1:
// Output: Try higher.
// Output: Guess 2:
// Output: Try lower.
// Output: Guess 3:
// Output: Correct!
```

The random service picks the secret once per game, both endpoints included. Invalid input repeats the prompt without using a guess; valid input increments the attempt count. The loop stops on success, after five valid guesses, or on a quit request; the runner still reports unexpected output failures.

## Coming from Effect

Run a held Effect such as `terminal.readLine` with `terminal.readLine()`. Ordinary functions and loops replace the generator and recursive Effect construction TypeScript uses for this interaction.

Keep the interactive entry in its own module, so tests can import `main` and supply terminal input:

```vibe
// guessing-main.vibe
import { NodeRuntime, NodeTerminal } from "@effect/platform-node"
import { main } from "./guessing.vibe"

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeTerminal.layer))
// Output: Guess an integer from 1 to 20. You have five guesses.
// Output: Guess 1: 
```
