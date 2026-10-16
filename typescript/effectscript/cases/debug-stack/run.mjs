// Runs the compiled program with source maps on, and prints the first frame
// of each failure's stack that points into main.es, without its directory.
process.setSourceMapsEnabled(true)
const { Cause, Effect, Exit } = await import("effect")
const program = await import("./out/main.js")

const frame = (error) => {
  const line = error.stack.split("\n").find((l) => l.includes("main.es"))
  return line.trim().replace(/\(.*\/(main\.es:\d+:\d+)\)$/, "($1)")
}
const failure = async (effect) => {
  const exit = await Effect.runPromiseExit(effect)
  return Exit.isFailure(exit) ? Cause.squash(exit.cause) : undefined
}

console.log(JSON.stringify(await Effect.runPromise(program.counted())))
console.log(frame(await failure(program.crash())))
console.log(frame(await failure(program.missing())))
console.log(frame(await failure(program.region())))
