package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"purelang/internal/project"
)

// Install reads project dependencies and downloads any missing ones,
// then writes pure.lock with resolved commit info.
func Install(p *project.Project) error {
	if len(p.Dependencies) == 0 {
		// Still write an empty lock file for determinism if dir exists, otherwise no-op.
		_ = WriteLock(p.LockPath(), nil)
		return nil
	}
	if !GitInstalled() {
		return ErrGitMissing
	}
	if err := os.MkdirAll(p.DepsDir(), 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(p.Dependencies))
	for n := range p.Dependencies {
		names = append(names, n)
	}
	sort.Strings(names)
	var locks []LockedDependency
	for _, name := range names {
		dep := p.Dependencies[name]
		if err := validateDep(dep); err != nil {
			return err
		}
		dep = applyDefaults(dep)
		dest := filepath.Join(p.DepsDir(), name)
		if _, err := os.Stat(dest); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := Clone(dep.Git, dest); err != nil {
				return err
			}
		}
		ref := refOf(dep)
		if ref != "" {
			if err := Checkout(dest, ref); err != nil {
				return err
			}
		}
		commit, err := CurrentCommit(dest)
		if err != nil {
			return err
		}
		locks = append(locks, LockedDependency{
			Name:      name,
			Git:       dep.Git,
			Source:    dep.Source(),
			Requested: dep.Requested(),
			Resolved:  commit,
		})
	}
	return WriteLock(p.LockPath(), locks)
}

// Add adds a dependency to pure.toml and runs install.
func Add(projectRoot string, name string, gitURL string) error {
	tomlPath := filepath.Join(projectRoot, "pure.toml")
	raw, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", tomlPath, err)
	}
	data, err := project.ParseTOML(string(raw))
	if err != nil {
		return fmt.Errorf("parse %s: %w", tomlPath, err)
	}
	if data.Dependencies == nil {
		data.Dependencies = map[string]project.Dependency{}
	}
	data.Dependencies[name] = project.Dependency{
		Name:   name,
		Git:    gitURL,
		Branch: "main",
	}
	if err := os.WriteFile(tomlPath, []byte(project.EncodeTOML(data)), 0o644); err != nil {
		return err
	}
	p, err := project.LoadProject(projectRoot)
	if err != nil {
		return err
	}
	return Install(p)
}

// Update refreshes branch-tracking dependencies; pinned ones (version/commit) stay.
func Update(p *project.Project) error {
	if len(p.Dependencies) == 0 {
		return nil
	}
	if !GitInstalled() {
		return ErrGitMissing
	}
	names := make([]string, 0, len(p.Dependencies))
	for n := range p.Dependencies {
		names = append(names, n)
	}
	sort.Strings(names)
	var locks []LockedDependency
	for _, name := range names {
		dep := applyDefaults(p.Dependencies[name])
		dest := filepath.Join(p.DepsDir(), name)
		if _, err := os.Stat(dest); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := Clone(dep.Git, dest); err != nil {
				return err
			}
		}
		switch dep.Source() {
		case "branch":
			if err := Checkout(dest, dep.Branch); err == nil {
				_ = Pull(dest)
			}
		case "version":
			_ = Checkout(dest, dep.Version)
		case "commit":
			_ = Checkout(dest, dep.Commit)
		}
		commit, err := CurrentCommit(dest)
		if err != nil {
			return err
		}
		locks = append(locks, LockedDependency{
			Name:      name,
			Git:       dep.Git,
			Source:    dep.Source(),
			Requested: dep.Requested(),
			Resolved:  commit,
		})
	}
	return WriteLock(p.LockPath(), locks)
}

// Clean removes the .pure/deps directory.
func Clean(p *project.Project) error {
	depsDir := p.DepsDir()
	if _, err := os.Stat(depsDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(depsDir)
}

func applyDefaults(d project.Dependency) project.Dependency {
	if d.Source() == "" {
		d.Branch = "main"
	}
	return d
}

func validateDep(d project.Dependency) error {
	if d.Git == "" {
		return fmt.Errorf("dependency %q is missing required 'git' field", d.Name)
	}
	count := 0
	if d.Version != "" {
		count++
	}
	if d.Branch != "" {
		count++
	}
	if d.Commit != "" {
		count++
	}
	if count > 1 {
		return fmt.Errorf("dependency %q must specify only one of version, branch, or commit", d.Name)
	}
	if !isValidName(d.Name) {
		return fmt.Errorf("dependency name %q is not a valid PureLang module name", d.Name)
	}
	return nil
}

func isValidName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
			continue
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func refOf(d project.Dependency) string {
	switch d.Source() {
	case "version":
		return d.Version
	case "branch":
		return d.Branch
	case "commit":
		return d.Commit
	}
	return ""
}
