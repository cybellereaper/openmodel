package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project describes a loaded PureLang project.
type Project struct {
	Name         string
	Version      string
	Entry        string
	RootDir      string
	SourceDir    string
	Dependencies map[string]Dependency
}

// FindProjectRoot walks upward from start until it finds a pure.toml.
func FindProjectRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		candidate := filepath.Join(dir, "pure.toml")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no pure.toml found from %q", start)
		}
		dir = parent
	}
}

// LoadProject reads a project's pure.toml.
func LoadProject(root string) (*Project, error) {
	tomlPath := filepath.Join(root, "pure.toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", tomlPath, err)
	}
	parsed, err := ParseTOML(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", tomlPath, err)
	}
	if parsed.Name == "" {
		return nil, fmt.Errorf("%s: missing 'name'", tomlPath)
	}
	entry := parsed.Entry
	if entry == "" {
		entry = "src/main.pure"
	}
	return &Project{
		Name:         parsed.Name,
		Version:      parsed.Version,
		Entry:        entry,
		RootDir:      root,
		SourceDir:    filepath.Join(root, "src"),
		Dependencies: parsed.Dependencies,
	}, nil
}

// CreateProject scaffolds a new project at <path>/<name>.
func CreateProject(path string, name string) error {
	root := filepath.Join(path, name)
	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("path %q already exists", root)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		return err
	}
	toml := fmt.Sprintf(`name = %q
version = "0.1.0"
entry = "src/main.pure"

[dependencies]
`, name)
	if err := os.WriteFile(filepath.Join(root, "pure.toml"), []byte(toml), 0o644); err != nil {
		return err
	}
	main := `use std.io

print "Hello, PureLang"
`
	if err := os.WriteFile(filepath.Join(root, "src", "main.pure"), []byte(main), 0o644); err != nil {
		return err
	}
	return nil
}

// ListSourceFiles returns sorted list of .pure files under src/.
func ListSourceFiles(p *Project) ([]string, error) {
	var files []string
	err := filepath.Walk(p.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".pure") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// EntryPath returns the absolute path of the entry file.
func (p *Project) EntryPath() string {
	return filepath.Join(p.RootDir, p.Entry)
}

// DepsDir returns the path to .pure/deps in this project.
func (p *Project) DepsDir() string {
	return filepath.Join(p.RootDir, ".pure", "deps")
}

// LockPath returns the absolute path of pure.lock for this project.
func (p *Project) LockPath() string {
	return filepath.Join(p.RootDir, "pure.lock")
}
