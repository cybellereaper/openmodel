// Package modules implements per-file module scopes and `use` resolution.
//
// Each .pure source file becomes a Module. A module has:
//   - a dotted name (e.g. "app.models.user")
//   - a set of exported top-level declarations
//   - a list of imports it requested via `use`
//
// Module isolation: a module only sees identifiers that are either
// declared in its own file, imported via `use`, or part of the global
// built-in scope (print, println, etc.).
package modules

import (
	"fmt"
	"path/filepath"
	"strings"

	"purelang/internal/ast"
	"purelang/internal/project"
)

// Module represents a single .pure file.
type Module struct {
	Name    string // dotted name, e.g. "app.models.user" or "math" or "std.io"
	Path    string // absolute file path; "" for built-in modules
	Program *ast.Program
	Imports []string // dotted names the module imported via `use`
}

// Graph is a collection of modules indexed by name.
type Graph struct {
	Modules map[string]*Module // name -> module
}

func NewGraph() *Graph {
	return &Graph{Modules: map[string]*Module{}}
}

// Add registers a module.
func (g *Graph) Add(m *Module) {
	g.Modules[m.Name] = m
}

// FileToModuleName turns an absolute source path under a project's src/
// (or under a dependency's src/) into a dotted module name rooted at root.
//
// e.g. project=app, file=src/models/user.pure  -> "app.models.user"
//      project=math, file=src/math.pure         -> "math"
//      project=math, file=src/geometry/area.pure -> "math.geometry.area"
func FileToModuleName(rootName, srcDir, file string) (string, error) {
	rel, err := filepath.Rel(srcDir, file)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(rel, ".pure") {
		return "", fmt.Errorf("not a .pure file: %s", file)
	}
	rel = strings.TrimSuffix(rel, ".pure")
	parts := strings.Split(filepath.ToSlash(rel), "/")
	// If the file is named the same as the root (e.g. math/src/math.pure),
	// collapse to just the root name. Otherwise prepend the root name.
	if len(parts) == 1 && parts[0] == rootName {
		return rootName, nil
	}
	out := []string{rootName}
	out = append(out, parts...)
	// Drop a trailing component that duplicates the root (rare but allowed).
	return strings.Join(out, "."), nil
}

// CollectImports walks a program for `use` declarations.
func CollectImports(prog *ast.Program) []string {
	var imports []string
	for _, stmt := range prog.Stmts {
		if u, ok := stmt.(*ast.UseDecl); ok {
			imports = append(imports, strings.Join(u.Path, "."))
		}
	}
	return imports
}

// ResolveModuleName tries to locate a module name across the project and its
// dependencies and returns the file path if found.
//
// Search order: current project src/, dependencies in .pure/deps/.
func ResolveModuleName(p *project.Project, name string) (string, bool) {
	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		return "", false
	}
	// Try current project
	if parts[0] == p.Name {
		if path, ok := tryModule(p.SourceDir, parts[1:]); ok {
			return path, true
		}
		// Project-name only -> entry file
		if len(parts) == 1 {
			return p.EntryPath(), true
		}
	}
	// Try dependency
	depRoot := filepath.Join(p.DepsDir(), parts[0])
	if depToml := filepath.Join(depRoot, "pure.toml"); fileExists(depToml) {
		depProj, err := project.LoadProject(depRoot)
		if err == nil {
			if len(parts) == 1 {
				if entry := depProj.EntryPath(); fileExists(entry) {
					return entry, true
				}
			}
			if path, ok := tryModule(depProj.SourceDir, parts[1:]); ok {
				return path, true
			}
		}
	}
	return "", false
}

func tryModule(srcDir string, parts []string) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}
	// Try as file: parts joined with /, suffixed with .pure
	candidate := filepath.Join(append([]string{srcDir}, parts...)...) + ".pure"
	if fileExists(candidate) {
		return candidate, true
	}
	// Try as folder with main.pure
	folder := filepath.Join(append([]string{srcDir}, parts...)...)
	main := filepath.Join(folder, "main.pure")
	if fileExists(main) {
		return main, true
	}
	return "", false
}

func fileExists(path string) bool {
	_, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return existsOnDisk(path)
}
