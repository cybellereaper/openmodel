// Package std hosts metadata about the PureLang standard library.
//
// For MVP, the standard library is implemented as built-in functions registered
// directly in the interpreter (see internal/runtime). This package exists so
// the standard-library namespace has a clear home, and is used by the compiler
// to validate `use std.<module>` imports.
package std

// Modules is the list of supported `use std.<module>` namespaces.
var Modules = map[string]bool{
	"io": true,
}

// IsStdModule reports whether the given dotted path is a known std module.
func IsStdModule(path []string) bool {
	if len(path) < 2 || path[0] != "std" {
		return false
	}
	return Modules[path[1]]
}
