// No rewriteRelativeImportExtensions, as in a build that emits only
// declarations and bundles the JavaScript: the declaration still names
// User.vibe by its .js output.
import { find } from "./User.vibe"

export { find, NotFound } from "./User.vibe"

export const lookup = (id: string) => find(id)
export const later = () => import("./User.vibe")
