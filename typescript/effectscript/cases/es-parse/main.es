// An .es file is parsed and checked as TypeScript. Nothing is lowered yet.
export interface Point {
  readonly x: number
  readonly y: number
}

export function add(a: Point, b: Point): Point {
  return { x: a.x + b.x, y: a.y + b.y }
}

export const origin: Point = { x: 0, y: 0 }

export const sum = [{ x: 1, y: 2 }, { x: 3, y: 4 }].reduce<Point>(add, origin)
