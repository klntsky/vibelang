// Stand-ins for @types/node's modules, so this case needs no install. Only
// the import specifiers decide what is a platform API.
declare module "node:path" {
  export function join(...paths: string[]): string
}
declare module "node:child_process" {
  export function execFileSync(command: string, args: ReadonlyArray<string>): string
}
declare module "node:crypto" {
  export function randomBytes(size: number): Uint8Array
}
declare module "node:os" {
  export function tmpdir(): string
  export function hostname(): string
}
declare module "node:fs/promises" {
  export function mkdtemp(prefix: string): Promise<string>
}
