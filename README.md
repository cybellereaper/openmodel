# PureLang

PureLang is a clean, strongly typed programming language inspired by Swift,
Kotlin, and Ruby. It removes boilerplate while keeping modern safety features
like immutable-by-default variables, type inference, and concise data types.

This repository contains the **MVP implementation** of the PureLang
compiler/interpreter, written in Go. It can run single PureLang files, manage
PureLang projects via `pure.toml`, and download GitHub dependencies into
`.pure/deps/`.

| Property                    | Value          |
| --------------------------- | -------------- |
| Source extension            | `.pure`        |
| Compiler / interpreter      | `pr`           |
| Project file                | `pure.toml`    |
| Lock file                   | `pure.lock`    |
| Standard library namespace  | `std`          |
| Dependency cache            | `.pure/deps/`  |
| Implementation language     | Go (stdlib only) |

## Building

PureLang's reference implementation requires Go 1.22+ and a local `git`
executable for downloading GitHub dependencies.

```bash
go build -o pr ./cmd/pr
./pr version
```

## Running single files

```bash
./pr run examples/hello.pure
./pr run examples/user.pure
./pr run examples/functions.pure
```

## Creating a project

```bash
./pr new my_app
cd my_app
../pr run
```

`pr new` generates:

```text
my_app/
  pure.toml
  src/
    main.pure
```

## Running projects

When you run `pr run` inside a project directory (or pass a project directory
as an argument), the compiler:

1. Walks upward to find `pure.toml`.
2. Loads project metadata.
3. Installs missing dependencies (or prints a helpful error).
4. Parses every `.pure` file under `src/` and inside `.pure/deps/<dep>/src/`.
5. Registers all top-level declarations.
6. Executes the configured entry file.

```bash
./pr run examples_project
./pr check examples_project
```

## Adding GitHub dependencies

```bash
./pr deps                                            # install / refresh deps
./pr deps add math https://github.com/example/pure-math.git
./pr deps update                                     # update branch deps
./pr deps clean                                      # delete .pure/deps
```

A dependency declaration in `pure.toml` looks like this:

```toml
[dependencies]
math = { git = "https://github.com/example/pure-math.git", version = "v0.1.0" }
json = { git = "https://github.com/example/pure-json.git", branch = "main" }
core = { git = "https://github.com/example/pure-core.git", commit = "abc123" }
```

Exactly one of `version`, `branch`, or `commit` should be supplied. If none is
given, `branch = "main"` is used by default.

Dependencies are downloaded to `.pure/deps/<name>/` using the local `git`
binary, and resolved commit hashes are recorded in `pure.lock`.

## CLI summary

```text
pr version
pr new <name>
pr run [file_or_dir]
pr check [file_or_dir]
pr build
pr test
pr fmt [file]
pr deps
pr deps add <name> <github-url>
pr deps update
pr deps clean
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

Data types are declared with parameter-style syntax. The compiler distinguishes
data declarations from functions by the leading capital letter.

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

Loops, lists, and command-style calls are also supported:

```pure
numbers = [1, 2, 3]

for n in numbers {
    print n
}

print "Hello"
print("Hello")
```

String interpolation supports both `$name` and `${expr}` forms:

```pure
name = "Alex"
print "Hello, $name"
print "Length is ${name.length}"
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
use app.models.user         # local project file (project name is from pure.toml)
```

For MVP, top-level declarations from all parsed files are visible globally.
Module isolation will be tightened in a future release.

## Testing

Run the Go test suite from the repository root:

```bash
go test ./...
```

The tests cover the lexer, parser, project loader, dependency manager, and a
set of CLI integration tests including a project that consumes a local Git
repository as a dependency.

## Roadmap

This MVP is intentionally small and extensible. Planned future features:

- Pattern matching with `when`
- Null safety with `?`
- Better module isolation
- Package registry integration with `purepkg`
- Bytecode VM
- Native compilation
- Self-hosted compiler
- Formatter implementation (`pr fmt`)
- Language server protocol support
- Editor syntax highlighting
- Standard library expansion
- Publishing packages to `purepkg`
