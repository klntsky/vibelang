// expect: ES1003
// await using is not part of EffectScript: using already waits.
declare function open(path: string): AsyncDisposable

export function read() {
  await using file = open("/tmp/a")
  return 1
}
