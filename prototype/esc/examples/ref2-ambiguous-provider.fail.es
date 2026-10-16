// expect: ES3202
// Reference program 2: two providers for Users. wire must reject the ambiguity instead of picking one.

abstract class Users {
  abstract find(id: string): Effect<string, never, never>
}

const UsersLive = Layer.succeed(Users, {
  find: (id) => `live-${id}`
})

const UsersTest = Layer.succeed(Users, {
  find: (id) => `test-${id}`
})

const AppLive = wire(UsersLive, UsersTest)
