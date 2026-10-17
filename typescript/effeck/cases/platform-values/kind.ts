/**
 * Stands in for an Effect API that receives a platform function or value.
 *
 * @effeck trusted
 */
export function kind(value: unknown): string {
  return typeof value
}
