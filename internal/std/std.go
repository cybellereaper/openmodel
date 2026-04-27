// Package std hosts metadata about the PureLang standard library.
//
// The standard library is implemented as built-in functions registered on
// demand by the interpreter when a module imports the corresponding
// `std.<module>` namespace. This file lists which std modules exist and
// provides query helpers for the import resolver.
package std

// Modules is the list of supported `use std.<module>` namespaces.
var Modules = map[string]bool{
	"io":     true,
	"list":   true,
	"string": true,
	"math":   true,
	"os":     true,
	"fs":     true,
}

// IsStdModule reports whether the given dotted path is a known std module.
func IsStdModule(path []string) bool {
	if len(path) < 2 || path[0] != "std" {
		return false
	}
	return Modules[path[1]]
}

// IsStdPath reports whether the given dotted name is a std.* path.
// Used to decide whether a module name should be looked up in std.
func IsStdPath(parts []string) bool {
	return len(parts) >= 1 && parts[0] == "std"
}
