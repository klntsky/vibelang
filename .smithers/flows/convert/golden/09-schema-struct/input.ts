import { Schema } from "effect"

export const Account = Schema.Struct({
  id: Schema.String,
  nickname: Schema.optional(Schema.String),
  role: Schema.Literals(["admin", "member"])
})
export interface Account extends Schema.Schema.Type<typeof Account> {}
