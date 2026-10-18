// Nanoseconds per call for the projector's call helpers, in the shapes of the
// text-only study's benchmarks: 200,000 calls of `n * 2` inside one Effect
// function, best of 15 rounds (the study took the best of 3; more rounds
// keep the minimum steady on a shared machine). Run from packages/runtime after a
// build:
//
//   bun run build && node tests/fastcalls-bench.mjs
import { Effect } from "effect"
import { $direct, $done, $fast, $isRun, $settle } from "vibelang-runtime"

const N = 200_000
const expected = N * (N - 1) // the sum of 2i for i < N

const double$direct = (n) => n * 2
const doubleBody = function* (n) { return n * 2 }
// D10's pair as the fork emits it: the plain function and its Effect wrapper.
function double(...args) { return Effect.sync(() => double$direct(...args)) }
double[$direct] = double$direct
const doubleEffectOnly = Effect.fnUntraced(doubleBody) // the slow call's Effect function
const exitBody = function* (n) { return yield* Effect.succeed(n * 2) } // yields an Exit
const viaExit = Effect.fnUntraced(exitBody)
const syncBody = function* (n) { return yield* Effect.sync(() => n * 2) } // needs the fiber
const viaSync = Effect.fnUntraced(syncBody)
// The two fast entries the projector writes: per function, and $fast's bounded one.
function double$fast(n) { const it = doubleBody(n), s = it.next(); return s.done ? $done(s.value) : $settle(it, s) }
function exit$fast(n) { const it = exitBody(n), s = it.next(); return s.done ? $done(s.value) : $settle(it, s) }
function sync$fast(n) { const it = syncBody(n), s = it.next(); return s.done ? $done(s.value) : $settle(it, s) }
const double$bounded = $fast(doubleBody), exit$bounded = $fast(exitBody), sync$bounded = $fast(syncBody)

const loops = {
  "$direct, same file (D10 today)": function* () { let s = 0; for (let i = 0; i < N; i++) s += double$direct(i); return s },
  "f[$direct], another module": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += ((t = (double[$direct] ?? double)(i)), double[$direct] ? t : ($isRun(t) ? yield* t : t)); return s },
  "plain helper + $isRun (L3)": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = double$direct(i), $isRun(t) ? yield* t : t); return s },
  "Effect fn + $isRun": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = doubleEffectOnly(i), $isRun(t) ? yield* t : t); return s },
  "Effect fn, yield* f(x)": function* () { let s = 0; for (let i = 0; i < N; i++) s += yield* doubleEffectOnly(i); return s },
  "fast call, pure body": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = double$fast(i), $isRun(t) ? yield* t : t); return s },
  "$fast entry, pure body": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = double$bounded(i), $isRun(t) ? yield* t : t); return s },
  "fast call, body yields an Exit": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = exit$fast(i), $isRun(t) ? yield* t : t); return s },
  "$fast entry, body yields an Exit": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = exit$bounded(i), $isRun(t) ? yield* t : t); return s },
  "Effect fn, body yields an Exit": function* () { let s = 0; for (let i = 0; i < N; i++) s += yield* viaExit(i); return s },
  "fast call, body suspends": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = sync$fast(i), $isRun(t) ? yield* t : t); return s },
  "$fast entry, body suspends": function* () { let s = 0, t; for (let i = 0; i < N; i++) s += (t = sync$bounded(i), $isRun(t) ? yield* t : t); return s },
  "Effect fn, body suspends": function* () { let s = 0; for (let i = 0; i < N; i++) s += yield* viaSync(i); return s }
}

const best = {}
for (let round = 0; round < 15; round++) {
  for (const [name, loop] of Object.entries(loops)) {
    const t0 = performance.now()
    const result = Effect.runSync(Effect.fnUntraced(loop)())
    const t1 = performance.now()
    if (result !== expected) throw new Error(`${name}: wrong result ${result}`)
    best[name] = Math.min(best[name] ?? Infinity, ((t1 - t0) * 1e6) / N)
  }
}
const engine = typeof Bun === "undefined" ? `Node ${process.version}` : `Bun ${Bun.version}`
console.log(`${engine}, ${N.toLocaleString("en-US")} calls of n * 2, best of 15`)
for (const [name, ns] of Object.entries(best)) console.log(`${name.padEnd(34)} ${ns.toFixed(1).padStart(7)} ns/call`)
const over = (name) => (best[name] - best["$direct, same file (D10 today)"]).toFixed(1).padStart(7)
console.log(`${"run check alone (L3)".padEnd(34)} ${over("plain helper + $isRun (L3)")} ns/call`)
console.log(`${"property check alone".padEnd(34)} ${over("f[$direct], another module")} ns/call`)
