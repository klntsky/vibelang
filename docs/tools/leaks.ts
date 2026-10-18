// What a published docs file must not contain: local paths, the build host,
// LAN addresses, the private repository, or any credential value present in
// the environment. prepare-site.ts checks the site with it, and hovers.ts
// checks the hover cache.
const forbidden: [string, RegExp][] = [
  ["a local path", /\/Users\/[A-Za-z]|\/home\/[a-z]/],
  ["a user name", /williamcory/i],
  ["the build host", /64\.34\.80\.163|160\.202\.128\.229|msb-lat|smithers-baremetal/],
  ["a LAN address", /\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b/],
  ["the private repository", /roninjin10/],
  ["a private key path", /\.ssh\//],
]

const credentials = Object.entries(process.env)
  .filter(([name, value]) => /TOKEN|KEY|SECRET|PASSWORD/.test(name) && value && value.length >= 16)
  .map(([name, value]) => [name, value!] as const)

// One message for each kind of leak in text. A message names a credential
// by its variable and never prints its value.
export function leaks(text: string): string[] {
  const problems: string[] = []
  for (const [what, pattern] of forbidden) {
    const match = pattern.exec(text)
    if (match) problems.push(`${what}: ${JSON.stringify(match[0])}`)
  }
  for (const [name, value] of credentials) if (text.includes(value)) problems.push(`the value of ${name}`)
  return problems
}
