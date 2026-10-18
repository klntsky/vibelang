/**
 * Stands in for an Effect API that receives a platform function or value.
 *
 * @vibelang trusted
 */
export function kind(value: unknown): string {
  return typeof value
}
