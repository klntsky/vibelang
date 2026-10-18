"use strict";
function greet({ name, excited = false }) {
    return `hello, ${name}${excited ? "!" : ""}`;
}
const greetings = [{ name: "typescript" }, { name: "vibelang", excited: true }];
for (const g of greetings)
    console.log(greet(g));
