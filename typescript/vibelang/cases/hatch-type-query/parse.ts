export const parseSync = (text: string): number => {
  const value = Number(text)
  if (Number.isNaN(value)) throw new Error("not a number")
  return value
}
