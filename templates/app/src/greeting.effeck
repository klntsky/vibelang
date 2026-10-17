export class EmptyName extends TaggedError("EmptyName")<{}> {}

export function greeting(name: string) {
  if (name.length === 0) throw new EmptyName()
  return `Hello, ${name}!`
}
