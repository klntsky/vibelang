// Pure helpers in their own module. Each compiles to a $direct variant, which
// other .es modules import and call inline, and an Effect form, which
// TypeScript modules import.
export function double(n: number) {
  return n * 2
}

export function square(n: number) {
  return n * n
}
