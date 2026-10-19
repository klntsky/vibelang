/** @vibelang trusted */
export function valueOnly(value: number): number
export function valueOnly(value: string): string
export function valueOnly(value: number | string) { return value }
