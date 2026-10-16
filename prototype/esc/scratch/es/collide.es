abstract class Rpc {
  abstract call(method: string): Effect<string, never, never>
  abstract pipe(x: number): Effect<number, never, never>
  static readonly pipe = 1
}
export function go() {
  return Rpc.call("ping") + Rpc.pipe(2)
}
