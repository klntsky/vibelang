function f() { const value = (() => { const x = read(); return x + 1; })(); return Effect.retry({ times: 2 }, () => { value; }); }
