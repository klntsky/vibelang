function f() { {
    $uninterruptibleBlock();
    {
        $restoreBlock();
        read();
    }
} ; return (() => { throw new Error("gone"); })(); }
