// expect: ES1003
// @ts-ignore hides TypeScript's diagnostics only; ES1003 is still reported.
declare const pending: Promise<number>

export function total() {
  // @ts-ignore
  const n = await pending
  return n
}
