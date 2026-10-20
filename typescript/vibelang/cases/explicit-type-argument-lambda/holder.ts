// Stores a function and returns it, shaped like vitest's vi.fn: a type
// parameter whose constraint is a plain function type.
export const hold = <T extends (...args: Array<any>) => any>(f: T): T => f
