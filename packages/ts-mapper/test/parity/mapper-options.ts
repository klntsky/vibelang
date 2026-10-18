// mapperEntry translates the fork's options before the stock adapter
// removes compilerOptions.effeck. The same tracing setting drives Emit.
export function mapperEntry(config: any): { package: string; extensions: string[]; options?: { tracing: string } } {
  const tracing = config.compilerOptions?.effeck?.tracing
    ?? config.effeck?.tracing
    ?? config.contentMappers?.find((m: any) => m.package === "effeck-ts-mapper")?.options?.tracing
  return { package: "effeck-ts-mapper", extensions: [".effeck"], ...(tracing === undefined ? {} : { options: { tracing } }) }
}
