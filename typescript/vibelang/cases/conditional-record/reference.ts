export const consume = (present: boolean): Readonly<Record<string, string>> => {
  const cache = present ? { URL: "cache" } : {}
  return cache
}
