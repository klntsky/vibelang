// expect: ES4301
// Chapter 10 "Callbacks into plain JavaScript": this callback can pause (Users.find), but
// Array.prototype.map calls it synchronously.

class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}

interface User {
  readonly id: string
  readonly name: string
}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
}

function allUsers(ids: ReadonlyArray<string>) {
  const users = ids.map((id) => Users.find(id))      // rejected
  return users
}
