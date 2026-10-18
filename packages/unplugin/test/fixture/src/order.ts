// shop.vibe imports this module for a type only, so only the compiler reads it.
export interface Order {
  readonly item: string
  readonly quantity: number
}
