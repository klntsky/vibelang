/**
 * @file VibeLang grammar for tree-sitter: TypeScript's grammar and VibeLang's forms
 * @license MIT
 *
 * VibeLang (.vibe) is TypeScript with short forms for Effect programs. The
 * compiler's parser, typescript/tsc/internal/parser/vibelang*.go, defines each
 * form; the comments here name the decision (D…) and the parser file.
 */

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

const TypeScript = require('tree-sitter-typescript/typescript/grammar');

module.exports = grammar(TypeScript, {
  name: 'vibe',

  precedences: ($, previous) => previous.filter(
    // tree-sitter-typescript 0.23.2 reads f<typeof import("m")>() as a
    // comparison: preferring call_expression to _type_query_call_expression
    // decides before the > that closes the type arguments. Without that
    // precedence both readings continue (see conflicts).
    (list) => list.map((rule) => rule.name).join() !== 'call_expression,_type_query_call_expression',
  ).concat([
    // An Effect type's slots hold unions, and a function type returns an
    // Effect type: () => A!E | F!R is () => A!(E | F)!R.
    [$.union_type, $.effect_type, $.function_type],
    [$.readonly_type, $.effect_type],
    // A!E.F and A!E<T> continue the slot's type.
    ['member', 'call', $._effect_slot_primary],
    // In a type, x is T stays a type predicate.
    [$.type_predicate, $.primary_type],
    [$.type_predicate, $.asserts],
    [$.effect_type, $.type_predicate],
    // catch binds looser than || and tighter than ?: (vibelang_catch.go); a
    // pipe binds looser than ?: and tighter than assignment (vibelang_pipes.go).
    // ?? shares ?:'s level here, so a ?? b catch c groups as a ?? (b catch c),
    // where the compiler reads (a ?? b) catch c.
    [
      'member',
      'template_call',
      'call',
      $.update_expression,
      'unary_void',
      'binary_exp',
      'binary_times',
      'binary_plus',
      'binary_shift',
      'binary_compare',
      'binary_relation',
      'binary_equality',
      'bitwise_and',
      'bitwise_xor',
      'bitwise_or',
      'logical_and',
      'logical_or',
      'catch',
      'ternary',
      'pipe',
      $.sequence_expression,
      $.arrow_function,
      'throw',
    ],
    ['binary', 'catch'],
    ['pipe', 'assign'],
    // A { after a name, a property access, or a call is a block param only
    // where TypeScript cannot read it: after extends or provide's providers,
    // it opens the class body or provide's block (vibelang_blocks.go).
    [$.primary_expression, 'block_param'],
    ['member', 'block_param'],
    ['call', 'block_param'],
    // A { that can open a block opens one, as for statement_block.
    [$._block, 'object'],
    // with (x) { ... } stays JavaScript's with statement.
    [$.with_statement, $.primary_expression],
  ]),

  conflicts: ($, previous) => previous.concat([
    // a < b against a<T>(x), as TypeScript's await has them.
    [$.call_expression, $.instantiation_expression, $.binary_expression, $.fork_expression],
    [$.call_expression, $.instantiation_expression, $.binary_expression, $.join_expression],
    // do { ... } starting a statement is a do-while statement (vibelang_blocks.go).
    [$.statement_block, $.object, $._block],
    [$.statement_block, $._block],
    // f<typeof import("m")>() against f < typeof import("m") > ().
    [$.call_expression, $._type_query_call_expression],
    // provide(x) and provide[i] stay a call and an element access.
    [$.primary_expression, $.provide_statement],
    [$.primary_expression, $._property_name, $.provide_statement],
    // fork(x) and join(x) stay calls.
    [$.primary_expression, $.fork_expression],
    [$.primary_expression, $.join_expression],
    [$.primary_expression, $._property_name, $.fork_expression],
    [$.primary_expression, $._property_name, $.join_expression],
    // match(x) stays a call.
    [$.primary_expression, $.match_expression],
    [$.primary_expression, $._property_name, $.match_expression],
  ]),

  rules: {
    // ---------------------------------------------------------- Modules

    // module text { ... } at a file's top level (D283, vibelang_modules.go).
    // Its body is TypeScript, and this grammar gives TypeScript code
    // tree-sitter-typescript's nodes, so the body is a statement_block of
    // TypeScript statements that the inherited TypeScript queries color.
    // module a.b { }, declare module x { }, export module x { }, a module
    // inside a block, and namespace x { } stay TypeScript's module nodes.
    program: $ => seq(
      optional($.hash_bang_line),
      repeat(choice($.statement, $.module_declaration)),
    ),

    module_declaration: $ => prec(1, seq(
      'module',
      field('name', $.identifier),
      field('body', $.statement_block),
    )),

    // import { clean } from text, export * from text, and import text: an
    // identifier module specifier names a module declaration (D283).
    _from_clause: $ => seq('from', field('source', choice($.string, $.identifier))),

    import_statement: $ => seq(
      'import',
      optional(choice('type', 'typeof')),
      choice(
        seq($.import_clause, $._from_clause),
        $.import_require_clause,
        field('source', choice($.string, $.identifier)),
      ),
      optional($.import_attribute),
      $._semicolon,
    ),

    // ------------------------------------------------------------ Types

    type: ($, previous) => choice(previous, $.effect_type),

    // Result!Error!Dependencies (D127, vibelang.go): User!NotFound, number!!,
    // string!!Approval. An empty slot is never; a slot does not start with {,
    // which opens a body.
    effect_type: $ => prec.right(seq(
      field('success', $.type),
      '!',
      optional(field('error', $._effect_slot)),
      optional(seq('!', optional(field('requirements', $._effect_slot)))),
    )),

    // A slot is a type that cannot start with {, so that a function's body
    // can follow: unions, intersections, and arrays of the types below.
    _effect_slot: $ => choice(
      $._effect_slot_operand,
      alias($.effect_slot_union, $.union_type),
    ),

    effect_slot_union: $ => prec.left(1, seq(
      choice($._effect_slot_operand, alias($.effect_slot_union, $.union_type)),
      '|',
      $._effect_slot_operand,
    )),

    _effect_slot_operand: $ => choice(
      $._effect_slot_primary,
      alias($.effect_slot_intersection, $.intersection_type),
    ),

    effect_slot_intersection: $ => prec.left(1, seq(
      choice($._effect_slot_primary, alias($.effect_slot_intersection, $.intersection_type)),
      '&',
      $._effect_slot_primary,
    )),

    _effect_slot_primary: $ => choice(
      $.parenthesized_type,
      $.predefined_type,
      $._type_identifier,
      $.nested_type_identifier,
      $.generic_type,
      alias($.effect_slot_array, $.array_type),
      $.tuple_type,
      $.type_query,
      $.index_type_query,
      alias($.this, $.this_type),
      $.literal_type,
      $.template_literal_type,
      $.infer_type,
    ),

    effect_slot_array: $ => prec(1, seq($._effect_slot_primary, '[', ']')),

    // Members are readonly unless written mutable (D134, vibelang.go).
    property_signature: ($, previous) => seq(optional('mutable'), previous),

    index_signature: ($, previous) => seq(optional('mutable'), previous),

    // ------------------------------------------------------ Declarations

    declaration: ($, previous) => choice(
      previous,
      $.error_declaration,
      $.service_declaration,
      $.control_declaration,
      $.schema_declaration,
      $.layer_declaration,
    ),

    // error NotFound { id: string }, error Timeout (D126). An error is a
    // class (D278): abstract, extends and implements, and a class body, whose
    // fields are field definitions; schema error FsError { method: string }
    // (D279). A { after the head is the body, never a block.
    error_declaration: $ => prec.right(seq(
      optional('abstract'),
      optional('schema'),
      'error',
      field('name', $._type_identifier),
      field('type_parameters', optional($.type_parameters)),
      optional(choice(
        prec.dynamic(10, seq(optional($.class_heritage), field('body', $.class_body))),
        // error Timeout extends AppError ends at its line.
        seq($.class_heritage, $._semicolon),
      )),
    )),

    // An error's field may be written mutable (D134).
    public_field_definition: ($, previous) => seq(optional('mutable'), previous),

    // service Users { find(id: string): User!NotFound } (D128, D223), the
    // older spelling service class Users { ... } (D220), and a service of any
    // shape, service Prefix: string or service Search: SearchApi { static
    // members } (D281), whose body is optional.
    service_declaration: $ => prec.right(seq(
      'service',
      optional('class'),
      field('name', $._type_identifier),
      field('type_parameters', optional($.type_parameters)),
      choice(
        field('body', $.class_body),
        seq(':', field('shape', $.type), field('body', optional($.class_body))),
      ),
    )),

    // control Approval { approve(request: string): boolean!! } (D128, D223).
    control_declaration: $ => seq(
      'control',
      optional('class'),
      field('name', $._type_identifier),
      field('type_parameters', optional($.type_parameters)),
      field('body', $.class_body),
    ),

    // schema Account { id: string; nickname?: string } (D133), and the alias
    // form schema Code = "a" | "b" (D280).
    schema_declaration: $ => prec.right(seq(
      'schema',
      field('name', $._type_identifier),
      optional(choice(
        field('body', $.object_type),
        seq('=', field('value', $.type), $._semicolon),
      )),
    )),

    // layer UsersLive: Users { setup; find(id) { ... } readonly region = "eu" },
    // the spelling before Layer.effect(Users) { ... } (D221).
    layer_declaration: $ => seq(
      'layer',
      field('name', $.identifier),
      ':',
      field('service', choice($.identifier, $.nested_identifier)),
      field('body', $.layer_body),
    ),

    // Its methods, name(params) { ... }, read as block params.
    layer_body: $ => seq('{', repeat(choice($.statement, $.layer_value)), '}'),

    layer_value: $ => seq(
      'readonly',
      field('name', alias($.identifier, $.property_identifier)),
      '=',
      field('value', $.expression),
      $._semicolon,
    ),

    // ------------------------------------------------------- Statements

    statement: ($, previous) => choice(previous, $.provide_statement),

    // provide UsersLive { ... }, provide AppLive, Random = seeded { ... }
    // (D130, D172, D173), and the old spelling with UsersLive { ... } (EK1009).
    provide_statement: $ => prec.dynamic(1, seq(
      choice('provide', 'with'),
      field('providers', $._expressions),
      field('body', $.statement_block),
    )),

    // try { ... } handle Approval.approve(request) { ... } catch (e) { ... }
    // finally (exit) { ... } (D132, D176, D178): handle clauses, then catch
    // clauses, typed or not, then finally.
    try_statement: $ => seq(
      'try',
      field('body', $.statement_block),
      repeat(field('handler', $.handle_clause)),
      repeat(field('handler', $.catch_clause)),
      optional(field('finalizer', $.finally_clause)),
    ),

    handle_clause: $ => seq(
      'handle',
      field('operation', $.nested_identifier),
      field('parameters', $.formal_parameters),
      field('body', $.statement_block),
    ),

    finally_clause: $ => seq(
      'finally',
      optional(seq('(', field('parameter', $.identifier), ')')),
      field('body', $.statement_block),
    ),

    // TypeScript's for (using x of xs), which tree-sitter-typescript 0.23.2
    // does not parse, and for (const x of xs).
    _for_header: $ => seq(
      '(',
      choice(
        field('left', choice($._lhs_expression, $.parenthesized_expression)),
        seq(
          field('kind', 'var'),
          field('left', choice($.identifier, $._destructuring_pattern)),
          optional($._initializer),
        ),
        seq(
          field('kind', choice('let', 'const', 'using', seq('await', 'using'))),
          field('left', choice($.identifier, $._destructuring_pattern)),
          optional($._automatic_semicolon),
        ),
      ),
      field('operator', choice('in', 'of')),
      field('right', $._expressions),
      ')',
    ),

    // for concurrent(8) (const id of ids) save(id) (D169, vibelang_fibers.go).
    for_in_statement: ($, previous) => choice(
      previous,
      seq(
        'for',
        'concurrent',
        optional(field('limit', $.parenthesized_expression)),
        $._for_header,
        field('body', $.statement),
      ),
    ),

    // ------------------------------------------------------ Expressions

    expression: ($, previous) => choice(
      previous,
      $.is_expression,
      $.catch_expression,
      $.pipe_expression,
      $.throw_expression,
      $.fork_expression,
      $.join_expression,
    ),

    primary_expression: ($, previous) => choice(
      previous,
      $.match_expression,
      $.do_expression,
      $.topic_reference,
    ),

    // x catch fallback, x catch (e) => handler (D236, vibelang_catch.go).
    catch_expression: $ => prec.left('catch', seq(
      field('left', $.expression),
      'catch',
      field('handler', $.expression),
    )),

    // x |> f(%) (D154, vibelang_pipes.go).
    pipe_expression: $ => prec.left('pipe', seq(
      field('left', $.expression),
      '|>',
      field('right', $.expression),
    )),

    // %, the topic of a pipe.
    topic_reference: _ => '%',

    // users.get(id) ?? throw new NotFound({ id }) (D186). The compiler reads a
    // unary operand; here throw takes the whole expression after it, as the
    // throw statement does, so a statement that starts with throw stays one.
    throw_expression: $ => prec.right('throw', seq(
      'throw',
      field('argument', $.expression),
    )),

    throw_statement: ($, previous) => prec.dynamic(1, previous),

    // do { ... } (D187): the block's value is its last statement's.
    do_expression: $ => seq(
      'do',
      field('body', alias($._block, $.statement_block)),
    ),

    // fork getUser(id), fork { ... } (D166); join fiber, join [a, b] (D225).
    fork_expression: $ => prec.right('unary_void', choice(
      prec.dynamic(1, seq('fork', field('argument', alias($._block, $.statement_block)))),
      prec.dynamic(-1, seq('fork', field('argument', $.expression))),
    )),

    // join [a, b] joins an array of fibers. The compiler reads join[i], without
    // the space, as an element access; this grammar does not see the space.
    join_expression: $ => prec.right('unary_void', choice(
      prec.dynamic(1, seq('join', field('argument', $.array))),
      prec.dynamic(-1, seq('join', field('argument', $.expression))),
    )),

    // await.all [a(), b()], and allSettled, race, and any (D196).
    await_expression: ($, previous) => choice(
      previous,
      prec('unary_void', seq(
        'await',
        '.',
        field('operation', choice('all', 'allSettled', 'race', 'any')),
        field('argument', $.expression),
      )),
    ),

    // match (x) { when P: v; default: v; } (D206-D225, vibelang_match.go).
    match_expression: $ => prec.dynamic(1, seq(
      'match',
      field('subject', $.parenthesized_expression),
      field('body', $.match_body),
    )),

    match_body: $ => seq('{', repeat($.match_arm), '}'),

    match_arm: $ => seq(
      choice(seq('when', field('pattern', $._match_pattern)), 'default'),
      ':',
      field('value', $.expression),
      $._semicolon,
    ),

    // x is P (D211): binds as instanceof does.
    is_expression: $ => prec.left('binary_relation', seq(
      field('left', $.expression),
      'is',
      field('pattern', $._match_pattern),
    )),

    // f(args) { block } and f { block } (D188, vibelang_blocks.go): the block
    // is the call's last argument, a program.
    call_expression: ($, previous) => choice(
      previous,
      prec('block_param', seq(
        field('function', $.expression),
        field('type_arguments', optional($.type_arguments)),
        field('arguments', $.arguments),
        field('block', alias($._block, $.statement_block)),
      )),
      prec('block_param', seq(
        field('function', choice($.identifier, $.member_expression)),
        field('block', alias($._block, $.statement_block)),
      )),
      // try(fn, ...), the boundary hatch (D148, vibelang_hatches.go).
      seq(
        field('function', alias('try', $.identifier)),
        field('arguments', $.arguments),
      ),
    ),

    _block: $ => seq('{', repeat($.statement), '}'),

    // ---------------------------------------------------------- Patterns

    _match_pattern: $ => choice($.combined_pattern, $._unary_match_pattern),

    // Patterns joined by and, or by or.
    combined_pattern: $ => prec.left(seq(
      field('left', $._match_pattern),
      field('operator', choice('and', 'or')),
      field('right', $._unary_match_pattern),
    )),

    _unary_match_pattern: $ => choice($.not_pattern, $._primary_match_pattern),

    not_pattern: $ => seq('not', field('pattern', $._unary_match_pattern)),

    _primary_match_pattern: $ => choice(
      $.parenthesized_pattern,
      $.binding_pattern,
      $.array_match_pattern,
      $.object_match_pattern,
      $.guard_pattern,
      $.extractor_pattern,
      $._match_value,
      $.string,
      $.number,
      alias($._negative_number, $.unary_expression),
      $.template_string,
      $.true,
      $.false,
      $.null,
      $.undefined,
    ),

    parenthesized_pattern: $ => seq('(', $._match_pattern, ')'),

    // let id, const id, var id: binds the value.
    binding_pattern: $ => seq(
      field('kind', choice('let', 'const', 'var')),
      field('name', $.identifier),
    ),

    array_match_pattern: $ => seq(
      '[',
      commaSep(choice($._match_pattern, $.rest_match_pattern)),
      optional(','),
      ']',
    ),

    object_match_pattern: $ => seq(
      '{',
      commaSep(choice(
        $.pair_match_pattern,
        $.binding_pattern,
        $.rest_match_pattern,
        field('key', $._property_name),
      )),
      optional(','),
      '}',
    ),

    pair_match_pattern: $ => seq(
      field('key', $._property_name),
      ':',
      field('value', $._match_pattern),
    ),

    rest_match_pattern: $ => prec.right(seq('...', optional($._match_pattern))),

    guard_pattern: $ => seq('if', field('condition', $.parenthesized_expression)),

    // Exit.Failure(let cause), Option.Some(let n).
    extractor_pattern: $ => prec(1, seq(
      field('extractor', $._match_value),
      '(',
      commaSep(choice($._match_pattern, $.rest_match_pattern)),
      optional(','),
      ')',
    )),

    // A name, then .name and [expression] accesses.
    _match_value: $ => choice(
      $.identifier,
      $.this,
      alias($.match_member_expression, $.member_expression),
      alias($.match_subscript_expression, $.subscript_expression),
    ),

    match_member_expression: $ => prec(1, seq(
      field('object', $._match_value),
      '.',
      field('property', alias($.identifier, $.property_identifier)),
    )),

    match_subscript_expression: $ => prec(1, seq(
      field('object', $._match_value),
      '[',
      field('index', $._expressions),
      ']',
    )),

    _negative_number: $ => seq(field('operator', choice('-', '+')), field('argument', $.number)),

    // Words of VibeLang that stay names where no form starts.
    _reserved_identifier: ($, previous) => choice(
      previous,
      'error',
      'service',
      'control',
      'schema',
      'layer',
      'provide',
      'fork',
      'join',
      'match',
      'mutable',
    ),
  },
});

/**
 * @param {RuleOrLiteral} rule
 * @returns {ChoiceRule}
 */
function commaSep(rule) {
  return optional(seq(rule, repeat(seq(',', rule))));
}
