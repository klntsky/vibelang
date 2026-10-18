// The part of vitest 5.0.5's declarations the case calls: assert is chai's,
// and expect returns an assertion object.
export interface Assertion<T> {
  toBe(expected: T): void
}
export declare const expect: <T>(actual: T) => Assertion<T>
export declare const assert: {
  strictEqual<T>(actual: T, expected: T): void
  isTrue(value: boolean): void
}
