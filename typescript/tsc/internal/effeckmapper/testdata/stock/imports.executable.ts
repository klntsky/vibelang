import { $direct } from "effeck-runtime"; import { sync as $sync } from "effect/Effect"; 
import { name } from "./names.js"; export { name } from "./names.js"; export function value(...$args) { return $sync(() => value$direct(...$args)) } value[$direct] = value$direct;
function value$direct() { return name }