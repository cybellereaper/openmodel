# purelangc — PureLang self-hosted compiler bootstrap

This directory holds the **seed** of a future self-hosted PureLang
compiler. It is a regular PureLang project (with its own `pure.toml`) so
it can be built and run with the existing reference compiler:

```sh
pr run compiler            # runs compiler/src/main.pure
pr run compiler/src/lexer.pure
```

## Goal

The reference PureLang compiler is implemented in Go under `internal/`.
The long-term plan is to rewrite it in PureLang itself. This directory is
the entry point for that effort.

## Current scope

Today, the bootstrap restricts itself to the subset of PureLang already
supported by the reference compiler:

- top-level immutable bindings
- functions with `=>` and block bodies
- data declarations with computed fields
- pattern matching with `when` (subject and subjectless forms)
- string interpolation (`$ident` / `${expr}`)
- standard library imports: `std.io`, `std.string`, `std.list`

Inside `src/lexer.pure` you'll find:

- a `Token` data type with a computed `show` property
- character classification helpers (`is_letter`, `is_digit`)
- a `keyword_kind` function using `when` to map identifiers to keyword
  tokens
- a `classify` function demonstrating subjectless `when`

## Roadmap

Stage 1 — what's here today:

- [x] Token type
- [x] Character classification
- [x] Keyword classification

Stage 2 — additional language features needed before the lexer can
fully scan a `.pure` file:

- [ ] Mutable arrays / `push` (already in `std.list`) usable inside
      functions for token accumulation
- [ ] String slicing (`s.substring(start, end)` or similar)
- [ ] Index access into strings (`s[i]`) returning a single-character
      string
- [ ] Bytes/runes interop helpers in `std.string`

Stage 3 — full self-hosting:

- [ ] AST types written in PureLang
- [ ] Pratt parser written in PureLang
- [ ] Type checker written in PureLang
- [ ] Code generation back-end (initially targeting the bytecode VM)
- [ ] `pr selfhost build` produces a `pr` binary by compiling this
      project with the reference compiler, then re-compiles the same
      sources using its own output (`pr-bootstrap`), confirming a
      successful bootstrap fixed point.

The reference compiler exposes `pr selfhost lex <file>` to run the
PureLang-implemented lexer against a file.
