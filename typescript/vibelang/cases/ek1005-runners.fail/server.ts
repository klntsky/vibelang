/**
 * A framework's route registration: it calls handler later, outside any fiber.
 *
 * @vibelang trusted
 */
export function onRequest(handler: () => void): void {
  void handler
}
