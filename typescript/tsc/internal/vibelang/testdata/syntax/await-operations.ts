function f() { const all = $awaitAll([read(), (() => { return 2; })()]); const settled = $awaitAllSettled({ one: read() }); const race = $awaitRace([read()]); return $awaitAny([read()]); }
