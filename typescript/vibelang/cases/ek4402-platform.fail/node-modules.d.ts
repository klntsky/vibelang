// Stand-ins for @types/node's modules, so this case needs no install. Only
// the import specifiers decide what is a platform API.
declare module "node:fs/promises" {
  export function readFile(path: string, encoding: string): Promise<string>
}
declare module "fs" {
  const fs: { existsSync(path: string): boolean }
  export default fs
}
declare module "node:os" {
  export function cpus(): ReadonlyArray<unknown>
  export const EOL: string
  export const constants: { readonly signals: Record<string, number> }
}
declare module "node:http" {
  export interface Server {
    readonly listening: boolean
  }
  export function createServer(): Server
}
