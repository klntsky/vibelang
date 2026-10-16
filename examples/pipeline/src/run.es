import { NodeRuntime } from "@effect/platform-node"
import { main } from "./main.es"

NodeRuntime.runMain(() => main())
