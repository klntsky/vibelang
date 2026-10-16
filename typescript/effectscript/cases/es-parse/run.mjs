// JavaScript prints the module's value: console is a platform API, which
// .es code does not call (D6).
import { sum } from "./out/main.js"

console.log(`sum = (${sum.x}, ${sum.y})`)
