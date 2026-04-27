# PureLang

PureLang is a clean, strongly typed programming language inspired by Swift,
Kotlin, and Ruby. It removes boilerplate while keeping modern safety features
like immutable-by-default variables, type inference, null safety, and concise
data types.

This repository contains the **reference implementation** of PureLang in Go.
It includes:

- A lexer, parser, type checker, and tree-walking interpreter.
- A bytecode compiler and stack VM (`pr run --vm`).
- A native compiler that emits Go and produces a real binary
  (`pr build --native`).
- A formatter (`pr fmt`).
- A Language Server Protocol server (`pr lsp`) over JSON-RPC stdio.
- A package manager that supports both Git URLs and the **purepkg**
  registry (`pr deps`, `pr deps add`, `pr pkg`).
- A growing standard library: `std.io`, `std.list`, `std.string`, `std.math`,
  `std.os`, `std.fs`.
- A self-hosted compiler bootstrap project under `compiler/`.
- A Visual Studio Code extension under `editors/vscode/`.

| Property                    | Value             |
| --------------------------- | ----------------- |
| Source extension            | `.pure`           |
| Compiler / interpreter      | `pr`              |
| Project file                | `pure.toml`       |
| Lock file                   | `pure.lock`       |
| Standard library namespace  | `std`             |
| Dependency cache            | `.pure/deps/`     |
| Implementation language     | Go (stdlib only)  |
| Registry env override       | `PUREPKG_URL`     |
| Registry auth env           | `PUREPKG_TOKEN`   |

## Building

```sh
go build -o pr ./cmd/pr
./pr version
```

## Running single files

```sh
./pr run examples/hello.pure
./pr run examples/user.pure
./pr run examples/functions.pure
```

## Creating a project

```sh
./pr new my_app
cd my_app
../pr run
```

## Running projects

```sh
./pr run examples_project
./pr check examples_project
```

When you run `pr run` inside a project directory, the compiler:

1. Walks upward to find `pure.toml`.
2. Loads project metadata.
3. Installs missing dependencies (or prints a helpful error).
4. Parses every `.pure` file under `src/` and inside `.pure/deps/<dep>/src/`.
5. Builds a per-file module graph and links imports through `use`.
6. Executes the configured entry file.

## Adding GitHub dependencies

```sh
./pr deps                                         # install / refresh
./pr deps add math https://github.com/example/pure-math.git
./pr deps update                                  # refresh branch deps
./pr deps clean                                   # delete .pure/deps
```

A dependency in `pure.toml` looks like one of these:

```toml
[dependencies]
math = { git = "https://github.com/example/pure-math.git", version = "v0.1.0" }
json = { git = "https://github.com/example/pure-json.git", branch = "main" }
core = { git = "https://github.com/example/pure-core.git", commit = "abc123" }
```

## Adding registry dependencies (purepkg)

```sh
./pr deps add --pkg math@0.2.0
./pr pkg info math
./pr pkg info math 0.2.0
./pr pkg search math
PUREPKG_TOKEN=... ./pr pkg publish --git=https://github.com/you/your-pkg.git
```

Registry deps look like this in `pure.toml`:

```toml
[dependencies]
math = { pkg = "math", version = "0.2.0" }
```

The registry is a thin layer on top of Git: every published version maps to
a public Git URL plus a tag/commit. You can override the registry endpoint
with `PUREPKG_URL`.

## CLI summary

```text
pr version
pr new <name>
pr run [file_or_dir]
pr run --vm <file>            # bytecode VM
pr check [file_or_dir]
pr build                      # writes build/app.txt
pr build --native <file> [-o] # compile to a native binary via Go
pr fmt [file]                 # format in-place
pr test                       # run *_test.pure files
pr lsp                        # LSP server over stdio
pr deps
pr deps add <name> <github-url>
pr deps add --pkg <name>[@version]
pr deps update
pr deps clean
pr pkg info <name> [version]
pr pkg search <query>
pr pkg publish --git=<url>
pr selfhost info
pr selfhost lex <file>        # run the PureLang-implemented lexer
```

## MVP syntax

PureLang allows top-level code with no required `main` function and no
semicolons.

```pure
use std.io

print "Hello, PureLang"
```

Variables are immutable by default. `var` introduces a mutable variable.

```pure
name = "Alex"
age = 21

var count = 0
count = count + 1
```

Functions are concise. Use `=>` for expression-body functions or `{ ... }` for
block-body functions.

```pure
greet(name: String) => "Hello, $name"

add(a: Int, b: Int) {
    a + b
}
```

Data types are declared with parameter-style syntax.

```pure
User(name: String, age: Int) {
    adult = age >= 18
    greeting => "Hello, $name"
}

user = User("Alex", 21)
print user.name
print user.adult
print user.greeting
```

`if` is an expression:

```pure
status = if age >= 18 {
    "adult"
} else {
    "minor"
}
```

Pattern matching with `when`:

```pure
result = when x {
    1 => "one"
    2, 3 => "small"
    n if n > 10 => "big"
    else => "other"
}

label = when {
    y < 10 => "small"
    y < 100 => "medium"
    else => "large"
}
```

Null safety with `T?`, `?.`, and `?:`:

```pure
User(name: String?, age: Int)

a = User("Alex", 21)
b = User(null, 30)

print a.name ?: "anonymous"
print b.name ?: "anonymous"

x = null
print x?.foo ?: "fallback"
```

Loops, lists, and command-style calls:

```pure
numbers = [1, 2, 3]

for n in numbers {
    print n
}

print "Hello"
print("Hello")
```

String interpolation supports both `$name` and `${expr}`:

```pure
name = "Alex"
print "Hello, $name"
print "Hello, ${user.name}"
```

## Module resolution

When the compiler encounters a `use` statement, it searches in this order:

1. The current project's `src/`
2. Downloaded dependencies in `.pure/deps/`
3. The built-in `std` namespace

Examples:

```pure
use std.io                  # built-in standard library module
use math                    # dependency at .pure/deps/math
use math.geometry           # nested file inside the dependency
use app.models.user         # local project file
```

Every PureLang source file becomes its own module. Top-level declarations
are visible only inside the file unless explicitly imported via `use`. The
project name in `pure.toml` is the dotted root for project-local files
(`use <project>.<dotted.path>`).

## Standard library

| Module       | Functions                                                                |
| ------------ | ------------------------------------------------------------------------ |
| `std.io`     | `print`, `println`, `input`                                              |
| `std.list`   | `len`, `first`, `last`, `push`, `range`, `reverse`, `sort_ints`          |
| `std.string` | `upper`, `lower`, `trim`, `split`, `join`, `contains`, `to_int`, `to_string` |
| `std.math`   | `abs`, `min`, `max`, `sqrt`, `pow`, `floor`, `ceil`, `pi`, `e`           |
| `std.os`     | `args`, `env`, `exit`                                                    |
| `std.fs`     | `read_file`, `write_file`, `exists`                                      |

## Editor support

A Visual Studio Code extension lives under `editors/vscode/`. It contributes:

- A TextMate grammar with comment, string (incl. `$ident`/`${expr}`),
  number, keyword, operator, type, data declaration, and function
  declaration scopes.
- `language-configuration.json` for brackets, comments, and indent rules.
- A small `extension.js` that launches `pr lsp` via
  `vscode-languageclient` so PureLang files get diagnostics, formatting,
  and hover documentation in any LSP-aware editor.

To install locally, see `editors/vscode/README.md`.

## Self-hosted compiler bootstrap

The `compiler/` directory hosts the seed of the future self-hosted
PureLang compiler, written in PureLang itself. Run the bootstrap lexer
scaffolding with:

```sh
./pr selfhost lex examples/hello.pure
./pr selfhost info
./pr run compiler
```

Today the bootstrap demonstrates `Token` data types, character
classification, and keyword classification using `when`. As the
language grows (mutable accumulators inside functions, string slicing,
etc.) the bootstrap will grow into a real lexer, parser, type checker,
and code generator that targets the bytecode VM. See
`compiler/README.md` for the staged roadmap.

## Testing

```sh
go test ./...
```

The tests cover:

- Lexer, parser, formatter
- Project loader and `pure.toml` parser
- Dependency manager (Git-backed and registry-backed paths via a fake
  resolver)
- purepkg HTTP client (against `httptest`)
- Bytecode VM (arithmetic, functions, if expressions, elvis, lists)
- Native compilation (writes a Go program, builds it, runs the binary)
- LSP server (initialize handshake and diagnostic publishing)
- Module isolation (unimported declarations are not visible)
- CLI integration (hello/user/functions examples, project mode, project
  with a local Git dependency)
