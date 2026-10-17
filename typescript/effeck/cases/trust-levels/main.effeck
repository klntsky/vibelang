// Trust levels (D5, D22, D23): what a call to code that is not Effect means.
import { Cause, Effect, Exit, Option } from "effect"
import { legacyParse, parseJson, parsePort, Ports } from "./parsers.ts"

// A built-in's shipped contract: JSON.parse fails with SyntaxError, which
// joins E, so a catch receives it.
function config(text: string) {
  try {
    return JSON.parse(text).port as number
  } catch (e) {
    return e instanceof SyntaxError ? -1 : -2
  }
}

// A trusted function: PortError joins E.
function port(text: string) {
  try {
    return parsePort(text)
  } catch (e) {
    return e._tag
  }
}

// An exception the contract does not list is a defect that no catch sees.
function crash() {
  return Effect.exit(() => parsePort("boom"))
}

// The arguments run their Effects before the trusted call.
function loaded() {
  return parsePort(Effect.succeed("8443"))
}

// A function that catches its own failures compiles to plain JavaScript
// (D54), so Array.prototype.map can call it.
function parseOr(text: string, fallback: number) {
  try {
    return JSON.parse(text) as number
  } catch (e) {
    return fallback
  }
}

// Effect's own exports are trusted, and so are the functions they return.
function first(values: ReadonlyArray<number>) {
  return Option.getOrElse(Option.fromNullishOr(values[0]), () => 0)
}

// A boundary: inside Effect.try, a plain TypeScript call is allowed.
function legacy(text: string) {
  try {
    return Effect.try({ try: () => legacyParse(text), catch: () => "bad port" })
  } catch (e) {
    return e
  }
}

// A trusted re-export and a trusted method.
function reexported(text: string) {
  try {
    return parseJson(text)
  } catch (e) {
    return e.name
  }
}

function offset(by: number) {
  try {
    return new Ports(65530).offset(by)
  } catch (e) {
    return e._tag
  }
}

// A parameter's callee is its caller's choice.
function apply(f: (x: number) => number, x: number) {
  return f(x)
}

// expect-result: {"config":[8080,-1],"port":[80,"PortError"],"crash":true,"loaded":8443,"parsed":[1,0],"first":3,"legacy":[81,"bad port"],"reexported":[{"a":1},"SyntaxError"],"offset":[65535,"PortError"],"apply":4}
export function main() {
  const exit = crash()
  return {
    config: [config(`{"port":8080}`), config("{")],
    port: [port("80"), port("x")],
    crash: Exit.isFailure(exit) && Cause.hasDies(exit.cause),
    loaded: loaded(),
    parsed: ["1", "{"].map((text) => parseOr(text, 0)),
    first: first([3, 4]),
    legacy: [legacy("81"), legacy("x")],
    reexported: [reexported(`{"a":1}`), reexported("{")],
    offset: [offset(5), offset(6)],
    apply: apply((x) => x * 2, 2)
  }
}
