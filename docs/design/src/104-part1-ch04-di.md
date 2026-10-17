### 4. Dependency injection: the R channel

Dependency injection is Effect's: services are `Context` keys, implementations are `Layer`s, and `Effect.provide` supplies them. Effeck removes the ceremony and adds compile-time checks of the dependency graph.

#### Declaring a service

```ts effect
class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound>
  rename(id: string, name: string): Effect.Effect<void, UserNotFound>
}>()("app/Users") {}
```

```ts es
service Users {
  find(id: string): User!UserNotFound
  rename(id: string, name: string): void!UserNotFound
}
```

`service Users { ... }` declares a service, and it does the job of the class on the left. It is short for `abstract class Users { ... }` in which every member without a body is abstract, and every value member is also readonly; static members, such as a layer, stay as written (D128). An abstract class whose instance members are all abstract is a service whichever way it is written. The rules, each checked by the compiler:

- **What makes a class a service.** In an `.effeck` file, a class is a service when it is `abstract`, has no `extends` clause and no type parameters, and every instance member is abstract: methods with full contracts, and `abstract readonly` values. Static members are allowed and stay on the class. An abstract class with a concrete instance member is an ordinary class.
- **Identity.** Effect identifies a service by its key string, and two services with the same string silently share a slot. The compiler derives the key from the package, module path, and class name (`@acme/app/src/users#Users`) and rejects a duplicate anywhere in the program (EK3001).
- **Each service is its own type.** TypeScript compares classes by shape, so two abstract classes with the same members would be one type: providing one would type-check as providing the other, and the program would die at run time with `Service not found`. The compiler gives every service class its own identity, as Effect's `Context.Service` classes have.
- **Contracts are written out.** A service has no body to infer from, so each method states its full `Effect<A, E, R>` (EK2101, Chapter 14).
- **A service is a key, not a base class.** `extends Users` (EK3002), `new Users()` (EK3003), `instanceof Users` (EK3004), and a generic service class (EK3005) are errors. Implement a service with an object literal or with a class that `implements Users`.

```text
error EK3002: a class cannot extend a service.
  users-memory.effeck:1:28   class UsersInMemory extends Users {
  Implement it instead:  class UsersInMemory implements Users { … }
```

What the class compiles to:

```ts gen
export interface Users {
  find(id: string): Effect.Effect<User, UserNotFound>
  rename(id: string, name: string): Effect.Effect<void, UserNotFound>
  readonly "~effeck/service"?: "@acme/app/src/users#Users"
}
export class Users extends serviceKey<Users>("@acme/app/src/users#Users") {}
```

TypeScript sees `Users` two ways. As a type, it is the interface, so `implements Users` and a parameter typed `db: Users` mean the methods. As a value, it is a `Context.Key`, so `Layer.succeed(Users, …)`, `Effect.provideService`, and `yield* Users` work as they do with a `Context.Service` class. The optional brand keeps two services with the same methods distinct in R. Effect's own class form cannot be the output: it makes the class's instance type the service identifier, so `implements Users` would fail with TS2720 (`prototype/esc/scratch/probe2.ts`).

The syntax takes nothing away from TypeScript. Calling an abstract member through its class (`Users.find(id)`) is error TS2339 today, and calling a class without `new` (`Users()`) is TS2348, so Effeck gives both a meaning. Fully abstract classes are also rare in ordinary code: Smithers has none in 4,461 files, and opencode has five in 3,235, whose only fully abstract one extends another class.

A `Context.Service` written in TypeScript, as a class or as a value like Effect's own `FileSystem`, is a service too, and Effeck uses it the same way.

#### Using a service

Call its methods. Using a service adds it to R.

```ts effect
const users = yield* Users
const user = yield* users.find(id)
```

```ts es
const user = Users.find(id)
```

Calling the service itself with no arguments, `Users()`, gives you its implementation object, the equivalent of `yield* Users`. You need it inside providers, and to read a service's value members:

```ts es
service Settings {
  baseUrl: string
  scale(): number!!
}

function apiUrl(path: string) {
  return `${Settings().baseUrl}/${path}`
}
// string!!Settings
```

Calling any member through the service looks the service up, so the service joins R whatever the member returns. `Path.join(a, b)`, from Effect's `Path` service, returns a string and still adds `Path`. A member that is itself an `Effect` value runs when called: `Terminal.readLine()`. A member the service declares wins over the key's own members, such as `pipe`, `use`, and `key`, so `Rpc.call(…)` reaches the service even though the key, a class, has a `call` of its own. A static member with the same name as a service member is EK3006.

#### Writing a provider

A provider is a `Layer`. Build one with Effect's own `Layer.effect`, passing a function that runs when the layer is built and returns the implementation. `layer UsersLive: Users { ... }` writes the same provider with less: the statements before the members run once, when the layer is built, the members form the implementation, and a value member is `readonly name = value` (D129). It is short for `Layer.effect(Users, () => { ...; return { ... } })`, or `Layer.succeed(Users, { ... })` when there are no statements. A statement after the members is EK2104.

```ts effect
const UsersLive = Layer.effect(Users, Effect.gen(function* () {
  const db = yield* Database
  return Users.of({
    find: Effect.fn("Users.find")(function* (id) {
      const rows = yield* db.query("select * from users where id = $1", [id]).pipe(Effect.orDie)
      if (rows.length === 0) return yield* new UserNotFound({ id })
      return toUser(rows[0])
    }),
    rename: Effect.fn("Users.rename")(function* (id, name) {
      yield* db.execute("update users set name = $1 where id = $2", [name, id]).pipe(Effect.orDie)
    })
  })
}))
// Layer<Users, never, Database>
```

```ts es
layer UsersLive: Users {
  const db = Database()              // runs once, when the layer is built
  find(id) {
    const rows = Effect.orDie(db.query("select * from users where id = $1", [id]))
    if (rows.length === 0) throw new UserNotFound({ id })
    return toUser(rows[0])
  }
  rename(id, name) {
    Effect.orDie(db.execute("update users set name = $1 where id = $2", [name, id]))
  }
}
// Layer<Users, never, Database>
```

`Database()` runs when the layer is built and captures the implementation in `db`. `Database` is therefore a requirement of *building* `UsersLive`, not of calling `Users.find`. Code that calls `Users.find` needs only `Users`.

The compiler checks each method against the service's contract. Without the `Effect.orDie`, the database's `DbError` would escape:

```text
error EK2001: DbError can escape UsersLive.find, but Users.find's contract allows only UserNotFound.
  users-live.effeck:5:20   const rows = db.query("select * from users where id = $1", [id])
                                    ^^^^^^^^ can fail with DbError
  Handle it, add DbError to Users.find's contract, or make it a defect with Effect.orDie.
```

Effect 4 code often keeps the default Layer on the service class as `static readonly layer`. A service class can do the same, because static members stay on the class:

```ts es
service Users {
  find(id: string): User!UserNotFound
  rename(id: string, name: string): void!UserNotFound

  static readonly layer = Layer.effect(Users, () => {
    const db = Database()
    return {
      find(id) { … },
      rename(id, name) { … }
    }
  })
}
```

An implementation can also be a class that `implements` the service. Its methods are Effeck methods, checked against the service's contracts:

```ts es
class UsersInMemory implements Users {
  readonly #rows = new Map<string, User>()

  find(id: string) {
    const user = this.#rows.get(id)
    if (user === undefined) throw new UserNotFound({ id })
    return user
  }

  rename(id: string, name: string) {
    this.#rows.set(id, { ...this.find(id), name })
  }
}

const UsersInMemoryLive = Layer.sync(Users, () => new UsersInMemory())
// Layer<Users, never, never>
```

#### Constructors and defaults

Effect 4's `make` option becomes a static method, and a service that should work without any provider declares a default:

```ts es
service Sessions {
  get(token: string): Session | undefined!!
  set(token: string, session: Session): void!!

  static make() {
    const entries = new Map<string, Session>()
    return {
      get: (token: string) => entries.get(token),
      set: (token: string, session: Session) => { entries.set(token, session) }
    }
  }

  static readonly layer = Layer.effect(Sessions, Sessions.make())
}

service FeatureFlags {
  newCheckout: boolean
  static readonly default: FeatureFlags = { newCheckout: false }
}

function checkoutPage(cartId: string) {
  return FeatureFlags().newCheckout ? newCheckout(cartId) : classicCheckout(cartId)
}
// FeatureFlags is not in R: it always has a value
```

A service with `static readonly default` compiles to Effect's `Context.Reference`. Code that uses it never needs a provider, and `Effect.provideService(() => …, FeatureFlags, { newCheckout: true })` overrides it for a region. Effect's own `Context.Reference("app/Flag", { defaultValue: () => false })` works unchanged too, because a call that returns a service key is data (Chapter 13).

#### Providers that hold resources

A provider can acquire a resource that lives as long as the layer, such as a connection pool. Acquire it without `using`:

```ts effect
const DatabaseLive = Layer.effect(Database, Effect.gen(function* () {
  const config = yield* DatabaseConfig
  const pool = yield* Postgres.connect(config.url)
  return Database.of({
    query: (sql, params) => pool.query(sql, params),
    execute: (sql, params) => pool.execute(sql, params)
  })
}))
// Layer<Database, ConnectError, DatabaseConfig>
```

```ts es
layer DatabaseLive: Database {
  const pool = Postgres.connect(DatabaseConfig().url)    // no `using`: the pool lives as long as the layer
  query(sql, params) { return pool.query(sql, params) }
  execute(sql, params) { return pool.execute(sql, params) }
}
// Layer<Database, ConnectError, DatabaseConfig>
```

`Postgres.connect` returns `Pool!ConnectError!Scope`. Without `using`, the pool belongs to whoever supplies the `Scope` (Chapter 6), and `Layer.effect` supplies the layer's own Scope, as it does in Effect today. The pool therefore closes when the program that provided `DatabaseLive` finishes. Writing `using pool = …` would close it as soon as the provider function returned, before any query ran.

#### Requirements that would leak

Sometimes a method looks up another service each time it is called, instead of capturing it when the layer is built. That lookup happens in the *caller's* context, so its requirement belongs to the caller, and the service contract has to say so. The compiler catches the mismatch:

```ts es
layer UsersAudited: Users {
  const db = Database()
  find(id) {
    Audit.record(`users.find ${id}`)    // looked up on every call
    return findIn(db, id)
  }
  rename(id, name) { renameIn(db, id, name) }
}
```

```text
error EK3101: UsersAudited.find looks up Audit each time it is called, but
  Users.find(id: string): Effect<User, UserNotFound, never>
  allows no call-time requirements.
  users-audited.effeck:5:7   Audit.record(`users.find ${id}`)
  Capture it once:      const audit = Audit()   …   audit.record(…)
  or declare it:        abstract find(id: string): Effect<User, UserNotFound, Audit>
```

Either fix is honest. Capturing makes `Audit` a requirement of building the layer. Declaring it makes every caller of `Users.find` provide `Audit`.

#### Composing an application

```ts effect
const AppLive = Layer.mergeAll(UsersLive, MailerLive).pipe(
  Layer.provideMerge(DatabaseLive),
  Layer.provideMerge(DatabaseConfigLive)
)
```

```ts es
const AppLive = wire(UsersLive, MailerLive, DatabaseLive, DatabaseConfigLive)
// Layer<Users | Mailer | Database | DatabaseConfig, ConfigError | ConnectError, never>
```

`wire` reads each provider's inputs and outputs from its `Layer<Out, E, In>` type, orders them, and generates the composition on the left. Its value is the errors. Today Effect reports a missing provider only as a leftover requirement where you run the program, silently picks one of two providers for the same service, and hangs on some cycles. `wire` reports all three at compile time:

```text
error EK3201: no provider for Smtp.
  welcome → Mailer.send → MailerLive needs Smtp
error EK3202: two providers for Users: UsersLive (src/users.effeck:12) and UsersTest (test/fakes.effeck:3). Pass one.
error EK3203: providers form a cycle: SessionsLive → Database → DatabaseLive → Sessions → SessionsLive
```

The contract is bounded by what Layer types say:

- Inputs that no argument provides stay in the result's `In`, as with `Layer.provide`, so you can wire part of a graph. EK3201 is reported where the program's requirements must be empty: at a runner, or at a contract whose R excludes them.
- An argument whose type does not name its outputs, such as a value typed `Layer<any>`, is EK3204: compose it with `Layer.provide`. A Layer chosen at run time with `Layer.unwrap` has a proper type and wires normally.
- `Layer.provide`, `Layer.merge`, and the rest keep working; `wire` is optional.

#### Running with providers

```ts es
function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  log(`welcomed ${name}`)
}
// void!UserNotFound | MailError | ConfigError | ConnectError!
```

`main`'s contract shows what providing costs: `ConfigError` and `ConnectError` come from building the layers. The layers are built when `Effect.provide` runs, shared by everything inside it, and released in reverse order when it finishes. If building a layer fails, the program fails with that error, and anything already acquired is released.

`with AppLive { ... }` provides a layer to a block (D130). It is short for `Effect.provide(() => { ... }, AppLive)`, except that its statements are code of the enclosing function: a `return` returns from the function, and `break` and `continue` leave its loops. Its errors are the block's and the layer's, and its requirements are the block's that the layer does not provide plus the layer's own. Like `Effect.provide`, it accepts a layer, an array of layers, or a `Context`. A `with` block at a module's top level would run a program there, so it is EK1001.

```ts es
function main() {
  with AppLive {
    log(`welcomed ${welcome("42")}`)
  }
}
// void!UserNotFound | MailError | ConfigError | ConnectError!
```

Each `Effect.provide` builds its layers once, so two sibling `provide` calls build them twice:

```ts es
function counts() {
  const users = Effect.provide(() => countUsers(), AppLive)      // opens a database pool
  const orders = Effect.provide(() => countOrders(), AppLive)    // opens a second pool
  return users + orders
}
```

Provide the application layer once, at `main`. A program with several entry points, such as a web server whose framework calls each handler, builds one `ManagedRuntime` from the layer and runs every entry point through it (Chapter 12).

#### Testing with fake services

```ts effect
const UsersTest = Layer.succeed(Users, Users.of({
  find: (id) => Effect.succeed({ id, name: "Ada", email: "ada@example.com" }),
  rename: () => Effect.void
}))

it.effect("welcome returns the user's name", () =>
  Effect.gen(function* () {
    const name = yield* welcome("42").pipe(Effect.provide(Layer.mergeAll(UsersTest, MailerTest)))
    assert.strictEqual(name, "Ada")
  }))
```

```ts es
layer UsersTest: Users {
  find(id) { return ({ id, name: "Ada", email: "ada@example.com" }) }
  rename() {}
}

it.effect("welcome returns the user's name", () => {
  const name = Effect.provide(() => welcome("42"), wire(UsersTest, MailerTest))
  assert.strictEqual(name, "Ada")
})
```

To replace one service for part of a program, provide it around that part. The override applies only inside:

```ts es
function welcomeQuietly(id: string) {
  return Effect.provideService(welcome(id), Mailer, { send: () => {} })
}
```

#### What a function sees, and when

- A service call uses the providers in effect where the call runs. Code inside `welcomeQuietly` sees the quiet `Mailer`; code outside does not.
- After a pause (`Effect.sleep`, a network call), a function sees the same providers it saw before.
- A forked fiber starts with the providers of the code that forked it (Chapter 7).
- An implementation you captured, `const db = Database()`, stays that implementation even if `Database` is overridden later.
