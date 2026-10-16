// A stand-in for @types/node's node:http, so a type-only import resolves.
declare module "node:http" {
  export interface Server {
    readonly listening: boolean
  }
}
