function f(items: number[]) { const task = fork(read()); const value = $join(task); const all = $join([task]); $forConcurrent(items, 2, item => {
    write(item);
}); return { value, all }; }
