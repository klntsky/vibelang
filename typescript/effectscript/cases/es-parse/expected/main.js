export function add(a, b) {
    return { x: a.x + b.x, y: a.y + b.y };
}
export const origin = { x: 0, y: 0 };
const sum = [{ x: 1, y: 2 }, { x: 3, y: 4 }].reduce(add, origin);
console.log(`sum = (${sum.x}, ${sum.y})`);
