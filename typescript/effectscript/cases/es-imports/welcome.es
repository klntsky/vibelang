// An .es module: welcome is an Effect function, shout a pure helper.
export class EmptyName extends TaggedError("EmptyName")<{}> {}

export const events: string[] = []

export function shout(text: string) {
  return text.toUpperCase()
}

export function welcome(name: string) {
  if (name.length === 0) Effect.fail(new EmptyName())
  Effect.sync(() => events.push(`welcome ${name}`))
  return `Welcome, ${shout(name)}!`
}
