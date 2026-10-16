export class NotFound extends TaggedError("NotFound")<{ readonly id: number }> {}

export function lookup(id: number) {
  if (id > 1) Effect.fail(new NotFound({ id }))
  return "ada"
}
