// expect: ES1003 ES1004
// await using at a module's top level is both ES1003 and ES1004.
declare function open(path: string): AsyncDisposable

await using log = open("/var/log/app.log")
