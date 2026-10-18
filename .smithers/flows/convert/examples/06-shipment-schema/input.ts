import { Schema } from "effect"

export const Shipment = Schema.Struct({
  tracking: Schema.String,
  weight: Schema.Number,
  note: Schema.optional(Schema.String),
  stage: Schema.Literals(["packed", "sent", "received"])
})
export interface Shipment extends Schema.Schema.Type<typeof Shipment> {}

export interface Manifest {
  readonly depot: string
  parcels: Array<Shipment>
}
