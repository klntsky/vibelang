// Q10: can an Effect 3 value run inside an Effect 4 fiber, or vice versa?
// Usage: <runtime> q10-v3-interop.ts <path-to-effect3-dist-esm-Effect.js>
import * as E4 from "effect/Effect"
import * as Exit4 from "effect/Exit"

const E3: any = await import(process.argv[2])
const v3 = E3.succeed(3)
const v4 = E4.succeed(4)

const symbols = (o: object) => [...Object.getOwnPropertySymbols(o), ...Object.getOwnPropertySymbols(Object.getPrototypeOf(o) ?? {})].map(String)
console.log("v3 effect symbol keys:", JSON.stringify(symbols(v3)), "| _op:", v3._op)
console.log("v4 has v3 TypeId Symbol.for('effect/Effect'):", Symbol.for("effect/Effect") in (v4 as any), "| v3 has '~effect/Effect':", "~effect/Effect" in v3)
console.log("E4.isEffect(v3):", E4.isEffect(v3), "| E3.isEffect(v4):", E3.isEffect(v4))

const show4 = (exit: Exit4.Exit<unknown, unknown>) =>
  exit._tag === "Success" ? `Success(${String(exit.value)})` : `Failure(${exit.cause.reasons.map((r: any) => `${r._tag}: ${String(r.defect ?? r.error).split("\n")[0]}`).join("; ")})`

console.log("v4 runPromiseExit(v3 effect):", show4(await E4.runPromiseExit(v3 as any)))
console.log("v4 Effect.gen yield* v3 effect:", show4(await E4.runPromiseExit(E4.gen(function*() {
  return yield* (v3 as any)
}))))
const v3exit: any = await E3.runPromiseExit(v4)
console.log("v3 runPromiseExit(v4 effect):", v3exit._tag, String(v3exit.cause?.defect ?? v3exit.value ?? "").split("\n")[0])
const v3gen: any = await E3.runPromiseExit(E3.gen(function*() {
  return yield* (v4 as any)
}))
console.log("v3 Effect.gen yield* v4 effect:", v3gen._tag, String(v3gen.cause?.defect ?? v3gen.value ?? "").split("\n")[0])

// a minimal adapter: run the foreign effect with its own runtime inside a callback
const adaptV3 = <A>(eff: any) => E4.callback<A>((resume) => {
  const cancel = E3.runCallback(eff, { onExit: (exit: any) => resume(exit._tag === "Success" ? E4.succeed(exit.value) : E4.die(exit.cause)) })
  return E4.sync(() => cancel())
})
console.log("v4 running v3 via callback adapter:", show4(await E4.runPromiseExit(adaptV3<number>(v3))))
