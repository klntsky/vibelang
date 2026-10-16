// expect: TS2304
// .ts files in the same program do not see the prelude: pipe is not in scope here.
export const four = pipe(2, (n: number) => n * 2)
