// Without rewriteRelativeImportExtensions, a .es file still imports ./names.es
// and ./format.ts by their source names, with no TS5097, and the emitted
// JavaScript imports ./names.js and ./format.js (D31).
import { fullName } from "./names.es"
import { formatName } from "./format.ts"
export { fullName } from "./names.es"

// expect-result: ["Lovelace, Ada","Hopper, Grace"]
export function main() {
  return [fullName("Ada", "Lovelace"), formatName("Grace", "Hopper")]
}
