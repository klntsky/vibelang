// Stand-ins for @types/node's modules, so this case needs no install.
declare module "node:http" {
  export function createServer(): unknown
}
declare module "node:os" {
  export const EOL: string
}
