// .es code imports another .es module by its source name, and a .ts module
// by its source name; the emitted JavaScript imports ./welcome.js and
// ./greeter.js (D31).
import { events, shout, welcome } from "./welcome.es"
import { greetTwice } from "./greeter.ts"

// expect-result: {"one":"Welcome, ADA!","two":"Welcome, GRACE! Welcome, GRACE!","loud":"HI","events":["welcome Ada","welcome Grace","welcome Grace"]}
export function main() {
  const one = welcome("Ada")
  const two = greetTwice("Grace")
  const loud = shout("hi")
  return { one, two, loud, events }
}
