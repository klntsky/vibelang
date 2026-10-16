// Chapter 5. Programs as values: retry, timeout, and pipe.
// NOTE: durations are shortened from the proposal's (100 millis, 1 second) so the run is fast.
import { Clock, Schedule } from "effect"

class FetchError extends TaggedError("FetchError")<{ readonly attempt: number }> {}
class SyncError extends TaggedError("SyncError")<{ readonly store: string }> {}

interface User {
  readonly id: string
  readonly name: string
}

interface Store {
  readonly name: string
}

interface SyncReport {
  readonly store: string
  readonly items: number
}

abstract class Inventory {
  abstract sync(store: Store): Effect<SyncReport, SyncError, never>
}

abstract class Http {
  abstract listen(port: number): Effect<string, never, never>
}

let fetchCalls = 0
let syncRuns = 0
let configLoads = 0

// Fails on the first two calls, then succeeds.
function fetchUser(id: string) {
  fetchCalls++
  if (fetchCalls < 3) throw new FetchError({ attempt: fetchCalls })
  const user: User = { id, name: "Ada" }
  return user
}

function fromCache(id: string) {
  Effect.sleep("1 millis")
  return "cache"
}

function fromNetwork(id: string) {
  Effect.sleep("50 millis")
  return "network"
}

function slowQuery(id: string) {
  Effect.sleep("50 millis")
  return "slow"
}

function syncInventory(store: Store) {
  syncRuns++
  return Inventory.sync(store)
}

// The one rule

function oneRule(store: Store) {
  syncInventory(store)                    // runs now

  const job = () => syncInventory(store)  // a program: nothing has run
  const before = syncRuns
  job()                                   // runs it
  job()                                   // runs it again, from scratch
  return { before, after: syncRuns }
}

function combinators(id: string) {
  const user = Effect.retry(() => fetchUser(id), Schedule.exponential("1 millis"))
  const fresh = Effect.timeout(() => fetchUser(id), "1 second")
  const fastest = Effect.race(() => fromCache(id), () => fromNetwork(id))
  let timedOut = "no"
  try {
    Effect.timeout(() => slowQuery(id), "5 millis")
  } catch (e) {
    timedOut = e._tag
  }
  return { user: user.name, fresh: fresh.name, fastest, timedOut }
}

// Pipe

function piped(id: string) {
  const user = pipe(
    () => fetchUser(id),
    Effect.retry({ times: 3 }),
    Effect.timeout("2 seconds"),
    Effect.withSpan("load-user")
  )
  // user: User;  E gains TimeoutError from Effect.timeout
  return user.name
}

// Storing, returning, and composing programs

function makeSync(store: Store) {
  return () => syncInventory(store)            // returns a program
}
// Effect<() => Effect<SyncReport, SyncError, Inventory>, never, never>

// NOTE: stores.map(makeSync) passes makeSync to Array.prototype.map (D9: its plain variant), so
// syncAll is an array of program functions; Effect.all must then convert each element to an Effect.
function syncEverything(stores: ReadonlyArray<Store>) {
  const syncAll = stores.map(makeSync)                    // an array of programs; nothing ran
  return Effect.all(syncAll, { concurrency: 4 })          // now they run
}

function loadConfig() {
  configLoads++
  return { port: 8080 }
}

function startServer() {
  const cachedConfig = Effect.cached(() => loadConfig())  // Effect.cached returns a program
  const config = cachedConfig()                           // loads once
  const again = cachedConfig()                            // returns the cached value
  return Http.listen(config.port)
}

// An Effect value from TypeScript is a program: calling it runs it.
function clockKind() {
  const millis = Clock.currentTimeMillis()
  return typeof millis
}

const stores: ReadonlyArray<Store> = [{ name: "north" }, { name: "south" }, { name: "east" }]

const InventoryTest = Layer.succeed(Inventory, {
  sync: (store) => ({ store: store.name, items: store.name.length })
})

const HttpTest = Layer.succeed(Http, {
  listen: (port) => `listening on ${port}`
})

function scenario() {
  const rule = oneRule(stores[0])
  const combined = combinators("42")
  const pipedName = piped("42")
  const reports = syncEverything(stores)
  const server = startServer()
  const clock = clockKind()
  return {
    rule,
    combined,
    piped: pipedName,
    reports: reports.map((report) => report.store),
    syncRuns,
    server,
    configLoads,
    clock,
    fetchCalls
  }
}

// expect-result: {"rule":{"before":1,"after":3},"combined":{"user":"Ada","fresh":"Ada","fastest":"cache","timedOut":"TimeoutError"},"piped":"Ada","reports":["north","south","east"],"syncRuns":6,"server":"listening on 8080","configLoads":1,"clock":"number","fetchCalls":5}
export function main() {
  return Effect.provide(() => scenario(), Layer.mergeAll(InventoryTest, HttpTest))
}
