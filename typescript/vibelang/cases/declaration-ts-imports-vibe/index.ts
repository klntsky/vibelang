import { find } from "./User.vibe"

export { find, NotFound } from "./User.vibe"
export { prefix } from "./prefix.ts"
export type { NotFound as Missing } from "./User.vibe"

export const lookup = (id: string) => find(id)
export const later = () => import("./User.vibe")
