export const foreign = (text: string): string => {
  if (!text) throw new Error("foreign")
  return text
}
/** @vibelang trusted @throws {SyntaxError} */
export function tagged(text: string): string {
  if (!text) throw new SyntaxError("tagged")
  return text
}
