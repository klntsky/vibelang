interface Greeting {
  readonly name: string
  readonly excited?: boolean
}

function greet({ name, excited = false }: Greeting): string {
  return `hello, ${name}${excited ? "!" : ""}`
}

const greetings: readonly Greeting[] = [{ name: "typescript" }, { name: "effectscript", excited: true }]
for (const g of greetings) console.log(greet(g))
