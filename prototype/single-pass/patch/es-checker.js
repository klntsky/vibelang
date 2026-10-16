  // ===== EffectScript single-pass experiment (prototype/single-pass/patch/es-checker.js) =====
  //
  // Hypothesis: if lambda classification happens inside overload resolution, per candidate
  // signature, one checker pass reaches the lowering decisions the multi-pass prototype reaches.
  //
  // The input is a neutral emission (prototype/single-pass/harness/emit-neutral.ts). Every lowering
  // choice that depends on a type is an intrinsic call ($es*, declared in harness/es-shim.d.ts) that
  // this code resolves while the checker checks the text once:
  //
  //   $esDual(plain, effect, program, meta, id)  a lambda. While chooseOverload tries a candidate, the
  //       lambda is classified against that candidate's declared, uninstantiated parameter type
  //       (FINDINGS.md finding 1) and checked as the variant the class selects: other variants are
  //       never checked against the candidate. Outside a call, its contextual type classifies it.
  //   $esBody(plain, effect, meta, id)  a class method or nested function body.
  //   $esBind(call, id) / $esBind0(callee, call, id) / $esVal0(callee, call, id)  the call rule:
  //       a call whose type is effect's own Effect runs (D24); a non-callable Effect value called
  //       with () runs; anything else is a value.
  //   $esSvc(receiver, "m", id)  a Context key whose service declares m is the service.
  //   $esConv(value, meta, id)  D7 program, $programs, D26 Stream, and D9 plain-variant conversions.
  //
  // Decisions are recorded in `es` and exposed as checker.__es. Nothing here iterates: each choice
  // is made once, from types the checker computes on demand in its usual order.
  var es = {
    selections: [],
    final: new Map(),
    byCandidate: new Map(),
    intrinsics: new Map(),
    calls: new Map(),
    conflicts: []
  };
  var esAliasCache = new Map();
  var esSeeThrough = new Set(["$esDual", "$esBody", "$esBind", "$esBind0", "$esVal0", "$esPlain", "$esSvc", "$esConv"]);
  function esName(node) {
    if (!node || node.kind !== SyntaxKind.CallExpression || node.expression.kind !== SyntaxKind.Identifier) return void 0;
    const name = idText(node.expression);
    return name.startsWith("$es") ? name : void 0;
  }
  function esIsDual(node) {
    return esName(node) === "$esDual";
  }
  function esNum(node) {
    const a = node.arguments[node.arguments.length - 1];
    return a && a.kind === SyntaxKind.NumericLiteral ? +a.text : -1;
  }
  function esMeta(node, index) {
    const a = node.arguments[index];
    return a && a.kind === SyntaxKind.StringLiteral ? JSON.parse(a.text) : {};
  }
  // Called from getContextualType2: an intrinsic's operands take the intrinsic's own contextual type.
  function esContextualOperand(node) {
    const parent = node.parent;
    if (!parent || parent.kind !== SyntaxKind.CallExpression) return false;
    const name = esName(parent);
    if (!name || !esSeeThrough.has(name)) return false;
    const i = parent.arguments.indexOf(node);
    if (i < 0) return false;
    if (name === "$esDual") return i <= 2;
    if (name === "$esBody" || name === "$esBind0" || name === "$esVal0") return i <= 1;
    return i === 0;
  }
  // Called from isContextSensitive.
  function esDualContextSensitive(node) {
    return isContextSensitive(node.arguments[0]) || isContextSensitive(node.arguments[1]) || isContextSensitive(node.arguments[2]);
  }
  // Called from chooseOverload: the candidate list (for the arity-compatible fallback) and the winner.
  function esRec(node) {
    let rec = es.calls.get(node);
    if (!rec) es.calls.set(node, rec = { list: [], current: void 0, winner: void 0 });
    return rec;
  }
  function esNoteCandidate(node, list, candidate) {
    esRec(node).list = list;
  }
  function esNoteWinner(node, candidate) {
    esRec(node).winner = candidate;
  }
  // The signature the call's arguments are being checked against: set while TypeScript infers type
  // arguments from them or checks them for applicability (also when it re-checks a failed candidate
  // to report an error), and restored afterwards.
  function inferTypeArguments(node, signature, args, checkMode, context) {
    const rec = esRec(node);
    const prev = rec.current;
    rec.current = signature;
    try {
      return esInferTypeArgumentsWorker(node, signature, args, checkMode, context);
    } finally {
      rec.current = prev;
    }
  }
  function getSignatureApplicabilityError(node, args, signature, relation, checkMode, reportErrors2, containingMessageChain) {
    const rec = esRec(node);
    const prev = rec.current;
    rec.current = signature;
    try {
      return esGetSignatureApplicabilityErrorWorker(node, args, signature, relation, checkMode, reportErrors2, containingMessageChain);
    } finally {
      rec.current = prev;
    }
  }

  // Called from narrowType and getTypePredicateArgument: the call or reference an intrinsic wraps.
  function esUnwrapRun(expr) {
    return expr.kind === SyntaxKind.YieldExpression && expr.asteriskToken && esName(expr.expression) === "$esBind" ? expr.expression.arguments[0] : void 0;
  }
  function esUnwrapConv(a) {
    return a && esName(a) === "$esConv" ? a.arguments[0] : a;
  }
  function esUnwrapSvc(e) {
    return e && e.kind === SyntaxKind.YieldExpression && esName(e.expression) === "$esSvc" ? e.expression.arguments[0] : e;
  }

  // ---- Types (ports of prototype/esc/src/facts.ts) ----
  function esProp(t, name) {
    return getPropertyOfType(t, escapeLeadingUnderscores(name));
  }
  function esUnwrapSub(t) {
    return t.flags & TypeFlags.Substitution ? esUnwrapSub(t.baseType) : t;
  }
  function esIsEffectType(t, depth = 0) {
    if (depth > 5) return "unknown";
    if (t.flags & (TypeFlags.Any | TypeFlags.Unknown)) return "unknown";
    if (t.flags & TypeFlags.Never) return "no";
    if (t.flags & TypeFlags.Union) {
      const parts = t.types.map((x) => esIsEffectType(x, depth + 1));
      if (parts.every((p) => p === "yes")) return "yes";
      if (parts.every((p) => p === "no")) return "no";
      return "unknown";
    }
    if (t.flags & (TypeFlags.TypeParameter | TypeFlags.Conditional | TypeFlags.IndexedAccess | TypeFlags.Substitution)) {
      const c = getBaseConstraintOfType(t);
      if (!c || c === t) return "unknown";
      return esIsEffectType(c, depth + 1) === "yes" ? "yes" : "unknown";
    }
    return esProp(t, "~effect/Effect") ? "yes" : "no";
  }
  // D24: a call runs only when its type is effect's own Effect interface.
  function esIsEffectItself(t, depth = 0) {
    if (depth > 5) return "unknown";
    if (t.flags & (TypeFlags.Any | TypeFlags.Unknown)) return "unknown";
    if (t.flags & TypeFlags.Never) return "no";
    if (t.flags & TypeFlags.Union) {
      const parts = t.types.map((x) => esIsEffectItself(x, depth + 1));
      if (parts.every((p) => p === "yes")) return "yes";
      if (parts.every((p) => p === "no")) return "no";
      return "unknown";
    }
    if (t.flags & (TypeFlags.TypeParameter | TypeFlags.Conditional | TypeFlags.IndexedAccess | TypeFlags.Substitution)) {
      const c = getBaseConstraintOfType(t);
      if (!c || c === t) return "unknown";
      return esIsEffectItself(c, depth + 1) === "yes" ? "yes" : "unknown";
    }
    if (!esProp(t, "~effect/Effect")) return "no";
    const sym = t.symbol;
    const own = sym && sym.escapedName === "Effect" && (sym.declarations || []).some((d) => /node_modules\/effect\//.test(getSourceFileOfNode(d).fileName));
    return own ? "yes" : "no";
  }
  function esEffectParts(t) {
    const p = esProp(t, "~effect/Effect");
    if (!p) return void 0;
    const variance = getTypeOfSymbol(p);
    const get = (name) => {
      const q = esProp(variance, name);
      if (!q) return void 0;
      const sig = getSignaturesOfType(getTypeOfSymbol(q), SignatureKind.Call)[0];
      return sig ? getReturnTypeOfSignature(sig) : void 0;
    };
    const A = get("_A"), E = get("_E"), R = get("_R");
    return A && E && R ? { A, E, R } : void 0;
  }
  function esClassify(types) {
    let program = false, effectFn = false, plainFn = false;
    const visit = (t0, depth) => {
      const t = esUnwrapSub(t0);
      if (depth > 4) return;
      if (t.flags & TypeFlags.Union) return t.types.forEach((x) => visit(x, depth + 1));
      if (t.flags & TypeFlags.TypeParameter) {
        const c = getBaseConstraintOfType(t);
        if (c && c !== t && !(c.flags & TypeFlags.Unknown)) visit(c, depth + 1);
        return;
      }
      const sigs = getSignaturesOfType(t, SignatureKind.Call);
      if (sigs.length === 0) {
        if (esIsEffectType(t) === "yes") program = true;
        return;
      }
      for (const s of sigs) {
        const r = esUnwrapSub(getReturnTypeOfSignature(s));
        if (esIsEffectType(r) === "yes") effectFn = true;
        else plainFn = true;
      }
    };
    types.forEach((t) => visit(t, 0));
    return program ? "program" : effectFn ? "effectFn" : plainFn ? "plainFn" : "none";
  }
  function esIsBareTypeParam(t) {
    const u = esUnwrapSub(t);
    if (!(u.flags & TypeFlags.TypeParameter)) return false;
    const c = getBaseConstraintOfType(u);
    return !c || c === u || (c.flags & TypeFlags.Unknown) !== 0;
  }
  function esRequiredCount(decl) {
    return decl.parameters.filter((p) => !p.questionToken && !p.initializer && !p.dotDotDotToken).length;
  }
  function esDeclaredAt(decl, index) {
    const sig = getSignatureFromDeclaration(decl);
    if (!sig) return void 0;
    const ps = sig.parameters;
    const last = decl.parameters[decl.parameters.length - 1];
    const rest = last !== void 0 && last.dotDotDotToken !== void 0;
    if (index >= ps.length && !rest) return void 0;
    const p = index < ps.length - (rest ? 1 : 0) ? ps[index] : ps[ps.length - 1];
    if (!p) return void 0;
    let t = getTypeOfSymbol(p);
    if (rest && index >= ps.length - 1) {
      const n = getIndexInfosOfType(t).find((i) => i.keyType.flags & TypeFlags.Number);
      if (n) t = n.type;
    }
    return t;
  }
  function esElementTypes(t) {
    const u = esUnwrapSub(t);
    if (u.flags & TypeFlags.Union) return u.types.flatMap(esElementTypes);
    if (u.flags & TypeFlags.TypeParameter) {
      const c = getBaseConstraintOfType(u);
      return c && c !== u ? esElementTypes(c) : [];
    }
    const out = [];
    for (const info of getIndexInfosOfType(u)) out.push(info.type);
    if (out.length === 0 && (u.target !== void 0 || u.symbol && u.symbol.escapedName === "Iterable")) {
      const args = u.target ? getTypeArguments(u) : [];
      if (args.length > 0) out.push(args[0]);
    }
    return out;
  }
  function esPropertyTypes(t, name) {
    const u = esUnwrapSub(t);
    if (u.flags & TypeFlags.Union) return u.types.flatMap((x) => esPropertyTypes(x, name));
    if (u.flags & TypeFlags.TypeParameter) {
      const c = getBaseConstraintOfType(u);
      return c && c !== u ? esPropertyTypes(c, name) : [];
    }
    const p = esProp(u, name);
    if (p) return [getTypeOfSymbol(p)];
    const s = getIndexInfosOfType(u).find((i) => i.keyType.flags & TypeFlags.String);
    return s ? [s.type] : [];
  }
  function esAlias(name, args) {
    let sym = esAliasCache.get(name);
    if (sym === void 0) {
      sym = getGlobalSymbol(escapeLeadingUnderscores(name), SymbolFlags.TypeAlias, void 0) || null;
      esAliasCache.set(name, sym);
    }
    return sym ? getTypeAliasInstantiation(sym, args) : errorType;
  }

  // ---- Expected types: the prototype's expectedTypes, evaluated inside the pass ----
  // The contextual type the checker has for `node` right now; inside inference, instantiated with
  // the inferences made so far (the prototype reads it after resolution, fully instantiated).
  function esContextualType(node) {
    let t = getContextualType2(node, void 0);
    if (t) {
      const ic = getInferenceContext(node);
      if (ic) t = instantiateType(t, ic.nonFixingMapper);
    }
    return t;
  }
  // The candidate the call is being checked against, else its resolved signature.
  function esCandidateDecl(call) {
    const rec = es.calls.get(call);
    if (rec && rec.current) return rec.current.declaration;
    const s = getNodeLinks(call).resolvedSignature;
    if (s && s !== resolvingSignature) return s.declaration;
    return rec && rec.winner ? rec.winner.declaration : void 0;
  }
  // Declared, uninstantiated parameter types at `index`: the candidate's, else every arity-compatible candidate's.
  function esDeclaredParamTypes(call, index) {
    const rec = es.calls.get(call);
    const decl = esCandidateDecl(call);
    const list = rec ? rec.list : [];
    const primary = decl && !isJSDocSignature(decl) ? esDeclaredAt(decl, index) : void 0;
    if (primary && !esIsBareTypeParam(primary) && esClassify([primary]) !== "none") return [primary];
    const out = primary ? [primary] : [];
    for (const c of list) {
      const d = c.declaration;
      if (!d || isJSDocSignature(d)) continue;
      if (esRequiredCount(d) > call.arguments.length) continue;
      const hasRest = d.parameters.some((p) => p.dotDotDotToken);
      if (!hasRest && d.parameters.length < call.arguments.length) continue;
      const t = esDeclaredAt(d, index);
      if (t) out.push(t);
    }
    return out;
  }
  function esIsEffectGen(call) {
    const e = call.expression;
    return e.kind === SyntaxKind.PropertyAccessExpression && e.expression.kind === SyntaxKind.Identifier && idText(e.expression) === "Effect" && idText(e.name) === "gen";
  }
  function esExpectedTypes(node) {
    const parent = node.parent;
    const fallback = () => {
      const c = esContextualType(node);
      return c ? [c] : [];
    };
    if (parent.kind === SyntaxKind.ParenthesizedExpression) return esExpectedTypes(parent);
    if (parent.kind === SyntaxKind.ReturnStatement) {
      let fn = parent.parent;
      while (fn && !isFunctionLike(fn)) fn = fn.parent;
      const gen = fn && fn.parent;
      if (fn && fn.kind === SyntaxKind.FunctionExpression && fn.asteriskToken && gen && gen.kind === SyntaxKind.CallExpression && esIsEffectGen(gen)) {
        const ctx = esContextualType(gen);
        const as = (ctx ? ctx.flags & TypeFlags.Union ? ctx.types : [ctx] : []).map((t) => {
          const p = esEffectParts(t);
          return p && p.A;
        }).filter((t) => t !== void 0 && !(t.flags & (TypeFlags.Any | TypeFlags.Unknown)));
        if (as.length > 0) return as;
      }
    }
    if (parent.kind === SyntaxKind.CallExpression && parent.arguments.indexOf(node) >= 0) {
      const declared = esDeclaredParamTypes(parent, parent.arguments.indexOf(node));
      const useful = declared.filter((t) => !esIsBareTypeParam(t));
      if (useful.length > 0 && esClassify(useful) !== "none") return useful;
      return fallback();
    }
    if (parent.kind === SyntaxKind.ArrayLiteralExpression) {
      const outer = esExpectedTypes(parent).flatMap(esElementTypes);
      if (outer.length > 0 && esClassify(outer) !== "none") return outer;
      return fallback();
    }
    if (parent.kind === SyntaxKind.PropertyAssignment && parent.parent.kind === SyntaxKind.ObjectLiteralExpression) {
      const name = getTextOfNode(parent.name);
      const outer = esExpectedTypes(parent.parent).flatMap((t) => esPropertyTypes(t, name));
      if (outer.length > 0 && esClassify(outer) !== "none") return outer;
      return fallback();
    }
    return fallback();
  }
  // The call whose argument list holds `node` (through array and object literals), for bookkeeping.
  function esOwnerCall(node) {
    let n = node;
    while (n.parent) {
      const p = n.parent;
      if (p.kind === SyntaxKind.CallExpression && p.arguments.indexOf(n) >= 0 && !esName(p)) return p;
      if (p.kind === SyntaxKind.ParenthesizedExpression || p.kind === SyntaxKind.ArrayLiteralExpression || p.kind === SyntaxKind.ObjectLiteralExpression || p.kind === SyntaxKind.PropertyAssignment) {
        n = p;
        continue;
      }
      return void 0;
    }
    return void 0;
  }
  // D8: pipe(program, stage, ...) when the first stage's parameter type is an Effect.
  function esPipeStageTakesEffect(call) {
    if (!call || call.kind !== SyntaxKind.CallExpression || call.arguments.length < 2) return false;
    let stage = call.arguments[1];
    if (esName(stage) === "$esConv") stage = stage.arguments[0];
    const st = checkExpression(stage);
    const sig = getSignaturesOfType(st, SignatureKind.Call)[0];
    const p0 = sig && sig.parameters[0];
    return !!p0 && esIsEffectType(getTypeOfSymbol(p0)) === "yes";
  }
  function esRecord(id, sel, owner) {
    sel.id = id;
    if (owner) {
      sel.cand = esCandidateDecl(owner);
      sel.owner = owner;
      let m = es.byCandidate.get(id);
      if (!m) es.byCandidate.set(id, m = new Map());
      // The first selection against a candidate is the one its argument types came from.
      const prev = m.get(sel.cand);
      if (prev && prev.mode !== sel.mode) es.conflicts.push({ id, cand: sel.cand, a: prev.mode, b: sel.mode });
      if (!prev) m.set(sel.cand, sel);
    }
    es.selections.push(sel);
    if (!es.final.has(id)) es.final.set(id, sel);
  }
  // A body with no expected type: an Effect function when it performs an Effect operation.
  // Its Effect variant is checked to find out; its intrinsics record what they decided.
  function esNeeds(effectVariant, meta, checkMode) {
    if (meta.s) return true;
    const t = checkExpression(effectVariant, checkMode & ~CheckMode.SkipContextSensitive);
    for (const s of getSignaturesOfType(t, SignatureKind.Call)) getReturnTypeOfSignature(s);
    return (meta.ids || []).some((i) => {
      const d = es.intrinsics.get(i);
      if (!d) return false;
      // D10: a call to an .es function that compiles to plain JavaScript does not pause. (A call
      // back into the function that encloses this body is not forced; it counts as an operation.)
      if (d.esCallee && d.type === "effect" && !isNodeDescendantOf(effectVariant, d.esCallee) && esDirect(d.esCallee)) return false;
      return d.type === "effect" || d.effectValue || d.service || d.throws;
    });
  }

  // ---- The intrinsics ----
  function esCheckCall(node, checkMode) {
    switch (esName(node)) {
      case "$esDual":
        return esCheckDual(node, checkMode);
      case "$esBody":
        return esCheckBody(node, checkMode);
      case "$esBind":
        return esCheckBind(node, node.arguments[0], checkMode, esNum(node));
      case "$esBind0":
        return esCheckBind0(node, checkMode, true);
      case "$esVal0":
        return esCheckBind0(node, checkMode, false);
      case "$esPlain":
        return esCheckPlain(node, node.arguments[0], checkMode, esNum(node));
      case "$esSvc":
        return esCheckSvc(node, checkMode);
      case "$esConv":
        return esCheckConv(node, checkMode);
      case "$esIter": {
        const t = checkExpression(node.arguments[0], checkMode);
        es.intrinsics.set(esNum(node), { kind: "iter", loop: esProp(t, "~esc/Producer") ? "producer" : esProp(t, "~effect/Stream") ? "stream" : "plain" });
        return void 0;
      }
    }
    return void 0;
  }
  function esCheckDual(node, checkMode) {
    const id = esNum(node);
    const meta = esMeta(node, 3);
    // An object-literal method has no separate program form: "program" compiles as its Effect form.
    const variants = [node.arguments[0], node.arguments[1], meta.n && !meta.m ? node.arguments[2] : node.arguments[1]];
    const owner = esOwnerCall(node);
    // D8 first (the prototype's lambdaKind checks pipePrograms before the lambda's own class).
    const pipe = !!meta.p && esPipeStageTakesEffect(node.parent);
    const ctx = esClassify(esExpectedTypes(node));
    let mode, needs, via = "expected";
    if (pipe && meta.n) {
      mode = "program";
      via = "pipe";
    } else if (ctx === "program" && meta.n) mode = "program";
    else if (ctx === "program" || ctx === "effectFn") mode = "effect";
    else if (ctx === "plainFn") mode = "plain";
    // First round of overload resolution: TypeScript skips context-sensitive arguments. Skip the
    // dual only if the variant this candidate selects is context sensitive: a program variant is a
    // call, checked now like any other argument. A class that needs the body is decided in round 2.
    if (checkMode & CheckMode.SkipContextSensitive) {
      const v = mode === "plain" ? variants[0] : mode === "effect" ? variants[1] : mode === "program" ? variants[2] : void 0;
      if (v ? isContextSensitive(v) : esDualContextSensitive(node)) return anyFunctionType;
    }
    if (!mode) {
      needs = esNeeds(variants[1], meta, checkMode);
      mode = needs ? "effect" : "plain";
      via = "needs";
    }
    esRecord(id, { kind: "dual", mode, ctx, via, needs, pipe }, owner);
    const variant = mode === "plain" ? variants[0] : mode === "effect" ? variants[1] : variants[2];
    return checkExpression(variant, checkMode);
  }
  function esCheckBody(node, checkMode) {
    const id = esNum(node);
    const meta = esMeta(node, 2);
    let fn = node.parent;
    while (fn && !isFunctionLike(fn)) fn = fn.parent;
    let ctx = "none";
    if (fn && fn.kind === SyntaxKind.MethodDeclaration && isClassLike(fn.parent)) {
      const impl = getEffectiveImplementsTypeNodes(fn.parent) || [];
      const types = impl.map((t) => getTypeFromTypeNode(t));
      ctx = esClassify(types.flatMap((t) => esPropertyTypes(t, getTextOfNode(fn.name))));
    }
    let mode = ctx === "plainFn" ? "plain" : ctx === "none" ? void 0 : "effect";
    let needs;
    if (!mode) {
      needs = esNeeds(node.arguments[1], meta, checkMode);
      mode = needs ? "effect" : "plain";
    }
    esRecord(id, { kind: "body", mode, ctx, via: ctx === "none" ? "needs" : "implements", needs });
    const thunk = checkExpression(mode === "plain" ? node.arguments[0] : node.arguments[1], checkMode);
    const sig = getSignaturesOfType(thunk, SignatureKind.Call)[0];
    return sig ? getReturnTypeOfSignature(sig) : errorType;
  }
  // A call in an Effect function. D23: a plain call to a trusted TypeScript function with @throws fails with those types.
  function esCheckBind(node, x, checkMode, id) {
    const T = checkExpression(x, checkMode);
    const tri = esIsEffectItself(T);
    const d = { kind: "bind", type: tri === "yes" ? "effect" : tri === "no" ? "plain" : "unknown", esCallee: esCalleeOf(x) };
    let result;
    if (d.type === "effect") result = T;
    else if (d.type === "plain") {
      const throws = esTrustedThrows(x);
      if (throws) {
        d.throws = true;
        result = esAlias("$EsTrusted", [T, throws]);
      } else result = esAlias("$EsSucceed", [T]);
    } else result = esAlias("$EsBind", [T]);
    es.intrinsics.set(id, d);
    return result;
  }
  function esCheckBind0(node, checkMode, effectMode) {
    const id = esNum(node);
    const F = checkExpression(node.arguments[0], checkMode);
    if (getSignaturesOfType(F, SignatureKind.Call).length === 0 && esIsEffectType(F) === "yes") {
      es.intrinsics.set(id, { kind: effectMode ? "bind0" : "val0", type: "effect", effectValue: true });
      return F;
    }
    if (effectMode) return esCheckBind(node, node.arguments[1], checkMode, id);
    return esCheckPlain(node, node.arguments[1], checkMode, id);
  }
  // A call in plain code. D10: calling an .es function that compiles to plain JavaScript calls its
  // plain variant, so the call's value is the function's A. (A call to any other .es function from
  // plain code is ES4301, and keeps its Effect type.)
  function esCheckPlain(node, call, checkMode, id) {
    const C = checkExpression(call, checkMode);
    const callee = esCalleeOf(call);
    if (callee && !isNodeDescendantOf(node, callee) && esDirect(callee)) {
      const parts = esEffectParts(C);
      if (parts) {
        const tri = esIsEffectItself(parts.A);
        es.intrinsics.set(id, { kind: "val0", type: tri === "yes" ? "effect" : tri === "no" ? "plain" : "unknown", esCallee: callee, direct: true });
        return parts.A;
      }
    }
    const tri = esIsEffectItself(C);
    es.intrinsics.set(id, { kind: "val0", type: tri === "yes" ? "effect" : tri === "no" ? "plain" : "unknown", esCallee: callee });
    return C;
  }
  // FINDINGS finding 4: a member the service's shape declares wins over the key's own members.
  function esCheckSvc(node, checkMode) {
    const id = esNum(node);
    const member = node.arguments[1].text;
    const R = checkExpression(node.arguments[0], checkMode);
    const isKey = !!esProp(R, "~effect/Context/Service");
    const shape = isKey ? esProp(R, "Service") : void 0;
    let service = false;
    if (shape && esProp(getTypeOfSymbol(shape), member)) service = true;
    else if (isKey && !esProp(R, member) && !esProp(getApparentType(R), member)) service = true;
    es.intrinsics.set(id, { kind: "svc", service });
    return service ? R : esAlias("$EsSucceed", [R]);
  }
  function esIsZeroArgProgramFn(t) {
    return esProgramSig(t) !== void 0;
  }
  function esProgramSig(t) {
    return getSignaturesOfType(t, SignatureKind.Call).find((s) => {
      const d = s.declaration;
      const required = d && !isJSDocSignature(d) ? esRequiredCount(d) : s.parameters.length;
      return required === 0 && esIsEffectType(getReturnTypeOfSignature(s)) === "yes";
    });
  }
  function esCheckConv(node, checkMode) {
    const id = esNum(node);
    const meta = esMeta(node, 1);
    const x = node.arguments[0];
    const own = checkExpression(x, checkMode);
    const d = { kind: "conv", program: false, programs: false, stream: false, direct: false, rejectPlain: false, pipe: false };
    let result = own;
    if (meta.p && esPipeStageTakesEffect(node.parent)) {
      d.program = d.pipe = true;
      const sig = esProgramSig(own);
      if (sig) result = getReturnTypeOfSignature(sig);
    } else {
      const expected = esExpectedTypes(node);
      const ctx = esClassify(expected);
      d.ctx = ctx;
      if (ctx === "program" && esIsZeroArgProgramFn(own)) {
        d.program = true;
        result = getReturnTypeOfSignature(esProgramSig(own));
      }
      if (esProp(own, "~esc/Producer") && expected.some((t) => esProp(t, "~effect/Stream"))) {
        d.stream = true;
        result = esAlias("$EsStreamOf", [own]);
      }
      if (ctx === "none" && !d.program && !d.stream) {
        const expectedEl = expected.flatMap(esElementTypes);
        const ownEl = esElementTypes(own);
        if (esClassify(expectedEl) === "program" && ownEl.length > 0 && ownEl.every(esIsZeroArgProgramFn)) {
          d.programs = true;
          result = createArrayType(getUnionType(ownEl.map((t) => getReturnTypeOfSignature(esProgramSig(t)))));
        }
      }
      if (ctx === "plainFn") {
        const fn = esFnDeclOf(x);
        if (fn) {
          if (esDirect(fn)) {
            d.direct = true;
            result = esAlias("$EsDirectOf", [own]);
          } else if (!esMeta(fn.initializer, 1).producer) d.rejectPlain = true;
        }
      }
    }
    es.intrinsics.set(id, d);
    return result;
  }

  // ---- D10: does an .es function compile to plain JavaScript? (computeDirect, on demand) ----
  function esFnDeclOf(x) {
    if (!x || x.kind !== SyntaxKind.Identifier) return void 0;
    let sym = getResolvedSymbol(x);
    if (!sym || sym === unknownSymbol) return void 0;
    if (sym.flags & SymbolFlags.Alias) sym = resolveAlias(sym);
    const d = sym.declarations && sym.declarations[0];
    if (!d || d.kind !== SyntaxKind.VariableDeclaration || !d.initializer || esName(d.initializer) !== "$esFn") return void 0;
    return d;
  }
  function esCalleeOf(call) {
    if (!call || call.kind !== SyntaxKind.CallExpression) return void 0;
    return esFnDeclOf(call.expression);
  }
  // Greatest fixpoint over the functions reachable through .es callees, as computeDirect does.
  function esDirect(decl) {
    const nodes = new Map();
    const queue = [decl];
    while (queue.length > 0) {
      const d = queue.pop();
      if (nodes.has(d)) continue;
      checkExpressionCached(d.initializer);
      const meta = esMeta(d.initializer, 1);
      let ok = !meta.bad && !meta.producer;
      const deps = [];
      for (const i of meta.ids || []) {
        const r = es.intrinsics.get(i);
        if (!r) {
          ok = false;
          continue;
        }
        if (r.kind === "iter") {
          if (r.loop !== "plain") ok = false;
          continue;
        }
        if (r.kind === "svc") {
          if (r.service) ok = false;
          continue;
        }
        if (r.esCallee) {
          if (!esMeta(r.esCallee.initializer, 1).producer) {
            deps.push(r.esCallee);
            queue.push(r.esCallee);
          }
          continue;
        }
        if (r.effectValue || r.type !== "plain" || r.throws) ok = false;
      }
      nodes.set(d, { ok, deps });
    }
    let changed = true;
    while (changed) {
      changed = false;
      for (const n of nodes.values()) {
        if (n.ok && n.deps.some((x) => !nodes.get(x).ok)) {
          n.ok = false;
          changed = true;
        }
      }
    }
    return nodes.get(decl).ok;
  }
  // D23: `@effectscript trusted` plus `@throws {T}` on the callee's declaration.
  function esTrustedThrows(call) {
    if (!call || call.kind !== SyntaxKind.CallExpression) return void 0;
    const decls = [];
    const links = getNodeLinks(call);
    const sig = links.resolvedSignature && links.resolvedSignature !== resolvingSignature ? links.resolvedSignature : void 0;
    if (sig && sig.declaration && !isJSDocSignature(sig.declaration)) decls.push(sig.declaration);
    if (call.expression.kind === SyntaxKind.Identifier) {
      let sym = getResolvedSymbol(call.expression);
      if (sym && sym.flags & SymbolFlags.Alias) sym = resolveAlias(sym);
      for (const x of sym && sym.declarations || []) decls.push(x);
    }
    for (const decl of decls) {
      const tags = getJSDocTags(decl);
      if (!tags.some((t) => idText(t.tagName) === "effectscript" && /trusted/.test(String(t.comment ?? "")))) continue;
      const types = tags.filter((t) => idText(t.tagName) === "throws" && t.typeExpression).map((t) => getTypeFromTypeNode(t.typeExpression.type));
      return types.length > 0 ? getUnionType(types) : void 0;
    }
    return void 0;
  }
  // ===== end of the EffectScript single-pass experiment =====
