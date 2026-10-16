/// <reference types="node" />
// NodeHttpServer takes Node's createServer, a platform API that .es files do
// not import (D6, ES4402), so the server layer is built in TypeScript.
import { NodeHttpServer } from "@effect/platform-node"
import { createServer } from "node:http"

export const ServerLive = NodeHttpServer.layer(createServer, { port: 3000 })
