/**
 * Stands in for an Effect API that receives a platform function or value.
 *
 * @effectscript trusted
 */
export function kind(value: unknown): string {
  return typeof value
}
