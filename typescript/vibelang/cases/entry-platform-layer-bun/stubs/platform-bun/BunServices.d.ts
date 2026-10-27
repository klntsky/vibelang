// @effect/platform-bun 4.0.6's BunServices.d.ts.
import type { Crypto } from "effect/Crypto"
import type { FileSystem } from "effect/FileSystem"
import * as Layer from "effect/Layer"
import type { Path } from "effect/Path"
import type { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"
import type { Stdio } from "effect/Stdio"
import type { Terminal } from "effect/Terminal"

export type BunServices = ChildProcessSpawner | Crypto | FileSystem | Path | Terminal | Stdio
export declare const layer: Layer.Layer<BunServices>
