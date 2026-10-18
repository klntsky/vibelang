// mapperEntry translates the fork's options before the stock adapter
// removes compilerOptions.vibelang. The same tracing setting drives Emit.
export function mapperEntry(config: any): { package: string; extensions: string[]; options?: { tracing: string } } {
  const tracing = config.compilerOptions?.vibelang?.tracing
    ?? config.vibelang?.tracing
    ?? config.contentMappers?.find((m: any) => m.package === "vibelang-ts-mapper")?.options?.tracing
  return { package: "vibelang-ts-mapper", extensions: [".vibe"], ...(tracing === undefined ? {} : { options: { tracing } }) }
}
