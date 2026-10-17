/**
 * Stands in for an Effect API, such as NodeHttpServer.layer, that receives a
 * platform function or value and runs it itself.
 *
 * @effeck trusted
 */
export function setup(create: () => unknown, separator: string): string {
  return `${typeof create}${separator.length}`
}
