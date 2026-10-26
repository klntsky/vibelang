// Stands in for @types/node, a platform package, whose globals are platform APIs.
export {}
declare global {
  var process: { cwd(): string; env: Record<string, string | undefined> }
}
