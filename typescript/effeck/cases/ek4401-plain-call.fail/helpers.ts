// TypeScript functions without contracts, with a contract, and with a
// contract whose @throws class the .effeck file cannot import.
export interface User {
  readonly name: string
}

export const formatName = (user: User): string => `<${user.name}>`

export const fetchName = (id: string): Promise<string> => Promise.resolve(id)

/** @effeck trusted */
export const shout = (text: string): string => text.toUpperCase()

class HiddenError extends Error {}

/**
 * @effeck trusted
 * @throws {HiddenError} when the text is empty
 */
export const parseHidden = (text: string): string => {
  if (text === "") throw new HiddenError("empty")
  return text
}
