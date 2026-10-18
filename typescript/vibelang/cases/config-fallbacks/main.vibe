// A Config is data until called (design D24), so a callback that Effect
// expects to return a Config, such as Config.orElse's fallback, returns the
// Config and stays a plain function.
import { Config, ConfigProvider } from "effect"

export const host = Config.orElse(
  Config.NonEmptyString("HOST"),
  () => Config.NonEmptyString("BACKUP_HOST")
)

function withFallback(name: string) {
  return Config.orElse(Config.String(name), () => Config.succeed(`no ${name}`))
}

function read() {
  return { host: host(), missing: withFallback("MISSING")() }
}

// expect-result: {"host":"backup.example.com","missing":"no MISSING"}
export function main() {
  return Effect.provide(() => read(), ConfigProvider.layer(ConfigProvider.fromEnvRecord({ BACKUP_HOST: "backup.example.com" })))
}
