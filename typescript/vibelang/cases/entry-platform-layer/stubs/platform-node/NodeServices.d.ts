// @effect/platform-node 4.0.6's NodeServices.d.ts.
import type { Crypto } from "effect/Crypto"
import type { FileSystem } from "effect/FileSystem"
import * as Layer from "effect/Layer"
import type { Path } from "effect/Path"
import type { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"
import type { Stdio } from "effect/Stdio"
import type { Terminal } from "effect/Terminal"

export type NodeServices = ChildProcessSpawner | Crypto | FileSystem | Path | Stdio | Terminal
export declare const layer: Layer.Layer<NodeServices>
